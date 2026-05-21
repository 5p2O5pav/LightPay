package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/go-telegram/bot"
    "github.com/robfig/cron/v3"
)

var tgBot *bot.Bot

func StartTelegramBot(cfg *Config) {
    var err error
    tgBot, err = bot.New(cfg.Telegram.BotToken)
    if err != nil {
        log.Println("Telegram 机器人初始化失败:", err)
        return
    }

    // 注册处理函数
    tgBot.RegisterHandler(bot.HandlerFunc(func(ctx context.Context, b *bot.Bot, update *bot.Update) {
        if update.Message != nil && update.Message.Text == "/today" {
            summary := getTodaySummary()
            b.SendMessage(ctx, &bot.SendMessageParams{
                ChatID: cfg.Telegram.ChatID,
                Text:   summary,
            })
        }
    }))

    // 每日定时总结（北京时间 20:00）
    c := cron.New()
    c.AddFunc("0 20 * * *", func() {
        summary := getTodaySummary()
        tgBot.SendMessage(context.Background(), &bot.SendMessageParams{
            ChatID: cfg.Telegram.ChatID,
            Text:   summary,
        })
    })
    c.Start()

    go tgBot.Start(context.Background())
}

func notifyTelegramPayment(order *Order) {
    if tgBot == nil {
        return
    }
    amount := float64(order.Amount) / 1e6 // 最小单位转正常单位
    msg := fmt.Sprintf("✅ 收到新付款\n\n订单号: %s\n链: %s\n金额: %.6f %s",
        order.OrderID, order.Chain, amount, order.Token)
    tgBot.SendMessage(context.Background(), &bot.SendMessageParams{
        ChatID: config.Telegram.ChatID,
        Text:   msg,
    })
}

func getTodaySummary() string {
    today := time.Now().Format("2006-01-02")
    stats := GetDayStats(today)
    msg := fmt.Sprintf("📊 %s 收入统计\n", today)
    msg += "══════════════════════\n"
    totalCNY := 0.0
    for chain, amount := range stats {
        cny := float64(amount) / 1e6 * 7.25 // 假设 USDT/CNY 汇率
        totalCNY += cny
        msg += fmt.Sprintf("%s: %.2f USDT (≈¥%.2f)\n", chain, float64(amount)/1e6, cny)
    }
    msg += "══════════════════════\n"
    msg += fmt.Sprintf("总计: ¥%.2f", totalCNY)
    return msg
}
