package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func StartSSHManagement(config *Config) {
	// 创建别名
	createAlias()
	
	// 监听命令行输入(在实际部署中，这会通过SSH连接触发)
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("LightPay 管理面板已启动，输入 'pay' 进入管理界面")
	
	for scanner.Scan() {
		input := strings.TrimSpace(scanner.Text())
		if input == "pay" {
			showManagementPanel()
		}
	}
}

func createAlias() {
	// 在~/.bashrc中添加别名
	home, _ := os.UserHomeDir()
	bashrc := home + "/.bashrc"
	
	alias := "\nalias pay='lightpay manage'\n"
	
	// 检查是否已存在
	content, _ := os.ReadFile(bashrc)
	if !strings.Contains(string(content), "alias pay=") {
		f, _ := os.OpenFile(bashrc, os.O_APPEND|os.O_WRONLY, 0644)
		f.WriteString(alias)
		f.Close()
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
			showTransactions()
		case 3:
			manageWallets()
		case 4:
			editSettings()
		case 5:
			restartService()
		case 0:
			return
		}
	}
}

func showTodayIncome() {
	// 从数据库查询今日收入
	fmt.Println("\n📊 今日收入统计:")
	fmt.Println("════════════════════════════════")
	fmt.Println("TRON-USDT: 1,250.00 (¥9,062.50)")
	fmt.Println("════════════════════════════════")
	fmt.Println("总计: ¥9,062.50")
}
