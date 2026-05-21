package main

import (
    "database/sql"
    "time"

    _ "modernc.org/sqlite"
)

var db *sql.DB

// InitDB 初始化数据库连接并创建表
func InitDB(cfg *Config) {
    var err error
    db, err = sql.Open("sqlite", cfg.Database)
    if err != nil {
        panic(err)
    }
    // 启用 WAL 模式提高并发性能
    db.Exec("PRAGMA journal_mode=WAL")
    // 创建订单表，增加 token 列
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
    // 兼容旧表：如果没有 token 列则添加
    db.Exec(`ALTER TABLE orders ADD COLUMN token TEXT DEFAULT ''`)
}

// SaveOrder 保存订单
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

// GetOrderByTradeID 根据交易ID查询订单
func GetOrderByTradeID(tradeID string) (*Order, error) {
    row := db.QueryRow(`SELECT trade_id, order_id, pid, chain, token, address, amount, fiat_amount, currency, notify_url, redirect_url, status, expired_at, created_at FROM orders WHERE trade_id=?`, tradeID)
    var o Order
    err := row.Scan(&o.TradeID, &o.OrderID, &o.Pid, &o.Chain, &o.Token, &o.Address, &o.Amount, &o.FiatAmount, &o.Currency, &o.NotifyURL, &o.RedirectURL, &o.Status, &o.ExpiredAt, &o.CreatedAt)
    if err != nil {
        return nil, err
    }
    return &o, nil
}

// hasPendingOrdersForChainToken 检查指定地址和代币是否存在未过期的待支付订单
func hasPendingOrdersForChainToken(chain, address, token string) bool {
    var count int
    db.QueryRow(`SELECT COUNT(*) FROM orders WHERE chain=? AND address=? AND token=? AND status='pending' AND expired_at > datetime('now')`, chain, address, token).Scan(&count)
    return count > 0
}

// GetPendingOrderByAddressAmountToken 精确匹配地址、金额、代币的待支付订单
func GetPendingOrderByAddressAmountToken(chain, address string, amount int64, token string) (*Order, error) {
    row := db.QueryRow(`SELECT trade_id, order_id, pid, chain, token, address, amount, fiat_amount, currency, notify_url, redirect_url, status, expired_at, created_at 
        FROM orders WHERE chain=? AND address=? AND amount=? AND token=? AND status='pending' AND expired_at > datetime('now')`, chain, address, amount, token)
    var o Order
    err := row.Scan(&o.TradeID, &o.OrderID, &o.Pid, &o.Chain, &o.Token, &o.Address, &o.Amount, &o.FiatAmount, &o.Currency, &o.NotifyURL, &o.RedirectURL, &o.Status, &o.ExpiredAt, &o.CreatedAt)
    if err != nil {
        return nil, err
    }
    return &o, nil
}

// MarkOrderPaid 将订单状态标记为已支付
func MarkOrderPaid(tradeID string) error {
    _, err := db.Exec(`UPDATE orders SET status='paid' WHERE trade_id=?`, tradeID)
    return err
}

// GetDayStats 按链统计某天的总金额（内部单位）
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

// expireOrders 将超时的待支付订单标记为过期
func expireOrders() {
    db.Exec(`UPDATE orders SET status='expired' WHERE status='pending' AND expired_at <= datetime('now')`)
}

// startExpireCleaner 启动定时清理过期订单的协程
func startExpireCleaner() {
    go func() {
        ticker := time.NewTicker(30 * time.Second)
        for range ticker.C {
            expireOrders()
        }
    }()
}
