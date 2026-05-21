package main

import (
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "strconv"
    "time"
)

type TronChain struct {
    config TronConfig
}

type TronResponse struct {
    Data []struct {
        TransactionID string `json:"transaction_id"`
        TokenInfo     struct {
            Symbol string `json:"symbol"`
        } `json:"token_info"`
        From  string `json:"from"`
        To    string `json:"to"`
        Value string `json:"value"` // 十进制字符串，已含精度
        Type  string `json:"type"`
        BlockTimestamp int64 `json:"block_timestamp"`
    } `json:"data"`
    Meta struct{} `json:"meta"`
}

func (t *TronChain) Name() string { return "tron" }
func (t *TronChain) GetWalletAddresses() []string { return t.config.Wallets }
func (t *TronChain) SelectWallet(orderID string) (string, error) {
    return SelectWalletFromList(t.config.Wallets, orderID), nil
}

func (t *TronChain) FetchRecentTransactions(address string, since time.Time) ([]IncomingTx, error) {
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
    var tr TronResponse
    json.Unmarshal(body, &tr)

    var txs []IncomingTx
    for _, d := range tr.Data {
        if d.To == address && d.TokenInfo.Symbol == "USDT" {
            amountFloat, err := strconv.ParseFloat(d.Value, 64)
            if err != nil {
                continue
            }
            // 转为最小单位（USDT 6 位小数）
            amountInt := int64(amountFloat * 1e6)
            txs = append(txs, IncomingTx{
                TxID:   d.TransactionID,
                To:     d.To,
                Amount: amountInt,
                Time:   time.Unix(d.BlockTimestamp/1000, 0),
            })
        }
    }
    return txs, nil
}

func (t *TronChain) EnsureAddressListener(address string) {
    ensureListenerForChain(t, address)
}
