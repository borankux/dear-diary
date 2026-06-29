package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/borankux/dear-diary/internal/client"
)

func runRemote(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("用法: diary remote <setup|login|status|stats|todo|candidates|memories|diary|calendar|search|accept|reject|done>")
	}
	c := client.NewClient()

	switch args[0] {
	case "setup":
		if len(args) < 2 {
			return fmt.Errorf("用法: diary remote setup <url> [password]")
		}
		url := args[1]
		password := "dear-diary-2026"
		if len(args) >= 3 {
			password = args[2]
		}
		c.SetConfig(url, "")
		if err := c.Login(password); err != nil {
			return fmt.Errorf("登录失败: %w", err)
		}
		if err := c.SaveConfig(); err != nil {
			return fmt.Errorf("保存配置失败: %w", err)
		}
		fmt.Println("远程配置已保存到 ~/.config/dear-diary/remote.json")
		return nil

	case "login":
		password := "dear-diary-2026"
		if len(args) >= 2 {
			password = args[1]
		}
		if c.Config().BaseURL == "" {
			return fmt.Errorf("未配置远程服务器，请先用 diary remote setup <url>")
		}
		if err := c.Login(password); err != nil {
			return fmt.Errorf("登录失败: %w", err)
		}
		if err := c.SaveConfig(); err != nil {
			return fmt.Errorf("保存配置失败: %w", err)
		}
		fmt.Println("登录成功，Token 已更新")
		return nil

	case "status":
		if !c.IsConfigured() {
			fmt.Println("未配置远程服务器")
			return nil
		}
		fmt.Printf("服务器: %s\n", c.Config().BaseURL)
		_, err := c.GetStats()
		if err != nil {
			fmt.Printf("连接状态: 失败 (%v)\n", err)
		} else {
			fmt.Println("连接状态: 正常")
		}
		return nil

	case "stats":
		if !c.IsConfigured() {
			return fmt.Errorf("未配置远程服务器，请先用 diary remote setup <url>")
		}
		s, err := c.GetStats()
		if err != nil {
			return err
		}
		fmt.Printf("待办: %d\n", s.Todo)
		fmt.Printf("记忆: %d\n", s.Memory)
		fmt.Printf("候选: %d\n", s.Candidate)
		fmt.Printf("日记: %d\n", s.Diary)
		fmt.Printf("处理状态: %s\n", s.Processing)
		return nil

	case "todo":
		if !c.IsConfigured() {
			return fmt.Errorf("未配置远程服务器，请先用 diary remote setup <url>")
		}
		list, err := c.GetTodos()
		if err != nil {
			return err
		}
		if len(list) == 0 {
			fmt.Println("没有 todos")
			return nil
		}
		printTodoTable(list)
		return nil

	case "candidates":
		if !c.IsConfigured() {
			return fmt.Errorf("未配置远程服务器，请先用 diary remote setup <url>")
		}
		list, err := c.GetCandidates()
		if err != nil {
			return err
		}
		if len(list) == 0 {
			fmt.Println("没有候选")
			return nil
		}
		printCandidateTable(list)
		return nil

	case "memories":
		if !c.IsConfigured() {
			return fmt.Errorf("未配置远程服务器，请先用 diary remote setup <url>")
		}
		list, err := c.GetMemories()
		if err != nil {
			return err
		}
		if len(list) == 0 {
			fmt.Println("没有记忆")
			return nil
		}
		for _, m := range list {
			fmt.Printf("#%d %s\n", m.ID, m.Topic)
		}
		return nil

	case "diary":
		if !c.IsConfigured() {
			return fmt.Errorf("未配置远程服务器，请先用 diary remote setup <url>")
		}
		date := time.Now().Format("2006-01-02")
		if len(args) >= 2 {
			date = args[1]
		}
		d, err := c.GetDiary(date)
		if err != nil {
			return err
		}
		fmt.Printf("日期: %s\n", d.Date)
		fmt.Printf("段落: %d\n", d.Sections)
		fmt.Printf("修改时间: %s\n\n", d.Mtime)
		fmt.Println(d.Content)
		return nil

	case "calendar":
		if !c.IsConfigured() {
			return fmt.Errorf("未配置远程服务器，请先用 diary remote setup <url>")
		}
		list, err := c.GetCalendar()
		if err != nil {
			return err
		}
		for _, m := range list {
			fmt.Printf("%d-%02d\n", m.Year, m.Month)
			for _, d := range m.Days {
				fmt.Printf("%2d ", d)
			}
			fmt.Println()
		}
		return nil

	case "search":
		if !c.IsConfigured() {
			return fmt.Errorf("未配置远程服务器，请先用 diary remote setup <url>")
		}
		if len(args) < 2 {
			return fmt.Errorf("用法: diary remote search <query>")
		}
		q := strings.Join(args[1:], " ")
		results, err := c.Search(q)
		if err != nil {
			return err
		}
		if len(results) == 0 {
			fmt.Println("未找到结果")
			return nil
		}
		for _, r := range results {
			fmt.Printf("\n%s\n", r.Date)
			for _, line := range r.Lines {
				fmt.Printf("  %s\n", line)
			}
		}
		return nil

	case "accept":
		if !c.IsConfigured() {
			return fmt.Errorf("未配置远程服务器，请先用 diary remote setup <url>")
		}
		if len(args) < 2 {
			return fmt.Errorf("用法: diary remote accept <id>")
		}
		id, err := strconv.Atoi(args[1])
		if err != nil {
			return fmt.Errorf("无效 id: %q", args[1])
		}
		if err := c.AcceptCandidate(id); err != nil {
			return err
		}
		fmt.Printf("候选 #%d 已接受\n", id)
		return nil

	case "reject":
		if !c.IsConfigured() {
			return fmt.Errorf("未配置远程服务器，请先用 diary remote setup <url>")
		}
		if len(args) < 2 {
			return fmt.Errorf("用法: diary remote reject <id>")
		}
		id, err := strconv.Atoi(args[1])
		if err != nil {
			return fmt.Errorf("无效 id: %q", args[1])
		}
		if err := c.RejectCandidate(id); err != nil {
			return err
		}
		fmt.Printf("候选 #%d 已拒绝\n", id)
		return nil

	case "done":
		if !c.IsConfigured() {
			return fmt.Errorf("未配置远程服务器，请先用 diary remote setup <url>")
		}
		if len(args) < 2 {
			return fmt.Errorf("用法: diary remote done <id>")
		}
		id, err := strconv.Atoi(args[1])
		if err != nil {
			return fmt.Errorf("无效 id: %q", args[1])
		}
		if err := c.UpdateTodoStatus(id, "done"); err != nil {
			return err
		}
		fmt.Printf("Todo #%d 已标记完成\n", id)
		return nil

	default:
		return fmt.Errorf("未知子命令: %s", args[0])
	}
}

