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
    // 原 chainRegistry 保留兼容（但不再使用），改用 chainTokenRegistry
    chainTokenRegistry = map[string]ChainHandler{}
    config *Config
)

func registerChainToken(handler ChainHandler, chain, token string) {
    key := chain + ":" + token
    chainTokenRegistry[key] = handler
}

func main() {
    config = LoadConfig("config.yaml")
    InitDB(config)

    // 注册 Tron USDT
    if len(config.Tron.Wallets) > 0 {
        tronUSDT := &TronUSDTChain{config: config.Tron}
        registerChainToken(tronUSDT, "tron", "usdt")
        log.Println("注册链: tron (USDT)")
    }
    // 注册 Tron TRX（如果需要）
    if len(config.Tron.Wallets) > 0 {
        tronTRX := &TronTRXChain{config: config.Tron}
        registerChainToken(tronTRX, "tron", "trx")
        log.Println("注册链: tron (TRX)")
    }
    // 注册 Polygon USDT
    if len(config.Polygon.Wallets) > 0 {
        polygon := NewPolygonChain(config.Polygon)
        registerChainToken(polygon, "polygon", "usdt")
        log.Println("注册链: polygon (USDT)")
    }
    // 注册 BSC USDT
    if len(config.BSC.Wallets) > 0 {
        bsc := NewBSCChain(config.BSC)
        registerChainToken(bsc, "bsc", "usdt")
        log.Println("注册链: bsc (USDT)")
    }

    if config.Telegram.Enabled {
        go StartTelegramBot(config)
    }
    go startExpireCleaner()
    go StartSSHManagement(config)

    router := gin.Default()

    // 根路径显示 fake.html
    router.GET("/", func(c *gin.Context) {
        data, err := webFiles.ReadFile("web/fake.html")
        if err != nil {
            c.String(http.StatusNotFound, "fake.html not found")
            return
        }
        c.Data(http.StatusOK, "text/html; charset=utf-8", data)
    })

    api := router.Group("/payments/gmpay/v1")
    {
        api.POST("/order/create-transaction", CreateTransaction)
        api.GET("/order/query", QueryOrder)
    }
    router.GET("/pay/:order_id", PayPage)
    router.GET("/api/order/:order_id", GetOrderInfo)
    router.GET("/api/order/:order_id/status", GetOrderStatus)

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
