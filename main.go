package main

import (
    "context"
    "embed"
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"

    "github.com/gin-gonic/gin"
)

// 嵌入前端支付页面
//
//go:embed web/*
var webFiles embed.FS

// 全局链注册表
var chainRegistry = map[string]ChainHandler{}

func registerChain(h ChainHandler) {
    chainRegistry[h.Name()] = h
}

func main() {
    // 1. 加载配置
    config := LoadConfig("config.yaml")

    // 2. 初始化数据库
    InitDB(config)

    // 3. 初始化各条链并注册
    // 3.1 TRON
    if config.Tron.Wallets != nil && len(config.Tron.Wallets) > 0 {
        tron := &TronChain{
            config: TronConfig{
                ApiKey:  config.Tron.ApiKey,
                Network: config.Tron.Network,
                Wallets: config.Tron.Wallets,
            },
        }
        registerChain(tron)
        log.Println("已注册链: tron")
    }

    // 3.2 Polygon
    if config.Polygon.Wallets != nil && len(config.Polygon.Wallets) > 0 {
        polygon := NewPolygonChain(config.Polygon) // 假设构造函数
        registerChain(polygon)
        log.Println("已注册链: polygon")
    }

    // 未来新增链时，只需在这里添加几行，例如：
    // if config.BSC.Wallets != nil { ... }

    // 4. 启动 Telegram 机器人（可选）
    if config.Telegram.Enabled {
        go StartTelegramBot(config)
    }

    // 5. 启动 SSH 管理面板（后台 goroutine）
    go StartSSHManagement(config)

    // 6. 设置 Gin 路由
    router := gin.Default()

    // Epusdt 兼容 API
    api := router.Group("/payments/gmpay/v1")
    {
        api.POST("/order/create-transaction", CreateTransaction)
        api.GET("/order/query", QueryOrder)
    }

    // 支付页面及状态查询（内嵌前端）
    router.GET("/pay/:order_id", PayPage)
    router.GET("/api/order/:order_id", GetOrderInfo)
    router.GET("/api/order/:order_id/status", GetOrderStatus)

    // 静态资源（前端 JS/CSS/图片等）
    router.StaticFS("/static", http.FS(webFiles))

    // 7. 启动 HTTP 服务
    srv := &http.Server{
        Addr:    config.Server.Listen, // 例如 ":8080"
        Handler: router,
    }

    go func() {
        log.Printf("HTTP 服务启动于 %s", config.Server.Listen)
        if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            log.Fatalf("HTTP 服务异常: %v", err)
        }
    }()

    // 8. 优雅退出
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit
    log.Println("正在关闭服务...")

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    srv.Shutdown(ctx)
    log.Println("服务已安全退出")
}
