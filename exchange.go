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

// GetExchangeRate 返回 1 token = X currency
func GetExchangeRate(currency, token string) (float64, error) {
    currency = strings.ToUpper(currency)
    token = strings.ToUpper(token)
    cacheKey := currency + "_" + token

    rateCacheMu.RLock()
    cached, ok := rateCache[cacheKey]
    rateCacheMu.RUnlock()
    if ok && time.Now().Before(cached.expiresAt) {
        return cached.rate, nil
    }

    var rate float64
    var err error

    if config.Pricing.Mode == "manual" {
        mapKey := currency + "_" + token
        if r, ok := config.Pricing.ManualRates[mapKey]; ok {
            rate = r
        } else {
            return 0, fmt.Errorf("manual rate not found for %s/%s", currency, token)
        }
    } else {
        // 实时模式
        rate, err = fetchCoinGeckoRate(currency, token)
        if err != nil {
            return 0, err
        }
    }

    cacheSeconds := config.Pricing.Realtime.CacheSeconds
    if cacheSeconds <= 0 {
        cacheSeconds = 60
    }
    rateCacheMu.Lock()
    rateCache[cacheKey] = cachedRate{
        rate:      rate,
        expiresAt: time.Now().Add(time.Duration(cacheSeconds) * time.Second),
    }
    rateCacheMu.Unlock()
    return rate, nil
}

func fetchCoinGeckoRate(currency, token string) (float64, error) {
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
