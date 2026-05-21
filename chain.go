package main

import "time"

type ChainHandler interface {
    Name() string
    GetWalletAddresses() []string
    SelectWallet(orderID string) (string, error)
    FetchRecentTransactions(address, token string, since time.Time) ([]IncomingTx, error)   // 增加 token
    EnsureAddressListener(address, token string)                                            // 增加 token
}

type IncomingTx struct {
    TxID   string
    To     string
    Amount int64   // 内部精度（3位）
    Time   time.Time
}
