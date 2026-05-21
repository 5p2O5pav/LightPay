package main

import (
    "context"
    "math/big"
    "time"

    "github.com/ethereum/go-ethereum"
    "github.com/ethereum/go-ethereum/common"
    "github.com/ethereum/go-ethereum/crypto"
    "github.com/ethereum/go-ethereum/ethclient"
)

type PolygonChain struct {
    client *ethclient.Client
    config PolygonConfig
}

func NewPolygonChain(cfg PolygonConfig) *PolygonChain {
    client, err := ethclient.Dial(cfg.RPCURL)
    if err != nil {
        panic(err)
    }
    return &PolygonChain{client: client, config: cfg}
}

func (p *PolygonChain) Name() string { return "polygon" }
func (p *PolygonChain) GetWalletAddresses() []string { return p.config.Wallets }
func (p *PolygonChain) SelectWallet(orderID string) (string, error) {
    return SelectWalletFromList(p.config.Wallets, orderID), nil
}

func (p *PolygonChain) FetchRecentTransactions(address string, since time.Time) ([]IncomingTx, error) {
    toAddr := common.HexToAddress(address)
    contract := common.HexToAddress(p.config.USDTContract)

    // 获取当前区块号
    header, err := p.client.HeaderByNumber(context.Background(), nil)
    if err != nil {
        return nil, err
    }
    endBlock := header.Number.Uint64()
    startBlock := endBlock - 500 // 粗略范围，可优化
    if startBlock < 0 {
        startBlock = 0
    }

    query := ethereum.FilterQuery{
        FromBlock: big.NewInt(int64(startBlock)),
        ToBlock:   big.NewInt(int64(endBlock)),
        Addresses: []common.Address{contract},
        Topics: [][]common.Hash{
            {crypto.Keccak256Hash([]byte("Transfer(address,address,uint256)"))},
            nil,
            {common.BytesToHash(toAddr.Bytes())},
        },
    }
    logs, err := p.client.FilterLogs(context.Background(), query)
    if err != nil {
        return nil, err
    }

    var txs []IncomingTx
    for _, vLog := range logs {
        amount := new(big.Int).SetBytes(vLog.Data)
        // 转为最小单位（USDT 6 位小数），避免大数溢出使用 Int64 安全转换
        amountInt := new(big.Int).Div(amount, big.NewInt(1e6)).Int64()
        block, err := p.client.BlockByNumber(context.Background(), big.NewInt(int64(vLog.BlockNumber)))
        var blockTime time.Time
        if err == nil {
            blockTime = time.Unix(int64(block.Time()), 0)
        } else {
            blockTime = time.Now()
        }
        txs = append(txs, IncomingTx{
            TxID:   vLog.TxHash.Hex(),
            To:     address,
            Amount: amountInt,
            Time:   blockTime,
        })
    }
    return txs, nil
}

func (p *PolygonChain) EnsureAddressListener(address string) {
    ensureListenerForChain(p, address)
}
