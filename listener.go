package main

import (
    "sync"
    "time"
)

var (
    listenerMap   = make(map[string]chan struct{})
    listenerMapMu sync.Mutex
)

// ensureListenerForChain 保证链的地址被监听
func ensureListenerForChain(chain ChainHandler, address string) {
    key := chain.Name() + ":" + address
    listenerMapMu.Lock()
    defer listenerMapMu.Unlock()

    if _, exists := listenerMap[key]; exists {
        return
    }

    stop := make(chan struct{})
    listenerMap[key] = stop

    go func() {
        ticker := time.NewTicker(3 * time.Second)
        defer ticker.Stop()
        lastCheck := time.Now().Add(-30 * time.Second)

        for {
            select {
            case <-ticker.C:
                // 先清理过期订单
                expireOrders()
                // 检查是否还有待支付订单
                if !hasPendingOrdersForChain(chain.Name(), address) {
                    listenerMapMu.Lock()
                    delete(listenerMap, key)
                    listenerMapMu.Unlock()
                    return
                }

                txs, err := chain.FetchRecentTransactions(address, lastCheck)
                if err != nil {
                    continue
                }
                if len(txs) > 0 {
                    lastCheck = time.Now()
                }
                for _, tx := range txs {
                    // 精确匹配金额（最小单位）
                    order, err := GetPendingOrderByAddressAndAmount(chain.Name(), address, tx.Amount)
                    if err == nil && order != nil {
                        handlePaymentSuccess(chain.Name(), tx.TxID, address, tx.Amount)
                    }
                }
            case <-stop:
                return
            }
        }
    }()
}
