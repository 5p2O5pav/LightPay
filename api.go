package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "strconv"
    "strings"
    "time"

    "github.com/gin-gonic/gin"
)

// 注意：webFiles 已在 main.go 中定义，这里需要引用外部变量，或者将 webFiles 移到全局
// 为清晰，假设 webFiles 是全局可访问的（通过包变量）

func CreateTransaction(c *gin.Context) {
    var req map[string]interface{}
    if err := c.BindJSON(&req); err != nil {
        log.Printf("[ERROR] 绑定请求参数失败: %v", err)
        c.JSON(http.StatusBadRequest, gin.H{"status_code": 400, "message": "请求参数错误"})
        return
    }

    pid := fmt.Sprint(req["pid"])
    orderID := fmt.Sprint(req["order_id"])
    rawCurrency := fmt.Sprint(req["currency"])
    rawToken := fmt.Sprint(req["token"])
    rawNetwork := fmt.Sprint(req["network"])
    amountFloat, _ := req["amount"].(float64)
    notifyURL := fmt.Sprint(req["notify_url"])
    redirectURL := fmt.Sprint(req["redirect_url"])

    log.Printf("[INFO] 收到订单创建请求: order_id=%s, amount=%.2f, raw_currency=%s, raw_token=%s, raw_network=%s",
        orderID, amountFloat, rawCurrency, rawToken, rawNetwork)

    currency := strings.ToLower(rawCurrency)
    token := strings.ToLower(rawToken)
    network := strings.ToLower(rawNetwork)

    if orderID == "" || amountFloat <= 0 {
        c.JSON(http.StatusBadRequest, gin.H{"status_code": 400, "message": "缺少必要参数"})
        return
    }
    if currency == "" {
        currency = "cny"
    }
    if token == "" {
        token = "usdt"
    }
    if network == "" {
        network = "tron"
    }

    // 签名验证
    signParams := map[string]interface{}{
        "pid":          pid,
        "order_id":     orderID,
        "currency":     rawCurrency,
        "token":        rawToken,
        "network":      rawNetwork,
        "amount":       amountFloat,
        "notify_url":   notifyURL,
        "redirect_url": redirectURL,
    }
    signature := fmt.Sprint(req["signature"])
    expectedSign := MakeSignatureFromMap(signParams, config.ApiToken)
    if signature != expectedSign {
        log.Printf("[ERROR] 签名验证失败")
        c.JSON(http.StatusUnauthorized, gin.H{"status_code": 401, "message": "签名验证失败"})
        return
    }

    // 获取链处理器（注意：现在每个链对象需要支持 token，但注册时是以链名为 key，一个链名对应一个处理器）
    // 但我们的拆分方案中，tron 同时有 USDT 和 TRX 两个处理器，它们 Name() 都返回 "tron"，会冲突！
    // 修正：链名应包含 token 区分，例如 "tron_usdt"、"tron_trx"、"polygon_usdt"、"bsc_usdt"
    // 为保持向后兼容，建议在 registry 中使用 "chain:token" 作为 key。
    // 这里简单起见，我们创建一个新的 map：chainTokenRegistry
    handlerKey := network + ":" + token
    handler, ok := chainTokenRegistry[handlerKey]
    if !ok {
        log.Printf("[ERROR] 不支持的链/币种组合: %s/%s", network, token)
        c.JSON(http.StatusBadRequest, gin.H{"status_code": 400, "message": fmt.Sprintf("不支持的链/币种: %s/%s", network, token)})
        return
    }

    address, err := handler.SelectWallet(orderID)
    if err != nil {
        log.Printf("[ERROR] 无可用钱包: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"status_code": 500, "message": "无可用钱包"})
        return
    }

    // 计算基础金额（内部精度3位）
    baseAmount, err := CalculateCryptoAmount(currency, token, amountFloat)
    if err != nil {
        log.Printf("[ERROR] 汇率计算失败: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"status_code": 500, "message": "汇率计算失败"})
        return
    }

    // 加价分配唯一金额
    finalAmount, err := LockAmountWithIncrement(address, baseAmount)
    if err != nil {
        if err.Error() == "busy: too many pending orders with similar amount" {
            // 返回交易火爆页面
            c.Header("Content-Type", "text/html; charset=utf-8")
            errorPage, _ := webFiles.ReadFile("web/error_busy.html")
            if errorPage == nil {
                c.String(http.StatusServiceUnavailable, "当前交易繁忙，请稍后再试")
            } else {
                c.Data(http.StatusServiceUnavailable, "text/html; charset=utf-8", errorPage)
            }
            return
        }
        log.Printf("[ERROR] 无法分配支付金额: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"status_code": 500, "message": "无法分配支付金额"})
        return
    }

    tradeID := generateTradeID()
    order := &Order{
        TradeID:     tradeID,
        OrderID:     orderID,
        Pid:         pid,
        Chain:       network,
        Token:       token,
        Address:     address,
        Amount:      finalAmount,
        FiatAmount:  amountFloat,
        Currency:    currency,
        NotifyURL:   notifyURL,
        RedirectURL: redirectURL,
        Status:      "pending",
        ExpiredAt:   time.Now().Add(10 * time.Minute),
        CreatedAt:   time.Now(),
    }
    if err := SaveOrder(order); err != nil {
        log.Printf("[ERROR] 订单保存失败: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"status_code": 500, "message": "订单保存失败"})
        return
    }

    handler.EnsureAddressListener(address, token)

    // 显示金额：内部精度转回显示（3位小数）
    displayAmount := float64(finalAmount) / InternalPrecision
    paymentURL := fmt.Sprintf("https://%s/pay/%s", config.Server.Domain, tradeID)
    c.JSON(http.StatusOK, gin.H{
        "status_code": 200,
        "message":     "success",
        "data": gin.H{
            "trade_id":    tradeID,
            "order_id":    orderID,
            "amount":      displayAmount,
            "address":     address,
            "payment_url": paymentURL,
            "expired_at":  order.ExpiredAt.Unix(),
        },
    })
}

// formatAmount 用于回调（保持3位小数）
func formatAmount(v float64) string {
    return strconv.FormatFloat(v, 'f', 3, 64)
}

// sendOrderNotify 回调商户（金额格式3位小数）
func sendOrderNotify(order *Order) {
    if order.NotifyURL == "" {
        return
    }
    params := map[string]string{
        "trade_id":   order.TradeID,
        "order_id":   order.OrderID,
        "status":     "2",
        "amount":     formatAmount(order.FiatAmount),
        "currency":   order.Currency,
        "token":      order.Token,
        "network":    order.Chain,
        "pid":        order.Pid,
    }
    params["signature"] = MakeSignature(params, config.ApiToken)

    jsonData, _ := json.Marshal(params)
    client := &http.Client{Timeout: 15 * time.Second}
    for i := 0; i < 3; i++ {
        resp, err := client.Post(order.NotifyURL, "application/json", bytes.NewBuffer(jsonData))
        if err == nil {
            resp.Body.Close()
            if resp.StatusCode >= 200 && resp.StatusCode < 300 {
                log.Printf("[INFO] 回调成功: %s", order.TradeID)
                return
            }
        }
        time.Sleep(time.Duration(i+1) * 2 * time.Second)
    }
    log.Printf("[ERROR] 回调最终失败: %s", order.TradeID)
}

// QueryOrder, PayPage, GetOrderInfo, GetOrderStatus 等函数保持不变，但注意金额显示用3位
func QueryOrder(c *gin.Context) {
    tradeID := c.Query("trade_id")
    order, err := GetOrderByTradeID(tradeID)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"status_code": 404, "message": "订单不存在"})
        return
    }
    c.JSON(http.StatusOK, gin.H{
        "status_code": 200,
        "data": gin.H{
            "trade_id": order.TradeID,
            "order_id": order.OrderID,
            "status":   order.Status,
            "amount":   float64(order.Amount) / InternalPrecision,
        },
    })
}

func PayPage(c *gin.Context) {
    data, err := webFiles.ReadFile("web/index.html")
    if err != nil {
        c.String(http.StatusInternalServerError, "支付页面加载失败")
        return
    }
    c.Data(http.StatusOK, "text/html; charset=utf-8", data)
}

func GetOrderInfo(c *gin.Context) {
    tradeID := c.Param("order_id")
    order, err := GetOrderByTradeID(tradeID)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "订单不存在"})
        return
    }
    c.JSON(http.StatusOK, gin.H{
        "trade_id":   order.TradeID,
        "address":    order.Address,
        "amount":     float64(order.Amount) / InternalPrecision,
        "token":      order.Token,
        "network":    order.Chain,
        "expired_at": order.ExpiredAt.Unix(),
        "status":     order.Status,
    })
}

func GetOrderStatus(c *gin.Context) {
    tradeID := c.Param("order_id")
    order, err := GetOrderByTradeID(tradeID)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"status": "error"})
        return
    }
    c.JSON(http.StatusOK, gin.H{
        "status":       order.Status,
        "redirect_url": order.RedirectURL,
    })
}

func generateTradeID() string {
    return fmt.Sprintf("LP%d", time.Now().UnixNano())
}
