package main

import (
    "crypto/md5"
    "encoding/hex"
    "fmt"
    "log"
    "sort"
    "strconv"
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

// MakeSignature 用于 map[string]string 类型的签名（简单兼容，主要用于回调）
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

// MakeSignatureFromMap 完全兼容 PHP Epusdt 的签名算法（支持 map[string]interface{} 和浮点数特殊处理）
func MakeSignatureFromMap(params map[string]interface{}, token string) string {
    keys := make([]string, 0, len(params))
    for k, v := range params {
        if k == "signature" {
            continue
        }
        if v == nil {
            continue
        }
        if s, ok := v.(string); ok && s == "" {
            continue
        }
        keys = append(keys, k)
    }
    sort.Strings(keys)

    var pairs []string
    for _, k := range keys {
        v := params[k]
        var valueStr string
        switch val := v.(type) {
        case float64:
            valueStr = formatAmountPHP(val)
        case float32:
            valueStr = formatAmountPHP(float64(val))
        case int:
            valueStr = formatAmountPHP(float64(val))
        case int32:
            valueStr = formatAmountPHP(float64(val))
        case int64:
            valueStr = formatAmountPHP(float64(val))
        case string:
            valueStr = val
        default:
            valueStr = fmt.Sprint(val)
        }
        pairs = append(pairs, fmt.Sprintf("%s=%s", k, valueStr))
    }

    signStr := strings.Join(pairs, "&") + token
    hash := md5.Sum([]byte(signStr))
    return strings.ToLower(hex.EncodeToString(hash[:]))
}

// formatAmountPHP 模拟 PHP 的 rtrim(rtrim(sprintf('%.12F', $v), '0'), '.')
func formatAmountPHP(v float64) string {
    s := strconv.FormatFloat(v, 'f', 12, 64)
    s = strings.TrimRight(s, "0")
    s = strings.TrimRight(s, ".")
    if s == "" {
        s = "0"
    }
    return s
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
    // token 转为大写，因为汇率 map 中的 key 是大写 "USDT"
    rate, err := GetExchangeRate(currency, strings.ToUpper(token))
    if err != nil {
        log.Printf("[ERROR] 获取汇率失败: %v", err)
        return 0, err
    }
    log.Printf("[DEBUG] 汇率: 1 %s = %.6f %s", strings.ToUpper(currency), rate, strings.ToUpper(token))
    usdtAmount := amount * rate
    if pricing.MarkupPercent > 0 {
        usdtAmount *= (1 + pricing.MarkupPercent/100.0)
        log.Printf("[DEBUG] 加价后 USDT 金额: %.6f (加价比例 %.2f%%)", usdtAmount, pricing.MarkupPercent)
    }
    minUnit := int64(usdtAmount * 1e6)
    log.Printf("[DEBUG] 最小单位金额: %d", minUnit)
    return minUnit, nil
}

func GetExchangeRate(from, to string) (float64, error) {
    rates := map[string]float64{
        "USD-USDT": 1.0,
        "CNY-USDT": 0.138,
        "EUR-USDT": 1.087,
    }
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
    log.Printf("[INFO] 支付成功回调: chain=%s, txID=%s, address=%s, amount=%d", chain, txID, address, amount)
    order, err := GetPendingOrderByAddressAndAmount(chain, address, amount)
    if err != nil {
        log.Printf("[ERROR] 未找到匹配的订单: %v", err)
        return
    }
    if err := MarkOrderPaid(order.TradeID); err != nil {
        log.Printf("[ERROR] 标记订单支付失败: %v", err)
        return
    }
    log.Printf("[INFO] 订单 %s 已标记为 paid", order.TradeID)
    // 发送商户回调
    go sendOrderNotify(order)

    if config.Telegram.Enabled {
        notifyTelegramPayment(order)
    }
}
