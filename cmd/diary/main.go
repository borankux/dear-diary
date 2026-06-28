package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/borankux/dear-diary/internal/editor"
	"github.com/borankux/dear-diary/internal/memory"
	"github.com/borankux/dear-diary/internal/process"
	"github.com/borankux/dear-diary/internal/search"
	"github.com/borankux/dear-diary/internal/stats"
	"github.com/borankux/dear-diary/internal/storage"
	"github.com/borankux/dear-diary/internal/tui"
)

const version = "0.4.0"

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
  diary process          用配置的 LLM 提炼最近日记为待确认候选项
  diary review           人工确认 / 拒绝 AI 候选项
  diary todo             查看 active todos
  diary todo done <id>   标记 todo 完成
  diary todo archive <id> 归档 todo
  diary todo status <id> <status>
                         设置生命周期状态: active, in_progress, done, wont_do, archived, other
  diary todo priority <id> <0-100|clear>
                         设置或清除优先级，数字越大越靠前
  diary dashboard        在浏览器打开本地看板
  diary -h | --help      显示本帮助
  diary -v | --version   显示版本号

存储:
  ~/Documents/dear-diary/YYYY-MM/YYYY-MM-DD.md
  例: ~/Documents/dear-diary/2026-06/2026-06-25.md

隐私:
  diary / browse / search / dashboard / todo / review 只读取本地文件和 SQLite
  diary process 会把待处理日记内容发送给配置的 LLM Provider

编辑器:
  优先级 $DIARY_EDITOR > $EDITOR > vim
`

func main() {
	args := os.Args[1:]

	if len(args) == 0 {
		printDailyHighlight()
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
	case "process":
		must(runProcess())
		return
	case "review":
		must(runReview())
		return
	case "todo":
		must(runTodo(args[1:]))
		return
	case "dashboard":
		must(process.RegenerateAndOpenDashboard())
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

// printDailyHighlight 在打开今天的 Vim 前打印回顾信息：
//   - 当前 streak（连续写作天数）
//   - "X 年前的今天"的日记预览
//
// 静默处理：没数据就不打印任何东西，避免噪音。
func printDailyHighlight() {
	s := storage.New()
	now := time.Now()

	streak := stats.CurrentStreak(s, now)
	memories := memory.OnThisDay(s, now, 10)

	if streak == 0 && len(memories) == 0 {
		return
	}

	fmt.Println()
	if streak > 0 {
		fmt.Printf("  🔥 连续写作 %d 天\n", streak)
	}
	for _, m := range memories {
		weekdayCN := []string{"日", "一", "二", "三", "四", "五", "六"}
		fmt.Printf("  📅 %d 年前的今天 (%s 周%s)\n", m.YearsAgo,
			m.Date.Format("2006-01-02"), weekdayCN[int(m.Date.Weekday())])
		fmt.Printf("     %s\n", truncate(m.FirstLine, 70))
	}
	fmt.Println()
}

func truncate(s string, maxLen int) string {
	r := []rune(s)
	if len(r) <= maxLen {
		return s
	}
	return string(r[:maxLen-1]) + "…"
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

func runProcess() error {
	s := storage.New()
	runner, err := process.NewRunner(s.RootDir(), process.ProcessOutDir())
	if err != nil {
		return err
	}
	defer runner.Close()
	return runner.Run()
}

func runReview() error {
	store, err := process.NewStore("")
	if err != nil {
		return err
	}
	defer store.Close()

	counts, err := store.CandidateStatusCounts()
	if err != nil {
		return err
	}
	candidates, err := store.ListPendingCandidates()
	if err != nil {
		return err
	}
	fmt.Printf("AI candidates: %d pending · %d accepted · %d rejected\n", counts.Pending, counts.Accepted, counts.Rejected)
	if len(candidates) == 0 {
		fmt.Println("没有待确认的 AI 候选项。")
		return nil
	}

	reader := bufio.NewReader(os.Stdin)
	for i, c := range candidates {
		fmt.Printf("\n[%d/%d] #%d %s\n", i+1, len(candidates), c.ID, c.Type)
		if c.Title != "" {
			fmt.Println("Title:", c.Title)
		}
		fmt.Println("Content:", c.Content)
		if c.EvidenceText != "" {
			fmt.Println("Evidence:", c.EvidenceText)
		}
		if c.SourceDate != "" {
			fmt.Println("Source:", c.SourceDate)
		} else {
			fmt.Println("Source:", c.SourceFile)
		}
		fmt.Print("Action [a=accept, r=reject, s=skip, q=quit]: ")
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return err
		}
		action := strings.TrimSpace(strings.ToLower(line))
		switch action {
		case "a", "accept":
			if err := store.AcceptCandidate(c.ID); err != nil {
				return err
			}
			fmt.Println("Accepted.")
		case "r", "reject":
			if err := store.RejectCandidate(c.ID); err != nil {
				return err
			}
			fmt.Println("Rejected.")
		case "s", "skip", "":
			fmt.Println("Skipped.")
		case "q", "quit":
			return nil
		default:
			fmt.Println("Unknown action, skipped.")
		}
	}
	return nil
}

func runTodo(args []string) error {
	store, err := process.NewStore("")
	if err != nil {
		return err
	}
	defer store.Close()

	if len(args) == 0 || args[0] == "list" {
		todos, err := store.ListActiveTodos()
		if err != nil {
			return err
		}
		if len(todos) == 0 {
			fmt.Println("没有 active todos。")
			return nil
		}
		for _, t := range todos {
			priority := ""
			if t.HasPriority {
				priority = fmt.Sprintf(" P%d", t.Priority)
			}
			fmt.Printf("#%d%s %s\n", t.ID, priority, t.Text)
			if t.SourceDate != "" {
				fmt.Printf("   source: %s\n", t.SourceDate)
			}
		}
		return nil
	}
	if len(args) < 2 {
		return errors.New(todoUsage())
	}
	id, err := strconv.Atoi(args[1])
	if err != nil {
		return fmt.Errorf("invalid todo id %q", args[1])
	}
	switch args[0] {
	case "done":
		if len(args) != 2 {
			return errors.New(todoUsage())
		}
		if err := store.MarkTodoDone(id); err != nil {
			return err
		}
		fmt.Printf("Todo #%d done.\n", id)
	case "archive":
		if len(args) != 2 {
			return errors.New(todoUsage())
		}
		if err := store.ArchiveTodo(id); err != nil {
			return err
		}
		fmt.Printf("Todo #%d archived.\n", id)
	case "status":
		if len(args) != 3 {
			return errors.New(todoUsage())
		}
		if err := store.SetTodoStatus(id, args[2]); err != nil {
			return err
		}
		fmt.Printf("Todo #%d status -> %s.\n", id, args[2])
	case "priority":
		if len(args) != 3 {
			return errors.New(todoUsage())
		}
		if strings.EqualFold(args[2], "clear") || args[2] == "-" {
			if err := store.SetTodoPriority(id, nil); err != nil {
				return err
			}
			fmt.Printf("Todo #%d priority cleared.\n", id)
			return nil
		}
		priority, err := strconv.Atoi(args[2])
		if err != nil {
			return fmt.Errorf("invalid priority %q", args[2])
		}
		if err := store.SetTodoPriority(id, &priority); err != nil {
			return err
		}
		fmt.Printf("Todo #%d priority -> %d.\n", id, priority)
	default:
		return errors.New(todoUsage())
	}
	return nil
}

func todoUsage() string {
	return "用法: diary todo [list] | diary todo done <id> | diary todo archive <id> | diary todo status <id> <status> | diary todo priority <id> <0-100|clear>"
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
