package main

import (
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "time"
)

type TronTRXChain struct {
    config TronConfig
}

type TronTRXTransaction struct {
    Data []struct {
        TxID string `json:"txID"`
        RawData struct {
            Contract []struct {
                Parameter struct {
                    Value struct {
                        Amount    int64  `json:"amount"`      // 单位 SUN
                        ToAddress string `json:"to_address"`
                    } `json:"value"`
                } `json:"parameter"`
            } `json:"contract"`
            Timestamp int64 `json:"timestamp"`
        } `json:"raw_data"`
    } `json:"data"`
}

func (t *TronTRXChain) Name() string { return "tron" }
func (t *TronTRXChain) GetWalletAddresses() []string { return t.config.Wallets }
func (t *TronTRXChain) SelectWallet(orderID string) (string, error) {
    return SelectWalletFromList(t.config.Wallets, orderID), nil
}

func (t *TronTRXChain) FetchRecentTransactions(address, token string, since time.Time) ([]IncomingTx, error) {
    if token != "trx" {
        return nil, nil
    }
    url := fmt.Sprintf("https://api.trongrid.io/v1/accounts/%s/transactions?limit=50&min_timestamp=%d",
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

    var trx TronTRXTransaction
    json.Unmarshal(body, &trx)

    var txs []IncomingTx
    for _, tx := range trx.Data {
        for _, contract := range tx.RawData.Contract {
            // 普通转账合约类型
            if contract.Parameter.Value.ToAddress == address {
                amountSun := contract.Parameter.Value.Amount // 单位 SUN (6位精度)
                // 转为内部3位精度：除以 1000
                amountInternal := amountSun / 1000
                txs = append(txs, IncomingTx{
                    TxID:   tx.TxID,
                    To:     address,
                    Amount: amountInternal,
                    Time:   time.Unix(tx.RawData.Timestamp/1000, 0),
                })
            }
        }
    }
    return txs, nil
}

func (t *TronTRXChain) EnsureAddressListener(address, token string) {
    ensureListenerForChain(t, address, token)
}
