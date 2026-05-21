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

// 模仿Epusdt的金额匹配逻辑
var (
	addressAmountMap = make(map[string]*LockedAmount)
	mutex            sync.RWMutex
)

type LockedAmount struct {
	Amount    float64
	OrderID   string
	ExpiresAt time.Time
}

// 创建订单
func CreateOrder(address string, expectedAmount float64, orderID string) error {
	mutex.Lock()
	defer mutex.Unlock()
	
	// 检查地址+金额组合是否已被占用
	key := fmt.Sprintf("%s:%.2f", address, expectedAmount)
	if locked, exists := addressAmountMap[key]; exists {
		if time.Now().Before(locked.ExpiresAt) {
			// 已被占用，尝试累加0.0001
			for i := 1; i <= 100; i++ {
				newAmount := expectedAmount + float64(i)*0.0001
				newKey := fmt.Sprintf("%s:%.2f", address, newAmount)
				if _, exists := addressAmountMap[newKey]; !exists {
					addressAmountMap[newKey] = &LockedAmount{
						Amount:    newAmount,
						OrderID:   orderID,
						ExpiresAt: time.Now().Add(10 * time.Minute),
					}
					return nil
				}
			}
			return fmt.Errorf("无法分配可用金额")
		}
	}
	
	// 锁定该组合10分钟
	addressAmountMap[key] = &LockedAmount{
		Amount:    expectedAmount,
		OrderID:   orderID,
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}
	return nil
}

// Epusdt签名算法实现
func MakeSignature(params map[string]string, token string) string {
	// 按key排序
	keys := make([]string, 0, len(params))
	for k := range params {
		if k != "signature" && params[k] != "" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	
	// 拼接字符串
	var pairs []string
	for _, k := range keys {
		pairs = append(pairs, fmt.Sprintf("%s=%s", k, params[k]))
	}
	
	// MD5加密
	signStr := strings.Join(pairs, "&") + token
	hash := md5.Sum([]byte(signStr))
	return strings.ToLower(hex.EncodeToString(hash[:]))
}

// 查询汇率(需对接汇率API)
func GetExchangeRate(from, to string) (float64, error) {
	// 这里需要对接实际的汇率API
	// 示例返回固定汇率
	rates := map[string]float64{
		"USD-CNY": 7.25,
		"USD-EUR": 0.92,
		"CNY-USD": 0.138,
		"EUR-USD": 1.087,
	}
	
	key := fmt.Sprintf("%s-%s", from, to)
	if rate, ok := rates[key]; ok {
		return rate, nil
	}
	return 0, fmt.Errorf("不支持的汇率转换")
}
