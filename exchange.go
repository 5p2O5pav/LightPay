package main

import (
    "encoding/json"
    "fmt"
    "io"
    "log"
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

    // 读缓存
    rateCacheMu.RLock()
    cached, ok := rateCache[cacheKey]
    rateCacheMu.RUnlock()
    if ok && time.Now().Before(cached.expiresAt) {
        return cached.rate, nil
    }

    var rate float64
    var err error

    switch config.Pricing.Mode {
    case "manual":
        mapKey := currency + "_" + token
        r, ok := config.Pricing.ManualRates[mapKey]
        if !ok {
            return 0, fmt.Errorf("manual rate not found for %s/%s", currency, token)
        }
        rate = r

    case "realtime":
        rate, err = fetchRealtimeRate(currency, token)
        if err != nil {
            // 实时获取失败，不自动 fallback 到手动（除非你愿意，但这里按原则不隐藏问题）
            return 0, fmt.Errorf("realtime rate failed: %v", err)
        }

    default:
        return 0, fmt.Errorf("unknown pricing mode: %s (must be 'manual' or 'realtime')", config.Pricing.Mode)
    }

    // 写入缓存（仅当配置了缓存时间 > 0 时）
    if config.Pricing.Realtime.CacheSeconds > 0 {
        rateCacheMu.Lock()
        rateCache[cacheKey] = cachedRate{
            rate:      rate,
            expiresAt: time.Now().Add(time.Duration(config.Pricing.Realtime.CacheSeconds) * time.Second),
        }
        rateCacheMu.Unlock()
    }

    return rate, nil
}

// fetchRealtimeRate 从配置的 URL 获取实时汇率，缺失必要配置时直接报错
func fetchRealtimeRate(currency, token string) (float64, error) {
    cfg := config.Pricing.Realtime

    // 检查必要字段
    if cfg.URL == "" {
        return 0, fmt.Errorf("realtime.url is empty in config.yaml")
    }
    if cfg.TimeoutSeconds <= 0 {
        return 0, fmt.Errorf("realtime.timeout_seconds must be > 0 in config.yaml")
    }
    if cfg.RetryCount < 0 {
        // RetryCount 可以为 0，表示不重试，但负数不允许
        return 0, fmt.Errorf("realtime.retry_count must be >= 0 in config.yaml")
    }
    if len(cfg.TokenIds) == 0 {
        return 0, fmt.Errorf("realtime.token_ids is empty in config.yaml")
    }

    tokenID, ok := cfg.TokenIds[token]
    if !ok {
        return 0, fmt.Errorf("no token_id mapping for %s in config.yaml", token)
    }

    // 构造请求 URL
    url := fmt.Sprintf("%s?ids=%s&vs_currencies=%s", cfg.URL, tokenID, strings.ToLower(currency))

    client := &http.Client{
        Timeout: time.Duration(cfg.TimeoutSeconds) * time.Second,
    }

    var resp *http.Response
    var err error
    for i := 0; i <= cfg.RetryCount; i++ {
        resp, err = client.Get(url)
        if err == nil && resp.StatusCode == http.StatusOK {
            break
        }
        if resp != nil {
            resp.Body.Close()
        }
        if i < cfg.RetryCount {
            time.Sleep(time.Duration(i+1) * 500 * time.Millisecond)
        }
    }
    if err != nil {
        return 0, fmt.Errorf("HTTP request failed after %d retries: %v", cfg.RetryCount, err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return 0, fmt.Errorf("API returned HTTP %d", resp.StatusCode)
    }

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return 0, fmt.Errorf("read response body: %v", err)
    }

    var result map[string]map[string]float64
    if err := json.Unmarshal(body, &result); err != nil {
        return 0, fmt.Errorf("parse JSON: %v", err)
    }

    priceMap, ok := result[tokenID]
    if !ok {
        return 0, fmt.Errorf("token ID %s not found in API response", tokenID)
    }

    rate, ok := priceMap[strings.ToLower(currency)]
    if !ok {
        return 0, fmt.Errorf("currency %s not found in API response", currency)
    }

    return rate, nil
}
