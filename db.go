package main

import (
    "database/sql"
    "time"

    _ "modernc.org/sqlite"
)

var db *sql.DB

func InitDB(cfg *Config) {
    var err error
    db, err = sql.Open("sqlite", cfg.Database)
    if err != nil {
        panic(err)
    }
    db.Exec("PRAGMA journal_mode=WAL")
    // 注意增加 token 列
    db.Exec(`CREATE TABLE IF NOT EXISTS orders (
        trade_id TEXT PRIMARY KEY,
        order_id TEXT NOT NULL,
        pid TEXT,
        chain TEXT NOT NULL,
        token TEXT NOT NULL DEFAULT '',
        address TEXT NOT NULL,
        amount INTEGER NOT NULL,
        fiat_amount REAL NOT NULL,
        currency TEXT NOT NULL,
        notify_url TEXT,
        redirect_url TEXT,
        status TEXT NOT NULL DEFAULT 'pending',
        expired_at DATETIME NOT NULL,
        created_at DATETIME NOT NULL
    )`)
    // 如果旧表没有 token 列，迁移
    db.Exec(`ALTER TABLE orders ADD COLUMN token TEXT DEFAULT ''`)
}

func SaveOrder(order *Order) error {
    _, err := db.Exec(
        `INSERT INTO orders (trade_id, order_id, pid, chain, token, address, amount, fiat_amount, currency, notify_url, redirect_url, status, expired_at, created_at)
         VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
        order.TradeID, order.OrderID, order.Pid, order.Chain, order.Token,
        order.Address, order.Amount, order.FiatAmount, order.Currency,
        order.NotifyURL, order.RedirectURL, order.Status,
        order.ExpiredAt, order.CreatedAt,
    )
    return err
}

func GetOrderByTradeID(tradeID string) (*Order, error) {
    row := db.QueryRow(`SELECT trade_id, order_id, pid, chain, token, address, amount, fiat_amount, currency, notify_url, redirect_url, status, expired_at, created_at FROM orders WHERE trade_id=?`, tradeID)
    var o Order
    err := row.Scan(&o.TradeID, &o.OrderID, &o.Pid, &o.Chain, &o.Token, &o.Address, &o.Amount, &o.FiatAmount, &o.Currency, &o.NotifyURL, &o.RedirectURL, &o.Status, &o.ExpiredAt, &o.CreatedAt)
    return &o, err
}

func hasPendingOrdersForChainToken(chain, address, token string) bool {
    var count int
    db.QueryRow(`SELECT COUNT(*) FROM orders WHERE chain=? AND address=? AND token=? AND status='pending' AND expired_at > datetime('now')`, chain, address, token).Scan(&count)
    return count > 0
}

func GetPendingOrderByAddressAmountToken(chain, address string, amount int64, token string) (*Order, error) {
    row := db.QueryRow(`SELECT trade_id, order_id, pid, chain, token, address, amount, fiat_amount, currency, notify_url, redirect_url, status, expired_at, created_at 
        FROM orders WHERE chain=? AND address=? AND amount=? AND token=? AND status='pending' AND expired_at > datetime('now')`, chain, address, amount, token)
    var o Order
    err := row.Scan(&o.TradeID, &o.OrderID, &o.Pid, &o.Chain, &o.Token, &o.Address, &o.Amount, &o.FiatAmount, &o.Currency, &o.NotifyURL, &o.RedirectURL, &o.Status, &o.ExpiredAt, &o.CreatedAt)
    return &o, err
}

func MarkOrderPaid(tradeID string) error {
    _, err := db.Exec(`UPDATE orders SET status='paid' WHERE trade_id=?`, tradeID)
    return err
}

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

func startExpireCleaner() {
    go func() {
        ticker := time.NewTicker(30 * time.Second)
        for range ticker.C {
            db.Exec(`UPDATE orders SET status='expired' WHERE status='pending' AND expired_at <= datetime('now')`)
            // 清理过期的金额锁（在 payment.go 中，锁有时间，但也可以主动清理 map，不过 map 已带过期检查）
        }
    }()
}
