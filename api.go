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

    // 转为小写（用于内部处理）
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

    // 签名验证（使用原始传入的值，保持与 PHP 一致）
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
    log.Printf("[DEBUG] 签名对比: 收到=%s, 计算=%s", signature, expectedSign)
    if signature != expectedSign {
        log.Printf("[ERROR] 签名验证失败")
        c.JSON(http.StatusUnauthorized, gin.H{"status_code": 401, "message": "签名验证失败"})
        return
    }
    log.Printf("[INFO] 签名验证通过")

    handler, ok := chainRegistry[network]
    if !ok {
        log.Printf("[ERROR] 不支持的网络: %s", network)
        c.JSON(http.StatusBadRequest, gin.H{"status_code": 400, "message": fmt.Sprintf("不支持的网络: %s", network)})
        return
    }

    address, err := handler.SelectWallet(orderID)
    if err != nil {
        log.Printf("[ERROR] 无可用钱包: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"status_code": 500, "message": "无可用钱包"})
        return
    }
    log.Printf("[INFO] 分配地址: %s", address)

    finalAmount, err := CalculateCryptoAmount(currency, token, amountFloat, config.Pricing)
    if err != nil {
        log.Printf("[ERROR] 汇率计算失败: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"status_code": 500, "message": "汇率计算失败"})
        return
    }
    log.Printf("[INFO] 加密货币金额（最小单位）: %d", finalAmount)

    lockedAmount, err := LockAmountWithIncrement(address, finalAmount)
    if err != nil {
        log.Printf("[ERROR] 无法分配支付金额: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"status_code": 500, "message": "无法分配支付金额"})
        return
    }
    log.Printf("[INFO] 锁定金额（最小单位）: %d", lockedAmount)

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
        log.Printf("[ERROR] 订单保存失败: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"status_code": 500, "message": "订单保存失败"})
        return
    }
    log.Printf("[INFO] 订单已保存: trade_id=%s", tradeID)

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

// formatAmount 用于回调通知中的金额格式化（与 Epusdt 插件一致）
func formatAmount(v float64) string {
    s := strconv.FormatFloat(v, 'f', -1, 64)
    if s == "" {
        return "0"
    }
    return s
}

// sendOrderNotify 发送异步通知给商户
func sendOrderNotify(order *Order) {
    if order.NotifyURL == "" {
        log.Printf("[WARN] 订单 %s 没有通知地址，跳过回调", order.TradeID)
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
    log.Printf("[INFO] 发送回调到 %s, 参数: %s", order.NotifyURL, string(jsonData))

    for i := 0; i < 3; i++ {
        resp, err := client.Post(order.NotifyURL, "application/json", bytes.NewBuffer(jsonData))
        if err == nil {
            resp.Body.Close()
            if resp.StatusCode >= 200 && resp.StatusCode < 300 {
                log.Printf("[INFO] 回调成功: HTTP %d", resp.StatusCode)
                return
            }
            log.Printf("[WARN] 回调返回非2xx状态码: %d, 重试 %d/3", resp.StatusCode, i+1)
        } else {
            log.Printf("[ERROR] 回调请求失败: %v, 重试 %d/3", err, i+1)
        }
        time.Sleep(time.Duration(i+1) * 2 * time.Second)
    }
    log.Printf("[ERROR] 订单 %s 回调最终失败", order.TradeID)
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
