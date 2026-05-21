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

type TelegramConfig struct {
    Enabled  bool   `yaml:"enabled"`
    BotToken string `yaml:"bot_token"`
    ChatID   int64  `yaml:"chat_id"`
}

type PricingConfig struct {
    DefaultCurrency string  `yaml:"default_currency"`
    MarkupPercent   float64 `yaml:"markup_percent"`
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
    }
    err = yaml.Unmarshal(data, config)
    if err != nil {
        panic(err)
    }
    return config
}
