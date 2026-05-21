package main

import (
    "crypto/md5"
    "encoding/hex"
    "fmt"
    "sort"
    "strings"
    "sync"
    "time"
)

type Order struct {
    TradeID     string
    OrderID     string
    Pid         string
    Chain       string
    Address     string
    Amount      int64   // 最小单位
    FiatAmount  float64 // 法币金额
    Currency    string
    Token       string
    NotifyURL   string
    RedirectURL string
    Status      string
    ExpiredAt   time.Time
    CreatedAt   time.Time
}

var (
    addressAmountMap = make(map[string]*LockedAmount)
    lockMutex        sync.RWMutex
)

type LockedAmount struct {
    Amount    int64
    OrderID   string
    ExpiresAt time.Time
}

// 与 PHP 完全一致的签名算法
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

func LockAmountWithIncrement(address string, amount int64) (int64, error) {
    lockMutex.Lock()
    defer lockMutex.Unlock()
    key := fmt.Sprintf("%s:%d", address, amount)
    if locked, exists := addressAmountMap[key]; exists {
        if time.Now().Before(locked.ExpiresAt) {
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

func CalculateCryptoAmount(currency, token string, amount float64, pricing PricingConfig) (int64, error) {
    // 将 token 转为大写，因为汇率 map 里的 key 是大写 "USDT"
    rate, err := GetExchangeRate(currency, strings.ToUpper(token))
    if err != nil {
        return 0, err
    }
    usdtAmount := amount * rate
    if pricing.MarkupPercent > 0 {
        usdtAmount *= (1 + pricing.MarkupPercent/100.0)
    }
    return int64(usdtAmount * 1e6), nil
}

func GetExchangeRate(from, to string) (float64, error) {
    rates := map[string]float64{
        "USD-USDT": 1.0,
        "CNY-USDT": 0.138,
        "EUR-USDT": 1.087,
    }
    // 统一转为大写，避免大小写问题
    key := strings.ToUpper(from) + "-" + strings.ToUpper(to)
    if rate, ok := rates[key]; ok {
        return rate, nil
    }
    return 0, fmt.Errorf("不支持的汇率: %s", key)
}

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

// 支付成功后：更新状态 + 发送异步通知
func handlePaymentSuccess(chain, txID, address string, amount int64) {
    order, err := GetPendingOrderByAddressAndAmount(chain, address, amount)
    if err != nil {
        return
    }
    if err := MarkOrderPaid(order.TradeID); err != nil {
        return
    }
    // 发送商户回调
    go sendOrderNotify(order)

    if config.Telegram.Enabled {
        notifyTelegramPayment(order)
    }
}
