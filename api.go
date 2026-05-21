package main

import (
	"fmt"
	"net/http"
	"time"
	
	"github.com/gin-gonic/gin"
)

// 创建交易 - 完全兼容Epusdt接口
func CreateTransaction(c *gin.Context) {
	var req map[string]interface{}
	if err := c.BindJSON(&req); err != nil {
		c.JSON(400, gin.H{"status_code": 400, "message": "参数错误"})
		return
	}
	
	// 验证签名
	params := make(map[string]string)
	for k, v := range req {
		if k != "signature" {
			params[k] = fmt.Sprintf("%v", v)
		}
	}
	
	expectedSign := MakeSignature(params, config.Tron.ApiKey)
	if req["signature"] != expectedSign {
		c.JSON(401, gin.H{"status_code": 401, "message": "签名验证失败"})
		return
	}
	
	orderID := req["order_id"].(string)
	amount := req["amount"].(float64)
	currency := req["currency"].(string)
	network := req["network"].(string)
	
	// 获取可用钱包地址
	address := getAvailableAddress(network)
	if address == "" {
		c.JSON(500, gin.H{"status_code": 500, "message": "无可用钱包"})
		return
	}
	
	// 汇率转换
	if currency != "USD" {
		rate, err := GetExchangeRate(currency, "USD")
		if err != nil {
			c.JSON(500, gin.H{"status_code": 500, "message": "汇率获取失败"})
			return
		}
		amount = amount * rate
	}
	
	// 创建订单
	if err := CreateOrder(address, amount, orderID); err != nil {
		c.JSON(500, gin.H{"status_code": 500, "message": err.Error()})
		return
	}
	
	// 保存订单到数据库
	SaveOrder(orderID, address, amount, currency)
	
	// 生成支付URL
	paymentURL := fmt.Sprintf("https://%s/pay/%s", config.Server.Domain, orderID)
	
	c.JSON(200, gin.H{
		"status_code": 200,
		"message":     "success",
		"data": gin.H{
			"trade_id":    generateTradeID(),
			"order_id":    orderID,
			"amount":      amount,
			"address":     address,
			"payment_url": paymentURL,
			"expired_at":  time.Now().Add(10 * time.Minute).Unix(),
		},
	})
}

// 查询订单状态
func QueryOrder(c *gin.Context) {
	orderID := c.Query("order_id")
	order := GetOrder(orderID)
	
	if order == nil {
		c.JSON(404, gin.H{"status_code": 404, "message": "订单不存在"})
		return
	}
	
	c.JSON(200, gin.H{
		"status_code": 200,
		"data": gin.H{
			"status": order.Status,
			"amount": order.Amount,
		},
	})
}
