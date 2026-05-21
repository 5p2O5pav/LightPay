package main

import (
    "crypto/md5"
    "encoding/hex"
    "fmt"
    "sort"
    "strings"
    "sync"
    "time"
    "math/big"
)

type Order struct {
    TradeID     string
    OrderID     string
    Chain       string
    Address     string
    Amount      int64   // 最小单位，例如 USDT * 1e6
    Currency    string
    Token       string
    NotifyURL   string
    RedirectURL string
    Status      string
    ExpiredAt   time.Time
    CreatedAt   time.Time
}

// 金额锁定内存映射（服务重启后从数据库恢复）
var (
    addressAmountMap = make(map[string]*LockedAmount)
    lockMutex        sync.RWMutex
)

type LockedAmount struct {
    Amount    int64
    OrderID   string
    ExpiresAt time.Time
}

// 生成签名（与 Epusdt 一致）
func MakeSignature(params map[string]string, token string) string {
    keys := make([]string, 0, len(params))
    for k := range params {
        if k != "signature" && params[k] != "" {
            keys = append(keys, k)
        }
    }
    sort.Strings(keys)
    var pairs []string
    for _, k := range keys {
        pairs = append(pairs, fmt.Sprintf("%s=%s", k, params[k]))
    }
    signStr := strings.Join(pairs, "&") + token
    hash := md5.Sum([]byte(signStr))
    return strings.ToLower(hex.EncodeToString(hash[:]))
}

// LockAmountWithIncrement 尝试锁定金额，模仿 Epusdt 的累加机制
func LockAmountWithIncrement(address string, amount int64) (int64, error) {
    lockMutex.Lock()
    defer lockMutex.Unlock()
    key := fmt.Sprintf("%s:%d", address, amount)
    if locked, exists := addressAmountMap[key]; exists {
        if time.Now().Before(locked.ExpiresAt) {
            // 被占用，尝试递增 0.0001 USDT (即 100 最小单位，假设精度为 6)
            inc := int64(100)
            for i := 1; i <= 100; i++ {
                newAmount := amount + inc*int64(i)
                newKey := fmt.Sprintf("%s:%d", address, newAmount)
                if _, exists := addressAmountMap[newKey]; !exists {
                    addressAmountMap[newKey] = &LockedAmount{
                        Amount:    newAmount,
                        OrderID:   "",
                        ExpiresAt: time.Now().Add(10 * time.Minute),
                    }
                    return newAmount, nil
                }
            }
            return 0, fmt.Errorf("无法分配可用金额")
        }
    }
    addressAmountMap[key] = &LockedAmount{
        Amount:    amount,
        OrderID:   "",
        ExpiresAt: time.Now().Add(10 * time.Minute),
    }
    return amount, nil
}

// CalculateCryptoAmount 法币转加密货币金额（含加价）
func CalculateCryptoAmount(currency, token string, amount float64, pricing PricingConfig) (int64, error) {
    // 1. 获取汇率（这里需对接实时 API，目前示例返回固定值）
    rate, err := GetExchangeRate(currency, "USDT")
    if err != nil {
        return 0, err
    }
    usdtAmount := amount * rate
    // 2. 加价
    if pricing.MarkupPercent > 0 {
        usdtAmount *= (1 + pricing.MarkupPercent/100.0)
    }
    // 3. 转为最小单位（假设 USDT 6 位小数）
    minUnit := int64(usdtAmount * 1e6)
    return minUnit, nil
}

// GetExchangeRate 获取汇率（示例固定值，生产环境需替换）
func GetExchangeRate(from, to string) (float64, error) {
    rates := map[string]float64{
        "USD-USDT": 1.0,
        "CNY-USDT": 0.138,
        "EUR-USDT": 1.087,
    }
    key := from + "-" + to
    if rate, ok := rates[key]; ok {
        return rate, nil
    }
    return 0, fmt.Errorf("不支持的汇率")
}

// SelectWalletFromList 通用轮选地址
func SelectWalletFromList(wallets []string, orderID string) string {
    h := 0
    for _, c := range orderID {
        h = h*31 + int(c)
    }
    if h < 0 {
        h = -h
    }
    return wallets[h%len(wallets)]
}

// handlePaymentSuccess 支付成功后的统一处理
func handlePaymentSuccess(chain, txID, address string, amount int64) {
    order, err := GetPendingOrderByAddressAndAmount(chain, address, amount)
    if err != nil {
        return
    }
    MarkOrderPaid(order.TradeID)
    // Telegram 通知
    if config.Telegram.Enabled {
        notifyTelegramPayment(order)
    }
}
