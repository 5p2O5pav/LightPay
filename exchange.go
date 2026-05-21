package main

import (
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "strings"
    "sync"
    "time"
)

var (
    rateCache   = make(map[string]cachedRate)
    rateCacheMu sync.RWMutex
)

type cachedRate struct {
    rate      float64
    expiresAt time.Time
}

// GetExchangeRate 返回 1 token = X currency (例如 1 USDT = 7.25 CNY)
func GetExchangeRate(currency, token string) (float64, error) {
    currency = strings.ToUpper(currency)
    token = strings.ToUpper(token)
    cacheKey := currency + "_" + token

    // 读缓存
    rateCacheMu.RLock()
    cached, ok := rateCache[cacheKey]
    rateCacheMu.RUnlock()
    if ok && time.Now().Before(cached.expiresAt) {
        return cached.rate, nil
    }

    var rate float64
    var err error

    if config.Pricing.Mode == "manual" {
        // 手动模式
        mapKey := currency + "_" + token
        if r, ok := config.Pricing.ManualRates[mapKey]; ok {
            rate = r
        } else {
            return 0, fmt.Errorf("manual rate not found for %s/%s", currency, token)
        }
    } else {
        // 实时模式（默认 CoinGecko）
        rate, err = fetchCoinGeckoRate(currency, token)
        if err != nil {
            return 0, err
        }
    }

    // 写入缓存
    rateCacheMu.Lock()
    rateCache[cacheKey] = cachedRate{
        rate:      rate,
        expiresAt: time.Now().Add(time.Duration(config.Pricing.Realtime.CacheSeconds) * time.Second),
    }
    rateCacheMu.Unlock()
    return rate, nil
}

// fetchCoinGeckoRate 从 CoinGecko 获取实时价格
// 返回 1 token = X currency
func fetchCoinGeckoRate(currency, token string) (float64, error) {
    // token 映射到 CoinGecko id
    idMap := map[string]string{
        "USDT": "tether",
        "TRX":  "tron",
        "BUSD": "binance-usd",
    }
    coinID, ok := idMap[token]
    if !ok {
        return 0, fmt.Errorf("unsupported token for realtime rate: %s", token)
    }

    url := fmt.Sprintf("https://api.coingecko.com/api/v3/simple/price?ids=%s&vs_currencies=%s", coinID, currency)
    resp, err := http.Get(url)
    if err != nil {
        return 0, err
    }
    defer resp.Body.Close()
    body, _ := io.ReadAll(resp.Body)
    var result map[string]map[string]float64
    if err := json.Unmarshal(body, &result); err != nil {
        return 0, err
    }
    if priceMap, ok := result[coinID]; ok {
        if price, ok := priceMap[currency]; ok {
            return price, nil
        }
    }
    return 0, fmt.Errorf("rate not found for %s/%s", token, currency)
}
