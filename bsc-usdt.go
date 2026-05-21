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

type BSCChain struct {
    client *ethclient.Client
    config BSCConfig
}

func NewBSCChain(cfg BSCConfig) *BSCChain {
    client, err := ethclient.Dial(cfg.RPCURL)
    if err != nil {
        panic(err)
    }
    return &BSCChain{client: client, config: cfg}
}

func (b *BSCChain) Name() string { return "bsc" }
func (b *BSCChain) GetWalletAddresses() []string { return b.config.Wallets }
func (b *BSCChain) SelectWallet(orderID string) (string, error) {
    return SelectWalletFromList(b.config.Wallets, orderID), nil
}

func (b *BSCChain) FetchRecentTransactions(address, token string, since time.Time) ([]IncomingTx, error) {
    if token != "usdt" {
        return nil, nil
    }
    toAddr := common.HexToAddress(address)
    contract := common.HexToAddress(b.config.USDTContract)

    header, _ := b.client.HeaderByNumber(context.Background(), nil)
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
    logs, err := b.client.FilterLogs(context.Background(), query)
    if err != nil {
        return nil, err
    }

    var txs []IncomingTx
    for _, vLog := range logs {
        rawAmount := new(big.Int).SetBytes(vLog.Data)
        // BSC USDT 精度 18，内部精度 3，缩放因子 = 1e15
        internalAmount := new(big.Int).Div(rawAmount, big.NewInt(1e15)).Int64()
        block, _ := b.client.BlockByNumber(context.Background(), big.NewInt(int64(vLog.BlockNumber)))
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

func (b *BSCChain) EnsureAddressListener(address, token string) {
    ensureListenerForChain(b, address, token)
}
