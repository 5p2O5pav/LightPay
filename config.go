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
    BSC      BSCConfig      `yaml:"bsc"`
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

type RealtimeConfig struct {
    Provider       string            `yaml:"provider"`
    URL            string            `yaml:"url"`
    TimeoutSeconds int               `yaml:"timeout_seconds"`
    RetryCount     int               `yaml:"retry_count"`
    CacheSeconds   int               `yaml:"cache_seconds"`
    TokenIds       map[string]string `yaml:"token_ids"`
}

type PricingConfig struct {
    DefaultCurrency string             `yaml:"default_currency"`
    MarkupPercent   float64            `yaml:"markup_percent"`
    Mode            string             `yaml:"mode"`
    ManualRates     map[string]float64 `yaml:"manual_rates"`
    Realtime        RealtimeConfig     `yaml:"realtime"`
}

func LoadConfig(path string) *Config {
    data, err := os.ReadFile(path)
    if err != nil {
        panic(err)
    }
    config := &Config{}
    err = yaml.Unmarshal(data, config)
    if err != nil {
        panic(err)
    }
    // 不再设置任何默认值，所有字段必须由 yaml 文件提供
    return config
}
