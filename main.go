package main

import (
    "context"
    "embed"
    "io/fs"
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/gin-gonic/gin"
)

//go:embed web
var webFiles embed.FS

var (
    chainRegistry = map[string]ChainHandler{}
    config        *Config
)

func registerChain(h ChainHandler) {
    chainRegistry[h.Name()] = h
}

func main() {
    config = LoadConfig("config.yaml")
    InitDB(config)

    // 注册链
    if len(config.Tron.Wallets) > 0 {
        tron := &TronChain{config: config.Tron}
        registerChain(tron)
        log.Println("注册链: tron")
    }
    if len(config.Polygon.Wallets) > 0 {
        poly := NewPolygonChain(config.Polygon)
        registerChain(poly)
        log.Println("注册链: polygon")
    }

    if config.Telegram.Enabled {
        go StartTelegramBot(config)
    }
    go startExpireCleaner()
    go StartSSHManagement(config)

    router := gin.Default()
    api := router.Group("/payments/gmpay/v1")
    {
        api.POST("/order/create-transaction", CreateTransaction)
        api.GET("/order/query", QueryOrder)
    }
    router.GET("/pay/:order_id", PayPage)
    router.GET("/api/order/:order_id", GetOrderInfo)
    router.GET("/api/order/:order_id/status", GetOrderStatus)
    // 修改静态文件路由
    webSub, _ := fs.Sub(webFiles, "web")
    router.StaticFS("/static", http.FS(webSub))

    srv := &http.Server{
        Addr:    config.Server.Listen,
        Handler: router,
    }
    go func() {
        log.Printf("HTTP 服务启动于 %s", config.Server.Listen)
        if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            log.Fatalf("HTTP 服务异常: %v", err)
        }
    }()

    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit
    log.Println("正在关闭服务...")
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    srv.Shutdown(ctx)
    log.Println("服务已安全退出")
}
