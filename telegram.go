package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "io"
    "log"
    "net/http"
    "strconv"
    "time"

    "github.com/robfig/cron/v3"
)

const telegramAPI = "https://api.telegram.org/bot"

// 发送消息到 Telegram
func sendTelegramMessage(chatID int64, text string) error {
    url := telegramAPI + config.Telegram.BotToken + "/sendMessage"
    body := map[string]interface{}{
        "chat_id": chatID,
        "text":    text,
    }
    jsonBody, _ := json.Marshal(body)
    resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonBody))
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    return nil
}

// 获取机器人收到的更新（用于简单命令监听）
func getUpdates(offset int) ([]map[string]interface{}, error) {
    url := telegramAPI + config.Telegram.BotToken + "/getUpdates"
    resp, err := http.Get(url + "?offset=" + strconv.Itoa(offset))
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    body, _ := io.ReadAll(resp.Body)
    var result struct {
        OK     bool `json:"ok"`
        Result []map[string]interface{} `json:"result"`
    }
    json.Unmarshal(body, &result)
    if !result.OK {
        return nil, fmt.Errorf("Telegram API error")
    }
    return result.Result, nil
}

// 轮询命令
func pollTelegramCommands() {
    offset := 0
    for {
        updates, err := getUpdates(offset)
        if err != nil {
            time.Sleep(5 * time.Second)
            continue
        }
        for _, upd := range updates {
            if updateID, ok := upd["update_id"].(float64); ok {
                offset = int(updateID) + 1
            }
            if message, ok := upd["message"].(map[string]interface{}); ok {
                if text, ok := message["text"].(string); ok {
                    if text == "/today" {
                        summary := getTodaySummary()
                        chat := message["chat"].(map[string]interface{})
                        chatID := int64(chat["id"].(float64))
                        sendTelegramMessage(chatID, summary)
                    }
                }
            }
        }
        time.Sleep(2 * time.Second)
    }
}

// 启动 Telegram 机器人
func StartTelegramBot(cfg *Config) {
    if cfg.Telegram.BotToken == "" || cfg.Telegram.ChatID == 0 {
        log.Println("Telegram 配置不完整，跳过启动")
        return
    }

    // 启动命令轮询
    go pollTelegramCommands()

    // 每日总结定时任务 (北京时间 20:00)
    c := cron.New()
    c.AddFunc("0 20 * * *", func() {
        summary := getTodaySummary()
        if err := sendTelegramMessage(cfg.Telegram.ChatID, summary); err != nil {
            log.Println("发送每日总结失败:", err)
        }
    })
    c.Start()

    log.Println("Telegram 机器人已启动")
}

// 支付成功时的实时通知
func notifyTelegramPayment(order *Order) {
    if config.Telegram.BotToken == "" || config.Telegram.ChatID == 0 {
        return
    }
    amount := float64(order.Amount) / 1e6
    msg := fmt.Sprintf("✅ 收到新付款\n\n订单号: %s\n链: %s\n金额: %.6f %s",
        order.OrderID, order.Chain, amount, order.Token)
    sendTelegramMessage(config.Telegram.ChatID, msg)
}

func getTodaySummary() string {
    today := time.Now().Format("2006-01-02")
    stats := GetDayStats(today)
    msg := fmt.Sprintf("📊 %s 收入统计\n", today)
    msg += "══════════════════════\n"
    totalCNY := 0.0
    for chain, amount := range stats {
        cny := float64(amount) / 1e6 * 7.25 // 示例汇率，可配置
        totalCNY += cny
        msg += fmt.Sprintf("%s: %.2f USDT (≈¥%.2f)\n", chain, float64(amount)/1e6, cny)
    }
    msg += "══════════════════════\n"
    msg += fmt.Sprintf("总计: ¥%.2f", totalCNY)
    return msg
}
