package main

import (
    "os"
    "gopkg.in/yaml.v3"
)

type Config struct {
    Server   ServerConfig   `yaml:"server"`
    Database string         `yaml:"database"`
    ApiToken string         `yaml:"api_token"`
    Tron     TronConfig     `yaml:"tron"`
    Polygon  PolygonConfig  `yaml:"polygon"`
    BSC      BSCConfig      `yaml:"bsc"`   // 新增 BSC 配置
    Telegram TelegramConfig `yaml:"telegram"`
    Pricing  PricingConfig  `yaml:"pricing"`
}

type ServerConfig struct {
    Listen string `yaml:"listen"`
    Domain string `yaml:"domain"`
}

type TronConfig struct {
    ApiKey  string   `yaml:"api_key"`
    Network string   `yaml:"network"`
    Wallets []string `yaml:"wallets"`
}

type PolygonConfig struct {
    RPCURL       string   `yaml:"rpc_url"`
    USDTContract string   `yaml:"usdt_contract"`
    Wallets      []string `yaml:"wallets"`
}

type BSCConfig struct {
    RPCURL       string   `yaml:"rpc_url"`
    USDTContract string   `yaml:"usdt_contract"`
    Wallets      []string `yaml:"wallets"`
}

type TelegramConfig struct {
    Enabled  bool   `yaml:"enabled"`
    BotToken string `yaml:"bot_token"`
    ChatID   int64  `yaml:"chat_id"`
}

type PricingConfig struct {
    DefaultCurrency string            `yaml:"default_currency"`
    MarkupPercent   float64           `yaml:"markup_percent"`
    Mode            string            `yaml:"mode"`          // manual / realtime
    ManualRates     map[string]float64 `yaml:"manual_rates"` // 如 "CNY_USDT": 7.25
    Realtime        struct {
        Provider     string `yaml:"provider"`
        CacheSeconds int    `yaml:"cache_seconds"`
    } `yaml:"realtime"`
}

func LoadConfig(path string) *Config {
    data, err := os.ReadFile(path)
    if err != nil {
        panic(err)
    }
    config := &Config{
        Server: ServerConfig{
            Listen: ":8080",
        },
        Database: "data/lightpay.db",
        // 设置默认值
        Pricing: PricingConfig{
            Mode: "manual",
            Realtime: struct {
                Provider     string `yaml:"provider"`
                CacheSeconds int    `yaml:"cache_seconds"`
            }{
                Provider:     "coingecko",
                CacheSeconds: 60,
            },
        },
    }
    err = yaml.Unmarshal(data, config)
    if err != nil {
        panic(err)
    }
    return config
}
