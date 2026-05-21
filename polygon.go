package main

import (
    "context"
    "math/big"
    "time"
    "github.com/ethereum/go-ethereum/common"
    "github.com/ethereum/go-ethereum/crypto"
    "github.com/ethereum/go-ethereum/ethclient"
)

type PolygonChain struct {
    client *ethclient.Client
    config PolygonConfig
}

type PolygonConfig struct {
    RPCURL       string   `yaml:"rpc_url"`
    USDTContract string   `yaml:"usdt_contract"`
    Wallets      []string `yaml:"wallets"`
}

func (p *PolygonChain) Name() string { return "polygon" }

func (p *PolygonChain) GetWalletAddresses() []string { return p.config.Wallets }

func (p *PolygonChain) SelectWallet(orderID string) (string, error) {
    return SelectWalletFromList(p.config.Wallets, orderID), nil
}

func (p *PolygonChain) FetchRecentTransactions(address string, since time.Time) ([]IncomingTx, error) {
    toAddr := common.HexToAddress(address)
    contract := common.HexToAddress(p.config.USDTContract)
    // 构建查询：只查 Transfer to 我们的地址，且区块时间>since
    // 这里简化，实际可通过区块号范围过滤
    query := ethereum.FilterQuery{
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
        amountFloat := float64(amount.Int64()) / 1e6
        txs = append(txs, IncomingTx{
            TxID:   vLog.TxHash.Hex(),
            To:     address,
            Amount: amountFloat,
            Time:   time.Now(), // 可从区块头获取准确时间
        })
    }
    return txs, nil
}

func (p *PolygonChain) EnsureAddressListener(address string) {
    ensureListenerForChain(p, address)
}
