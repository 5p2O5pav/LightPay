package main

import (
    "bufio"
    "fmt"
    "os"
    "strings"
)

func StartSSHManagement(config *Config) {
    scanner := bufio.NewScanner(os.Stdin)
    fmt.Println("LightPay 管理面板已启动，输入 'pay' 进入管理界面")
    for scanner.Scan() {
        input := strings.TrimSpace(scanner.Text())
        if input == "pay" {
            showManagementPanel()
        }
    }
}

func showManagementPanel() {
    for {
        fmt.Println("\n╔════════════════════════════════╗")
        fmt.Println("║     LightPay 管理面板         ║")
        fmt.Println("╠════════════════════════════════╣")
        fmt.Println("║ 1. 查看今日收入               ║")
        fmt.Println("║ 2. 查看交易记录               ║")
        fmt.Println("║ 3. 钱包管理                   ║")
        fmt.Println("║ 4. 系统设置                   ║")
        fmt.Println("║ 5. 重启服务                   ║")
        fmt.Println("║ 0. 退出                       ║")
        fmt.Println("╚════════════════════════════════╝")
        fmt.Print("请选择: ")
        var choice int
        fmt.Scanln(&choice)
        switch choice {
        case 1:
            showTodayIncome()
        case 2:
            fmt.Println("交易记录功能开发中...")
        case 3:
            fmt.Println("钱包管理功能开发中...")
        case 4:
            fmt.Println("系统设置功能开发中...")
        case 5:
            fmt.Println("重启功能开发中...")
        case 0:
            return
        }
    }
}

func showTodayIncome() {
    stats := GetDayStats("2024-01-01") // 应传入当天日期
    fmt.Println("今日收入统计（示例日期）:")
    for chain, amount := range stats {
        fmt.Printf("%s: %.2f USDT\n", chain, float64(amount)/1e6)
    }
}
