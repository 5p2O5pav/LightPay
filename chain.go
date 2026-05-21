package main

import "time"

type ChainHandler interface {
    Name() string
    GetWalletAddresses() []string
    SelectWallet(orderID string) (string, error)
    FetchRecentTransactions(address string, since time.Time) ([]IncomingTx, error)
    EnsureAddressListener(address string)
}

type IncomingTx struct {
    TxID   string
    To     string
    Amount int64   // 最小单位
    Time   time.Time
}
