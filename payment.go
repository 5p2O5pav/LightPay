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

const (
    // 内部精度：1 个数字货币单位 = 1000 (即小数点后3位)
    InternalPrecision = 1000
)

type Order struct {
    TradeID     string
    OrderID     string
    Pid         string
    Chain       string
    Token       string   // 新增：区分同链不同币种
    Address     string
    Amount      int64    // 内部精度（3位小数）
    FiatAmount  float64
    Currency    string
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

// MakeSignature 用于 map[string]string（回调用）
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

// MakeSignatureFromMap 兼容 PHP Epusdt
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
        case int, int32, int64:
            valueStr = formatAmountPHP(float64(val.(int64)))
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

func formatAmountPHP(v float64) string {
    s := strconv.FormatFloat(v, 'f', 12, 64)
    s = strings.TrimRight(s, "0")
    s = strings.TrimRight(s, ".")
    if s == "" {
        s = "0"
    }
    return s
}

// SelectWalletFromList 根据订单ID哈希选择一个钱包地址
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

// LockAmountWithIncrement 加价算法：同一地址+同一金额，尚未完成/过期的订单，新订单递增 0.001（内部单位 1），最多 20 次
func LockAmountWithIncrement(address string, baseAmount int64) (int64, error) {
    lockMutex.Lock()
    defer lockMutex.Unlock()

    // 检查当前地址下所有 pending 且未过期的订单金额（从数据库读取更精确，但为了性能可先查map+db结合）
    // 简化：直接从 addressAmountMap 中找出所有以该地址+任意金额开头的锁，并检查数据库实际pending订单
    // 这里我们直接按递增尝试，最多尝试 20 次
    for inc := 0; inc <= 20; inc++ {
        candidate := baseAmount + int64(inc)
        key := fmt.Sprintf("%s:%d", address, candidate)
        locked, exists := addressAmountMap[key]

        // 如果不存在锁，或者锁已过期，则可以占用
        if !exists || time.Now().After(locked.ExpiresAt) {
            // 还需要检查数据库中是否真的没有 pending 订单使用这个金额（防止程序重启后map丢失）
            hasPending, err := hasPendingOrderForAddressAmount(address, candidate)
            if err != nil {
                return 0, err
            }
            if !hasPending {
                addressAmountMap[key] = &LockedAmount{
                    Amount:    candidate,
                    ExpiresAt: time.Now().Add(10 * time.Minute),
                }
                return candidate, nil
            }
        }
        // 如果 inc == 20 仍未找到可用金额，返回错误
        if inc == 20 {
            return 0, fmt.Errorf("busy: too many pending orders with similar amount")
        }
    }
    return 0, fmt.Errorf("busy: cannot assign unique amount")
}

// hasPendingOrderForAddressAmount 检查数据库中是否存在 pending 或 paid 但未过期的订单使用了该金额
func hasPendingOrderForAddressAmount(address string, amount int64) (bool, error) {
    var count int
    err := db.QueryRow(`SELECT COUNT(*) FROM orders WHERE address=? AND amount=? AND status IN ('pending','paid') AND expired_at > datetime('now')`,
        address, amount).Scan(&count)
    return count > 0, err
}

// ReleaseLockedAmount 订单完成后释放锁
func ReleaseLockedAmount(address string, amount int64) {
    lockMutex.Lock()
    defer lockMutex.Unlock()
    key := fmt.Sprintf("%s:%d", address, amount)
    delete(addressAmountMap, key)
}

// CalculateCryptoAmount 计算数字货币金额（内部精度，3位小数）
func CalculateCryptoAmount(currency, token string, fiatAmount float64) (int64, error) {
    // 获取 1 token = X currency
    rateTokenToCurrency, err := GetExchangeRate(currency, token)
    if err != nil {
        return 0, err
    }
    // 需要 1 currency = ? token
    rateCurrencyToToken := 1.0 / rateTokenToCurrency
    tokenAmount := fiatAmount * rateCurrencyToToken
    if config.Pricing.MarkupPercent > 0 {
        tokenAmount *= (1 + config.Pricing.MarkupPercent/100.0)
    }
    // 转为内部精度（3位小数）
    amountInt := int64(tokenAmount * InternalPrecision)
    if amountInt <= 0 {
        return 0, fmt.Errorf("calculated amount is zero")
    }
    return amountInt, nil
}

// 支付成功处理
func handlePaymentSuccess(chain, token, txID, address string, amount int64) {
    log.Printf("[INFO] 支付成功: chain=%s token=%s tx=%s addr=%s amount=%d", chain, token, txID, address, amount)
    order, err := GetPendingOrderByAddressAmountToken(chain, address, amount, token)
    if err != nil {
        log.Printf("[ERROR] 未找到匹配订单: %v", err)
        return
    }
    if err := MarkOrderPaid(order.TradeID); err != nil {
        log.Printf("[ERROR] 更新订单状态失败: %v", err)
        return
    }
    ReleaseLockedAmount(address, amount)
    go sendOrderNotify(order)
    if config.Telegram.Enabled {
        notifyTelegramPayment(order)
    }
}