// ── Table helpers ────────────────────────────────────────────────────

func printTodoTable(todos []client.Todo) {
	// 表头
	fmt.Println("ID  | 创建日期       | 内容                               | 状态   | 优先级 | 标签")
	fmt.Println("----|----------------|------------------------------------|--------|--------|------")
	for _, t := range todos {
		priority := ""
		if t.Priority != nil {
			priority = fmt.Sprintf("%d", *t.Priority)
		}
		tags := strings.Join(t.Tags, ", ")
		if len(t.Tags) == 0 {
			tags = ""
		}
		fmt.Printf("%-3d | %-14s | %-34s | %-6s | %-6s | %s\n",
			t.ID, truncateDate(t.CreatedAt), truncateString(t.Text, 34), t.Status, priority, tags)
	}
}

func printCandidateTable(candidates []client.Candidate) {
	fmt.Println("ID  | 类型       | 标题                             | 标签")
	fmt.Println("----|------------|----------------------------------|------")
	for _, c := range candidates {
		tags := strings.Join(c.Tags, ", ")
		fmt.Printf("%-3d | %-10s | %-32s | %s\n", c.ID, c.Type, truncateString(c.Title, 32), tags)
	}
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-1] + "…"
}

func truncateDate(s string) string {
	// 2026-06-25T12:34:56Z -> 2026-06-25
	if idx := strings.Index(s, "T"); idx > 0 {
		return s[:idx]
	}
	return s
}
