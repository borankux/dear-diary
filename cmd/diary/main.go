package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/borankux/dear-diary/internal/editor"
	"github.com/borankux/dear-diary/internal/search"
	"github.com/borankux/dear-diary/internal/storage"
	"github.com/borankux/dear-diary/internal/tui"
)

const version = "0.1.0"

const usage = `亲爱的日记 — 一个 TUI 日记应用

用法:
  diary                  打开今天的日记 (默认行为)
  diary browse           进入 TUI 月历浏览
  diary today            显式打开今天 (= diary)
  diary yesterday        打开昨天 (别名: diary y)
  diary <date>           打开指定日期，格式支持:
                            2026-06-24    ISO 完整
                            06-24         月-日 (默认今年)
                            6/24          月/日 (默认今年)
  diary search <keyword> 搜索所有日记内容 (匹配的行倒序列出)
  diary -h | --help      显示本帮助
  diary -v | --version   显示版本号

存储:
  ~/Documents/dear-diary/YYYY-MM/YYYY-MM-DD.md
  例: ~/Documents/dear-diary/2026-06/2026-06-25.md

编辑器:
  优先级 $DIARY_EDITOR > $EDITOR > vim
`

func main() {
	args := os.Args[1:]

	if len(args) == 0 {
		mustOpen(time.Now(), false)
		return
	}

	switch args[0] {
	case "-h", "--help":
		fmt.Print(usage)
		return
	case "-v", "--version":
		fmt.Println(version)
		return
	case "browse":
		must(runBrowse())
		return
	case "y", "yesterday":
		mustOpen(time.Now().AddDate(0, 0, -1), true)
		return
	case "today":
		mustOpen(time.Now(), false)
		return
	case "search":
		if len(args) < 2 {
			die(2, "缺少搜索关键词", "用法: diary search <关键词>")
		}
		must(runSearch(strings.Join(args[1:], " ")))
		return
	}

	// 尝试解析为日期
	d, err := parseDate(args[0])
	if err != nil {
		die(2, "无法识别的命令或日期: "+args[0], "")
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	mustOpen(d, true)
}

func runBrowse() error {
	s := storage.New()
	m := tui.NewBrowseModel(s)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func runSearch(keyword string) error {
	s := storage.New()
	results, err := search.Search(s.RootDir(), keyword)
	if err != nil {
		return err
	}
	if len(results) == 0 {
		fmt.Printf("没有找到匹配 %q 的日记\n", keyword)
		return nil
	}
	m := tui.NewSearchModel(s, results, keyword)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err = p.Run()
	return err
}

// mustOpen 打开指定日期的 Vim。
// honorAppendMode: false 表示"今天用默认行为 (智能追加)"
//
//	true 表示"明确非今天 (历史回看)，不要追加时间戳段落"
func mustOpen(t time.Time, historical bool) {
	s := storage.New()
	path := s.PathFor(t)

	now := time.Now()
	isToday := sameDay(t, now)
	isExist := s.Exists(t)

	if !isExist {
		if _, err := s.EnsureFile(path, t); err != nil {
			die(1, "创建日记文件失败: "+err.Error(), "")
		}
	} else if isToday && !historical {
		// 同一天再次打开今天，追加新的时间戳段落
		if !endsWithRecentTimestamp(path, now) {
			if err := s.AppendTimestamp(path, now); err != nil {
				die(1, "追加时间戳失败: "+err.Error(), "")
			}
		}
	}

	// appendMode=true 让 Vim 跳到末尾追加位置
	appendMode := isExist
	if err := editor.Open(path, appendMode); err != nil {
		die(1, "编辑器异常退出: "+err.Error(), "")
	}
}

func parseDate(s string) (time.Time, error) {
	now := time.Now()

	if d, err := time.ParseInLocation("2006-01-02", s, time.Local); err == nil {
		return d, nil
	}
	if d, err := time.ParseInLocation("01-02", s, time.Local); err == nil {
		return time.Date(now.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.Local), nil
	}
	if d, err := time.ParseInLocation("1-2", s, time.Local); err == nil {
		return time.Date(now.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.Local), nil
	}
	parts := strings.Split(s, "/")
	if len(parts) == 2 {
		m, err1 := strconv.Atoi(parts[0])
		d, err2 := strconv.Atoi(parts[1])
		if err1 == nil && err2 == nil && m >= 1 && m <= 12 && d >= 1 && d <= 31 {
			return time.Date(now.Year(), time.Month(m), d, 0, 0, 0, 0, time.Local), nil
		}
	}
	return time.Time{}, errors.New("unsupported date format")
}

func sameDay(a, b time.Time) bool {
	return a.Year() == b.Year() && a.Month() == b.Month() && a.Day() == b.Day()
}

// endsWithRecentTimestamp 检查文件最后非空行是否是 5 分钟内的时间戳段落。
// 防止用户连续多次打开同一天导致一堆空白时间段。
func endsWithRecentTimestamp(path string, now time.Time) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	// 找最后一个非空行
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		// 形如 "## 21:30"
		if !strings.HasPrefix(line, "## ") {
			return false
		}
		ts := strings.TrimSpace(strings.TrimPrefix(line, "## "))
		t, err := time.ParseInLocation("15:04", ts, time.Local)
		if err != nil {
			return false
		}
		// 跟当前时间比较，同日才生效
		stampToday := time.Date(now.Year(), now.Month(), now.Day(),
			t.Hour(), t.Minute(), 0, 0, time.Local)
		diff := now.Sub(stampToday)
		if diff < 0 {
			diff = -diff
		}
		return diff < 5*time.Minute
	}
	return false
}

func die(code int, msg, hint string) {
	fmt.Fprintln(os.Stderr, "错误:", msg)
	if hint != "" {
		fmt.Fprintln(os.Stderr, hint)
	}
	os.Exit(code)
}

func must(err error) {
	if err != nil {
		die(1, err.Error(), "")
	}
}
