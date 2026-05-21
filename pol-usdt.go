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

func (p *PolygonChain) FetchRecentTransactions(address, token string, since time.Time) ([]IncomingTx, error) {
    if token != "usdt" {
        return nil, nil
    }
    toAddr := common.HexToAddress(address)
    contract := common.HexToAddress(p.config.USDTContract)

    header, err := p.client.HeaderByNumber(context.Background(), nil)
    if err != nil {
        return nil, err
    }
    endBlock := header.Number.Uint64()
    startBlock := uint64(0)
    if endBlock > 5000 {
        startBlock = endBlock - 5000
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
        rawAmount := new(big.Int).SetBytes(vLog.Data)
        // Polygon USDT 精度 6，内部精度 3，缩放因子 = 1000
        internalAmount := new(big.Int).Div(rawAmount, big.NewInt(1000)).Int64()
        block, _ := p.client.BlockByNumber(context.Background(), big.NewInt(int64(vLog.BlockNumber)))
        blockTime := time.Unix(int64(block.Time()), 0)
        txs = append(txs, IncomingTx{
            TxID:   vLog.TxHash.Hex(),
            To:     address,
            Amount: internalAmount,
            Time:   blockTime,
        })
    }
    return txs, nil
}

func (p *PolygonChain) EnsureAddressListener(address, token string) {
    ensureListenerForChain(p, address, token)
}
