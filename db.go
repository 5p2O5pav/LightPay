package main

import (
    "database/sql"
    "time"
    _ "github.com/mattn/go-sqlite3"
)

var db *sql.DB

func InitDB(cfg *Config) {
    var err error
    db, err = sql.Open("sqlite3", cfg.Database)
    if err != nil {
        panic(err)
    }
    // 开启 WAL 模式提升并发
    db.Exec("PRAGMA journal_mode=WAL")
    db.Exec(`CREATE TABLE IF NOT EXISTS orders (
        trade_id TEXT PRIMARY KEY,
        order_id TEXT NOT NULL,
        chain TEXT NOT NULL,
        address TEXT NOT NULL,
        amount INTEGER NOT NULL,       -- 最小单位，如 USDT 的 1e6
        currency TEXT NOT NULL,
        token TEXT NOT NULL,
        notify_url TEXT,
        redirect_url TEXT,
        status TEXT NOT NULL DEFAULT 'pending',
        expired_at DATETIME NOT NULL,
        created_at DATETIME NOT NULL
    )`)
}

// SaveOrder 保存订单，amount 传入最小单位整数
func SaveOrder(order *Order) error {
    _, err := db.Exec(
        `INSERT INTO orders (trade_id, order_id, chain, address, amount, currency, token, notify_url, redirect_url, status, expired_at, created_at)
         VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
        order.TradeID, order.OrderID, order.Chain, order.Address,
        order.Amount, order.Currency, order.Token,
        order.NotifyURL, order.RedirectURL, order.Status,
        order.ExpiredAt, order.CreatedAt,
    )
    return err
}

func GetOrderByTradeID(tradeID string) (*Order, error) {
    row := db.QueryRow(`SELECT trade_id, order_id, chain, address, amount, currency, token, notify_url, redirect_url, status, expired_at, created_at FROM orders WHERE trade_id=?`, tradeID)
    var o Order
    err := row.Scan(&o.TradeID, &o.OrderID, &o.Chain, &o.Address, &o.Amount, &o.Currency, &o.Token, &o.NotifyURL, &o.RedirectURL, &o.Status, &o.ExpiredAt, &o.CreatedAt)
    if err != nil {
        return nil, err
    }
    return &o, nil
}

// hasPendingOrdersForChain 检查指定链的指定地址是否存在未支付且未过期的订单
func hasPendingOrdersForChain(chain, address string) bool {
    var count int
    db.QueryRow(`SELECT COUNT(*) FROM orders WHERE chain=? AND address=? AND status='pending' AND expired_at > datetime('now')`, chain, address).Scan(&count)
    return count > 0
}

// expireOrders 将已过期的 pending 订单标记为 expired
func expireOrders() {
    db.Exec(`UPDATE orders SET status='expired' WHERE status='pending' AND expired_at <= datetime('now')`)
}

// GetPendingOrderByAddressAndAmount 根据地址和金额（最小单位）查找 pending 订单
func GetPendingOrderByAddressAndAmount(chain, address string, amount int64) (*Order, error) {
    row := db.QueryRow(`SELECT trade_id, order_id, chain, address, amount, currency, token, notify_url, redirect_url, status, expired_at, created_at FROM orders WHERE chain=? AND address=? AND amount=? AND status='pending' AND expired_at > datetime('now')`, chain, address, amount)
    var o Order
    err := row.Scan(&o.TradeID, &o.OrderID, &o.Chain, &o.Address, &o.Amount, &o.Currency, &o.Token, &o.NotifyURL, &o.RedirectURL, &o.Status, &o.ExpiredAt, &o.CreatedAt)
    if err != nil {
        return nil, err
    }
    return &o, nil
}

// MarkOrderPaid 标记订单为已支付
func MarkOrderPaid(tradeID string) error {
    _, err := db.Exec(`UPDATE orders SET status='paid' WHERE trade_id=?`, tradeID)
    return err
}

// GetDayStats 获取某日的各链总收入（最小单位）
func GetDayStats(day string) map[string]int64 {
    rows, err := db.Query(`SELECT chain, SUM(amount) FROM orders WHERE status='paid' AND date(created_at)=? GROUP BY chain`, day)
    if err != nil {
        return nil
    }
    defer rows.Close()
    stats := map[string]int64{}
    for rows.Next() {
        var chain string
        var total int64
        rows.Scan(&chain, &total)
        stats[chain] = total
    }
    return stats
}

// 清理过期订单的定时任务
func startExpireCleaner() {
    go func() {
        ticker := time.NewTicker(30 * time.Second)
        for range ticker.C {
            expireOrders()
        }
    }()
}
