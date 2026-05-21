package main

import (
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "time"
)

type TronChain struct {
    config TronConfig
}

type TronConfig struct {
    ApiKey  string   `yaml:"api_key"`
    Network string   `yaml:"network"`
    Wallets []string `yaml:"wallets"`
}

func (t *TronChain) Name() string { return "tron" }

func (t *TronChain) GetWalletAddresses() []string {
    return t.config.Wallets
}

func (t *TronChain) SelectWallet(orderID string) (string, error) {
    // 公共的简单轮选，也可以在 config 中预设选择逻辑
    return SelectWalletFromList(t.config.Wallets, orderID), nil
}

func (t *TronChain) FetchRecentTransactions(address string, since time.Time) ([]IncomingTx, error) {
    // 调用 TronGrid API，用 since 过滤时间
    url := fmt.Sprintf("https://api.trongrid.io/v1/accounts/%s/transactions/trc20?limit=20&min_timestamp=%d",
        address, since.UnixMilli())
    req, _ := http.NewRequest("GET", url, nil)
    req.Header.Set("TRON-PRO-API-KEY", t.config.ApiKey)
    client := &http.Client{Timeout: 10 * time.Second}
    resp, err := client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    body, _ := io.ReadAll(resp.Body)
    var result TronResponse
    json.Unmarshal(body, &result)
    // 解析成 []IncomingTx
    return parseTronTransactions(result, address), nil
}

func (t *TronChain) EnsureAddressListener(address string) {
    ensureListenerForChain(t, address)
}

// 辅助：解析 TRON 返回的交易
func parseTronTransactions(resp TronResponse, targetAddr string) []IncomingTx {
    // 实现省略，提取 to == targetAddr 且 token='USDT' 的交易
    // 返回 []IncomingTx
}
