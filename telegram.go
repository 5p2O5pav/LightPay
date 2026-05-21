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

    ctx := context.Background()
    // 监听 /today 命令
    tgBot.OnText("/today", func(ctx context.Context, msg *bot.Message) {
        summary := getTodaySummary()
        tgBot.SendMessage(ctx, &bot.SendMessageParams{
            ChatID: cfg.Telegram.ChatID,
            Text:   summary,
        })
    })

    // 每日定时总结
    c := cron.New()
    c.AddFunc("0 20 * * *", func() {
        summary := getTodaySummary()
        tgBot.SendMessage(context.Background(), &bot.SendMessageParams{
            ChatID: cfg.Telegram.ChatID,
            Text:   summary,
        })
    })
    c.Start()

    go tgBot.Start(ctx)
}

func notifyTelegramPayment(order *Order) {
    if tgBot == nil {
        return
    }
    msg := fmt.Sprintf("✅ 收到新付款\n\n订单号: %s\n链: %s\n金额: %.6f %s", order.OrderID, order.Chain, float64(order.Amount)/1e6, order.Token)
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
        // 假定所有代币为 USDT，汇率 7.25（示例）
        cny := float64(amount) / 1e6 * 7.25
        totalCNY += cny
        msg += fmt.Sprintf("%s: %.2f USDT (≈¥%.2f)\n", chain, float64(amount)/1e6, cny)
    }
    msg += "══════════════════════\n"
    msg += fmt.Sprintf("总计: ¥%.2f", totalCNY)
    return msg
}
