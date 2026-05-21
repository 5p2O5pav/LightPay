package main

import (
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "strconv"
    "time"
)

type TronUSDTChain struct {
    config TronConfig
}

type TronUSDTResponse struct {
    Data []struct {
        TransactionID string `json:"transaction_id"`
        TokenInfo     struct {
            Symbol string `json:"symbol"`
        } `json:"token_info"`
        From  string `json:"from"`
        To    string `json:"to"`
        Value string `json:"value"` // 十进制字符串，已含精度（6位）
        Type  string `json:"type"`
        BlockTimestamp int64 `json:"block_timestamp"`
    } `json:"data"`
}

func (t *TronUSDTChain) Name() string { return "tron" }
func (t *TronUSDTChain) GetWalletAddresses() []string { return t.config.Wallets }
func (t *TronUSDTChain) SelectWallet(orderID string) (string, error) {
    return SelectWalletFromList(t.config.Wallets, orderID), nil
}

func (t *TronUSDTChain) FetchRecentTransactions(address, token string, since time.Time) ([]IncomingTx, error) {
    if token != "usdt" {
        return nil, nil
    }
    url := fmt.Sprintf("https://api.trongrid.io/v1/accounts/%s/transactions/trc20?limit=50&min_timestamp=%d",
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

    var tr TronUSDTResponse
    json.Unmarshal(body, &tr)

    var txs []IncomingTx
    for _, d := range tr.Data {
        if d.To == address && d.TokenInfo.Symbol == "USDT" {
            amountFloat, err := strconv.ParseFloat(d.Value, 64)
            if err != nil {
                continue
            }
            // 链上精度 6 位，内部精度 3 位，除以 1000
            amountInt := int64(amountFloat * 1000) // 因为 amountFloat 是带6位小数的，乘1000得到3位内部单位
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

func (t *TronUSDTChain) EnsureAddressListener(address, token string) {
    ensureListenerForChain(t, address, token)
}
