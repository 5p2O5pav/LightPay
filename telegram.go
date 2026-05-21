package main

import (
	"context"
	"fmt"
	"log"
	"time"
	
	"github.com/go-telegram/bot"
	"github.com/robfig/cron/v3"
)

func StartTelegramBot(config *Config) {
	ctx := context.Background()
	
	b, err := bot.New(config.Telegram.BotToken)
	if err != nil {
		log.Printf("Telegram机器人启动失败: %v", err)
		return
	}
	
	// 注册命令
	b.RegisterHandler(bot.HandlerFunc(func(ctx context.Context, b *bot.Bot, update *bot.Update) {
		if update.Message.Text == "/today" {
			summary := getTodaySummary()
			b.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: config.Telegram.ChatID,
				Text:   summary,
			})
		}
	}))
	
	// 定时每日总结
	c := cron.New()
	c.AddFunc("0 20 * * *", func() { // 每天20:00
		summary := getTodaySummary()
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: config.Telegram.ChatID,
			Text:   summary,
		})
	})
	c.Start()
	
	go b.Start(ctx)
}

func getTodaySummary() string {
	today := time.Now().Format("2006-01-02")
	
	// 从数据库查询今日收入
	stats := GetDayStats(today)
	
	msg := fmt.Sprintf("📊 %s 收入统计\n", today)
	msg += "══════════════════════\n"
	
	totalCNY := 0.0
	for chain, amount := range stats {
		cny := amount * 7.25 // 假设汇率
		totalCNY += cny
		msg += fmt.Sprintf("%s: %.2f (≈¥%.2f)\n", chain, amount, cny)
	}
	
	msg += "══════════════════════\n"
	msg += fmt.Sprintf("总计: ¥%.2f", totalCNY)
	
	return msg
}
