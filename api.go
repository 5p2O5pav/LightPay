package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
    "strconv"
    "time"

    "github.com/gin-gonic/gin"
)

func CreateTransaction(c *gin.Context) {
    var req map[string]interface{}
    if err := c.BindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"status_code": 400, "message": "请求参数错误"})
        return
    }

    pid := fmt.Sprint(req["pid"])
    orderID := fmt.Sprint(req["order_id"])
    currency := fmt.Sprint(req["currency"])
    token := fmt.Sprint(req["token"])
    network := fmt.Sprint(req["network"])
    amountFloat, _ := req["amount"].(float64)
    notifyURL := fmt.Sprint(req["notify_url"])
    redirectURL := fmt.Sprint(req["redirect_url"])

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

    // 金额转为与 PHP 一致的字符串（去除末尾零和小数点）
    amountStr := formatAmount(amountFloat)

    // 签名验证
    params := map[string]string{
        "pid":          pid,
        "order_id":     orderID,
        "currency":     currency,
        "token":        token,
        "network":      network,
        "amount":       amountStr,
        "notify_url":   notifyURL,
        "redirect_url": redirectURL,
    }
    signature := fmt.Sprint(req["signature"])
    expectedSign := MakeSignature(params, config.ApiToken)

// ========== 添加这三行日志 ==========
log.Printf("请求中的 signature: %s", signature)
log.Printf("服务端计算的签名: %s", expectedSign)
log.Printf("用于签名的参数: %+v", params)
// =================================
    
    if signature != expectedSign {
        c.JSON(http.StatusUnauthorized, gin.H{"status_code": 401, "message": "签名验证失败"})
        return
    }

    handler, ok := chainRegistry[network]
    if !ok {
        c.JSON(http.StatusBadRequest, gin.H{"status_code": 400, "message": fmt.Sprintf("不支持的网络: %s", network)})
        return
    }

    address, err := handler.SelectWallet(orderID)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"status_code": 500, "message": "无可用钱包"})
        return
    }

    // 加密货币金额（最小单位）
    finalAmount, err := CalculateCryptoAmount(currency, token, amountFloat, config.Pricing)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"status_code": 500, "message": "汇率计算失败"})
        return
    }

    lockedAmount, err := LockAmountWithIncrement(address, finalAmount)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"status_code": 500, "message": "无法分配支付金额"})
        return
    }

    tradeID := generateTradeID()
    order := &Order{
        TradeID:     tradeID,
        OrderID:     orderID,
        Pid:         pid,
        Chain:       network,
        Address:     address,
        Amount:      lockedAmount,
        FiatAmount:  amountFloat,
        Currency:    currency,
        Token:       token,
        NotifyURL:   notifyURL,
        RedirectURL: redirectURL,
        Status:      "pending",
        ExpiredAt:   time.Now().Add(10 * time.Minute),
        CreatedAt:   time.Now(),
    }
    if err := SaveOrder(order); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"status_code": 500, "message": "订单保存失败"})
        return
    }

    handler.EnsureAddressListener(address)

    paymentURL := fmt.Sprintf("https://%s/pay/%s", config.Server.Domain, tradeID)
    c.JSON(http.StatusOK, gin.H{
        "status_code": 200,
        "message":     "success",
        "data": gin.H{
            "trade_id":    tradeID,
            "order_id":    orderID,
            "amount":      float64(lockedAmount) / 1e6,
            "address":     address,
            "payment_url": paymentURL,
            "expired_at":  order.ExpiredAt.Unix(),
        },
    })
}

// formatAmount 与 PHP 的 rtrim(rtrim(sprintf('%.12F', $value), '0'), '.') 行为一致
func formatAmount(v float64) string {
    // 使用 'f' 格式，-1 表示自动去除末尾零
    s := strconv.FormatFloat(v, 'f', -1, 64)
    // 防止空字符串（极小概率）
    if s == "" {
        return "0"
    }
    return s
}

// 发送异步通知给商户
func sendOrderNotify(order *Order) {
    if order.NotifyURL == "" {
        return
    }
    // 构造回调参数（与 Epusdt 规范一致）
    params := map[string]string{
        "trade_id":   order.TradeID,
        "order_id":   order.OrderID,
        "status":     "2", // 2 表示支付成功
        "amount":     formatAmount(order.FiatAmount),
        "currency":   order.Currency,
        "token":      order.Token,
        "network":    order.Chain,
        "pid":        order.Pid,
    }
    params["signature"] = MakeSignature(params, config.ApiToken)

    jsonData, _ := json.Marshal(params)
    client := &http.Client{Timeout: 15 * time.Second}

    // 重试 3 次
    for i := 0; i < 3; i++ {
        resp, err := client.Post(order.NotifyURL, "application/json", bytes.NewBuffer(jsonData))
        if err == nil {
            resp.Body.Close()
            if resp.StatusCode >= 200 && resp.StatusCode < 300 {
                break
            }
        }
        time.Sleep(time.Duration(i+1) * 2 * time.Second)
    }
}

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
            "amount":   float64(order.Amount) / 1e6,
        },
    })
}

func PayPage(c *gin.Context) {
    c.FileFromFS("web/index.html", http.FS(webFiles))
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
        "amount":     float64(order.Amount) / 1e6,
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
