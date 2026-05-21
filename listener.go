package main

import (
    "sync"
    "time"
)

var (
    listenerMap   = make(map[string]chan struct{})
    listenerMapMu sync.Mutex
)

func ensureListenerForChain(chain ChainHandler, address, token string) {
    key := chain.Name() + ":" + token + ":" + address
    listenerMapMu.Lock()
    defer listenerMapMu.Unlock()

    if _, exists := listenerMap[key]; exists {
        return
    }

    stop := make(chan struct{})
    listenerMap[key] = stop

    go func() {
        ticker := time.NewTicker(10 * time.Second)
        defer ticker.Stop()
        lastCheck := time.Now().Add(-30 * time.Second)

        for {
            select {
            case <-ticker.C:
                expireOrders()
                // 检查是否还有待支付订单（该地址+该币种）
                if !hasPendingOrdersForChainToken(chain.Name(), address, token) {
                    listenerMapMu.Lock()
                    delete(listenerMap, key)
                    listenerMapMu.Unlock()
                    return
                }

                txs, err := chain.FetchRecentTransactions(address, token, lastCheck)
                if err != nil {
                    continue
                }
                if len(txs) > 0 {
                    lastCheck = time.Now()
                }
                for _, tx := range txs {
                    order, err := GetPendingOrderByAddressAmountToken(chain.Name(), address, tx.Amount, token)
                    if err == nil && order != nil {
                        handlePaymentSuccess(chain.Name(), token, tx.TxID, address, tx.Amount)
                    }
                }
            case <-stop:
                return
            }
        }
    }()
}
