package main

import (
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "sync"
    "time"
)

// 监听器管理
var (
    listeners   = map[string]chan struct{}{}
    listenersMu sync.Mutex
)

// 确保为指定地址启动专属监听（如果尚未启动）
func ensureListenerForAddress(address string, cfg *Config) {
    listenersMu.Lock()
    defer listenersMu.Unlock()

    if _, exists := listeners[address]; exists {
        return // 已在监听
    }

    stop := make(chan struct{})
    listeners[address] = stop

    go func() {
        // 有活跃订单时用较短间隔，减少延迟
        ticker := time.NewTicker(3 * time.Second)
        defer ticker.Stop()

        for {
            select {
            case <-ticker.C:
                checkTronTransactions(address, cfg)
                // 如果该地址不再有未支付订单，退出监听
                if !hasPendingOrdersForAddress(address) {
                    listenersMu.Lock()
                    delete(listeners, address)
                    listenersMu.Unlock()
                    return
                }
            case <-stop:
                return
            }
        }
    }()
}

// 查询地址最近交易，匹配订单
func checkTronTransactions(address string, cfg *Config) {
    url := fmt.Sprintf(
        "https://api.trongrid.io/v1/accounts/%s/transactions/trc20?limit=10",
        address,
    )
    req, err := http.NewRequest("GET", url, nil)
    if err != nil {
        return
    }
    req.Header.Set("TRON-PRO-API-KEY", cfg.Tron.ApiKey)

    client := &http.Client{Timeout: 10 * time.Second}
    resp, err := client.Do(req)
    if err != nil {
        return
    }
    defer resp.Body.Close()

    body, _ := io.ReadAll(resp.Body)
    var result map[string]interface{}
    json.Unmarshal(body, &result)

    // 解析交易并匹配订单（这里需要你已有的订单匹配逻辑）
    processTronTransactions(address, result)
}

// 检查地址是否仍有未支付订单（需从订单数据查询）
func hasPendingOrdersForAddress(address string) bool {
    // 查询数据库或内存映射，返回该地址是否还有 status=pending 的订单
    return GetPendingOrderCountForAddress(address) > 0
}
