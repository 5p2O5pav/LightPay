package main

import (
    "fmt"
    "net/http"
    "time"

    "github.com/gin-gonic/gin"
)

// CreateTransaction 处理创建订单请求（完全兼容 Epusdt 接口）
func CreateTransaction(c *gin.Context) {
    var req map[string]interface{}
    if err := c.BindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{
            "status_code": 400,
            "message":     "请求参数错误",
        })
        return
    }

    // 提取参数
    pid, _ := req["pid"].(string)
    orderID, _ := req["order_id"].(string)
    currency, _ := req["currency"].(string)
    token, _ := req["token"].(string)
    network, _ := req["network"].(string)
    amount, _ := req["amount"].(float64)
    notifyURL, _ := req["notify_url"].(string)
    redirectURL, _ := req["redirect_url"].(string)

    if orderID == "" || amount <= 0 {
        c.JSON(http.StatusBadRequest, gin.H{
            "status_code": 400,
            "message":     "缺少必要参数",
        })
        return
    }

    // 默认值
    if currency == "" {
        currency = "cny"
    }
    if token == "" {
        token = "usdt"
    }
    if network == "" {
        network = "tron"
    }

    // 验证签名（使用配置文件中的 api_token，这里假设 config.Tron.ApiKey 实际为签名 token）
    // 注意：实际部署时应将签名 token 与 API Key 分开
    params := map[string]string{
        "pid":          pid,
        "order_id":     orderID,
        "currency":     currency,
        "token":        token,
        "network":      network,
        "amount":       fmt.Sprintf("%.2f", amount),
        "notify_url":   notifyURL,
        "redirect_url": redirectURL,
    }
    signature, _ := req["signature"].(string)
    expectedSign := MakeSignature(params, config.Tron.ApiKey) // 签名 token 应独立存储
    if signature != expectedSign {
        c.JSON(http.StatusUnauthorized, gin.H{
            "status_code": 401,
            "message":     "签名验证失败",
        })
        return
    }

    // 查找对应链的处理器
    handler, ok := chainRegistry[network]
    if !ok {
        c.JSON(http.StatusBadRequest, gin.H{
            "status_code": 400,
            "message":     fmt.Sprintf("不支持的网络: %s", network),
        })
        return
    }

    // 选取钱包地址（由链实现决定，可以是轮选或随机）
    address, err := handler.SelectWallet(orderID)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{
            "status_code": 500,
            "message":     "无可用钱包",
        })
        return
    }

    // 金额可能需要进行法币 → 加密货币的转换，以及加价处理
    finalAmount, err := CalculateCryptoAmount(currency, token, amount, config.Pricing)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{
            "status_code": 500,
            "message":     "汇率计算失败",
        })
        return
    }

    // 锁定金额（模仿 Epusdt 的金额差异匹配）
    lockedAmount, err := LockAmountWithIncrement(address, finalAmount)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{
            "status_code": 500,
            "message":     "无法分配支付金额",
        })
        return
    }

    // 保存订单到数据库
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
        c.JSON(http.StatusInternalServerError, gin.H{
            "status_code": 500,
            "message":     "订单保存失败",
        })
        return
    }

    // **关键步骤：启动该地址的监听（如果尚未启动）**
    handler.EnsureAddressListener(address)

    // 生成支付页面 URL
    paymentURL := fmt.Sprintf("https://%s/pay/%s", config.Server.Domain, tradeID)

    c.JSON(http.StatusOK, gin.H{
        "status_code": 200,
        "message":     "success",
        "data": gin.H{
            "trade_id":    tradeID,
            "order_id":    orderID,
            "amount":      lockedAmount,
            "address":     address,
            "payment_url": paymentURL,
            "expired_at":  order.ExpiredAt.Unix(),
        },
    })
}

// QueryOrder 查询订单状态（Epusdt 兼容接口）
func QueryOrder(c *gin.Context) {
    tradeID := c.Query("trade_id")
    if tradeID == "" {
        c.JSON(http.StatusBadRequest, gin.H{
            "status_code": 400,
            "message":     "缺少 trade_id",
        })
        return
    }

    order, err := GetOrderByTradeID(tradeID)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{
            "status_code": 404,
            "message":     "订单不存在",
        })
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "status_code": 200,
        "data": gin.H{
            "trade_id": order.TradeID,
            "order_id": order.OrderID,
            "status":   order.Status, // pending, paid, expired
            "amount":   order.Amount,
        },
    })
}

// PayPage 渲染支付页面（单页应用，由前端 JS 加载数据）
func PayPage(c *gin.Context) {
    // 返回内嵌的 index.html
    c.FileFromFS("web/index.html", http.FS(webFiles))
}

// GetOrderInfo 前端 AJAX 获取订单详情
func GetOrderInfo(c *gin.Context) {
    tradeID := c.Param("order_id")
    order, err := GetOrderByTradeID(tradeID)
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "订单不存在"})
        return
    }

    // 隐藏敏感字段，只返回前端需要的信息
    c.JSON(http.StatusOK, gin.H{
        "trade_id":   order.TradeID,
        "address":    order.Address,
        "amount":     order.Amount,
        "token":      order.Token,
        "network":    order.Chain,
        "expired_at": order.ExpiredAt.Unix(),
        "status":     order.Status,
    })
}

// GetOrderStatus 前端 AJAX 轮询支付状态
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
