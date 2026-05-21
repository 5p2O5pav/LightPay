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
    db.Exec("PRAGMA journal_mode=WAL")
    db.Exec(`CREATE TABLE IF NOT EXISTS orders (
        trade_id TEXT PRIMARY KEY,
        order_id TEXT NOT NULL,
        pid TEXT,
        chain TEXT NOT NULL,
        address TEXT NOT NULL,
        amount INTEGER NOT NULL,
        fiat_amount REAL NOT NULL,
        currency TEXT NOT NULL,
        token TEXT NOT NULL,
        notify_url TEXT,
        redirect_url TEXT,
        status TEXT NOT NULL DEFAULT 'pending',
        expired_at DATETIME NOT NULL,
        created_at DATETIME NOT NULL
    )`)
    // 兼容旧表（如果没有 pid 或 fiat_amount 列则添加，新表直接创建）
    if _, err := db.Exec("ALTER TABLE orders ADD COLUMN pid TEXT"); err != nil {
        // 列已存在则忽略
    }
    if _, err := db.Exec("ALTER TABLE orders ADD COLUMN fiat_amount REAL"); err != nil {
        // 列已存在则忽略
    }
}

func SaveOrder(order *Order) error {
    _, err := db.Exec(
        `INSERT INTO orders (trade_id, order_id, pid, chain, address, amount, fiat_amount, currency, token, notify_url, redirect_url, status, expired_at, created_at)
         VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
        order.TradeID, order.OrderID, order.Pid, order.Chain, order.Address,
        order.Amount, order.FiatAmount, order.Currency, order.Token,
        order.NotifyURL, order.RedirectURL, order.Status,
        order.ExpiredAt, order.CreatedAt,
    )
    return err
}

func GetOrderByTradeID(tradeID string) (*Order, error) {
    row := db.QueryRow(`SELECT trade_id, order_id, pid, chain, address, amount, fiat_amount, currency, token, notify_url, redirect_url, status, expired_at, created_at FROM orders WHERE trade_id=?`, tradeID)
    var o Order
    err := row.Scan(&o.TradeID, &o.OrderID, &o.Pid, &o.Chain, &o.Address, &o.Amount, &o.FiatAmount, &o.Currency, &o.Token, &o.NotifyURL, &o.RedirectURL, &o.Status, &o.ExpiredAt, &o.CreatedAt)
    if err != nil {
        return nil, err
    }
    return &o, nil
}

func hasPendingOrdersForChain(chain, address string) bool {
    var count int
    db.QueryRow(`SELECT COUNT(*) FROM orders WHERE chain=? AND address=? AND status='pending' AND expired_at > datetime('now')`, chain, address).Scan(&count)
    return count > 0
}

func expireOrders() {
    db.Exec(`UPDATE orders SET status='expired' WHERE status='pending' AND expired_at <= datetime('now')`)
}

func GetPendingOrderByAddressAndAmount(chain, address string, amount int64) (*Order, error) {
    row := db.QueryRow(`SELECT trade_id, order_id, pid, chain, address, amount, fiat_amount, currency, token, notify_url, redirect_url, status, expired_at, created_at FROM orders WHERE chain=? AND address=? AND amount=? AND status='pending' AND expired_at > datetime('now')`, chain, address, amount)
    var o Order
    err := row.Scan(&o.TradeID, &o.OrderID, &o.Pid, &o.Chain, &o.Address, &o.Amount, &o.FiatAmount, &o.Currency, &o.Token, &o.NotifyURL, &o.RedirectURL, &o.Status, &o.ExpiredAt, &o.CreatedAt)
    if err != nil {
        return nil, err
    }
    return &o, nil
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
            expireOrders()
        }
    }()
}
