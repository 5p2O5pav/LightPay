package main

import (
	"embed"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
)

//go:embed web/*
var webFiles embed.FS

func main() {
	// 加载配置
	config := LoadConfig("config.yaml")
	
	// 初始化数据库
	InitDB(config)
	
	// 初始化TRON监听器
	go StartTronListener(config)
	
	// 初始化Telegram机器人
	if config.Telegram.Enabled {
		go StartTelegramBot(config)
	}
	
	// 启动SSH管理面板
	go StartSSHManagement(config)
	
	// 设置Gin路由
	router := gin.Default()
	
	// API接口 - 兼容Epusdt
	api := router.Group("/payments/gmpay/v1")
	{
		api.POST("/order/create-transaction", CreateTransaction)
		api.POST("/order/query", QueryOrder)
	}
	
	// 支付页面
	router.GET("/pay/:order_id", PayPage)
	router.GET("/api/order/:order_id", GetOrderInfo)
	router.GET("/api/order/:order_id/status", GetOrderStatus)
	
	// 静态文件
	router.StaticFS("/static", http.FS(webFiles))
	
	// 启动HTTP服务
	go func() {
		log.Printf("HTTP服务启动在 %s", config.Server.Listen)
		router.Run(config.Server.Listen)
	}()
	
	// 优雅退出
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("正在关闭服务...")
}
