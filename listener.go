package main

import (
    "sync"
    "time"
)

// 全局监听器管理，key 为 "chain:address"
var (
    listenerMap   = make(map[string]chan struct{})
    listenerMapMu sync.Mutex
)

// ensureListenerForChain 确保指定链的指定地址正在被监听（不存在则启动）
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
        // 有订单时轮询间隔3秒
        ticker := time.NewTicker(3 * time.Second)
        defer ticker.Stop()
        // 记录上次查询时间，避免重复处理旧交易
        lastCheck := time.Now().Add(-30 * time.Second)

        for {
            select {
            case <-ticker.C:
                // 1. 检查该地址是否仍有 pending 订单
                if !hasPendingOrdersForChain(chain.Name(), address) {
                    // 无订单则退出监听
                    listenerMapMu.Lock()
                    delete(listenerMap, key)
                    listenerMapMu.Unlock()
                    return
                }

                // 2. 拉取该地址最近的交易
                txs, err := chain.FetchRecentTransactions(address, lastCheck)
                if err != nil {
                    // 可以记录日志
                    continue
                }
                if len(txs) > 0 {
                    // 更新最后检查时间为当前时间
                    lastCheck = time.Now()
                }

                // 3. 对每笔交易尝试匹配订单
                for _, tx := range txs {
                    if matchOrderByChain(chain.Name(), address, tx.Amount) {
                        handlePaymentSuccess(chain.Name(), tx.TxID, address, tx.Amount)
                    }
                }

            case <-stop:
                return
            }
        }
    }()
}
