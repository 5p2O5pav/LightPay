package main

import (
    "fmt"
    "net/http"
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

    // 签名验证使用独立的 api_token
    params := map[string]string{
        "pid":          pid,
        "order_id":     orderID,
        "currency":     currency,
        "token":        token,
        "network":      network,
        "amount":       fmt.Sprintf("%.2f", amountFloat),
        "notify_url":   notifyURL,
        "redirect_url": redirectURL,
    }
    signature := fmt.Sprint(req["signature"])
    expectedSign := MakeSignature(params, config.ApiToken) // 独立 token
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

    // 金额转换为最小单位
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
        Chain:       network,
        Address:     address,
        Amount:      lockedAmount,
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
