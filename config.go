package main

import (
	"os"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database string         `yaml:"database"`
	Tron     TronConfig     `yaml:"tron"`
	Telegram TelegramConfig `yaml:"telegram"`
	Pricing  PricingConfig  `yaml:"pricing"`
}

type ServerConfig struct {
	Listen string `yaml:"listen"` // 如 ":8080"
	Domain string `yaml:"domain"`
}

type TronConfig struct {
	ApiKey     string   `yaml:"api_key"`     // TronGrid API Key
	Network    string   `yaml:"network"`     // mainnet/shasta
	Wallets    []string `yaml:"wallets"`     // 监控的钱包地址列表
}

type TelegramConfig struct {
	Enabled  bool   `yaml:"enabled"`
	BotToken string `yaml:"bot_token"`
	ChatID   string `yaml:"chat_id"`
}

type PricingConfig struct {
	DefaultCurrency string  `yaml:"default_currency"` // USD, CNY, EUR
	MarkupPercent   float64 `yaml:"markup_percent"`   // 加价百分比
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
