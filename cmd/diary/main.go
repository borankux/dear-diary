package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
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
	"github.com/borankux/dear-diary/internal/web"
)

const version = "0.6.0"

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
  diary dashboard        在浏览器打开交互式看板 (SPA)
  diary calendar         在浏览器打开日历视图 (SPA)
  diary serve            启动后台 Web 服务 (默认打开看板)
  diary serve --calendar 启动并打开日历视图
  diary serve --no-open  只启动服务，不打开浏览器
  diary sync             将本地日记同步到远程服务器 (git push)
  diary sync pull        从远程服务器拉取日记 (git pull)
  diary remote setup <url> [password]  配置远程服务器并登录
  diary remote login [password]        登录已配置的服务器
  diary remote status                检查连接
  diary remote stats                 显示服务器统计
  diary remote todo                  显示服务器 todos（文本表格）
  diary remote candidates            显示服务器候选
  diary remote memories              显示服务器记忆
  diary remote diary [date]          查看某天日记
  diary remote calendar              显示日历
  diary remote search <query>        搜索日记
  diary remote accept <id>           接受候选
  diary remote reject <id>           拒绝候选
  diary remote done <id>             标记 todo 完成
  diary -h | --help      显示本帮助
  diary -v | --version   显示版本号

  看板和日历均为交互式 Web 应用，启动后会持续在后台运行。
  使用 Ctrl+C 或在终端中关闭来停止服务。

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
		must(runWebServer(args[1:], "dashboard"))
		return
	case "calendar":
		must(runWebServer(args[1:], "calendar"))
		return
	case "serve":
		must(runWebServer(args[1:], "dashboard"))
		return
	case "sync":
		must(runSync(args[1:]))
		return
	case "remote":
		must(runRemote(args[1:]))
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

// runSync 执行 Git 同步操作。
// diary sync     → git add . && git commit -m "..." && git push
// diary sync pull → git pull
func runSync(args []string) error {
	s := storage.New()
	rootDir := s.RootDir()

	// 检查日记目录是否是 Git 仓库
	if _, err := os.Stat(filepath.Join(rootDir, ".git")); os.IsNotExist(err) {
		fmt.Println("日记目录还不是 Git 仓库。正在初始化...")
		if err := execGit(rootDir, "init"); err != nil {
			return fmt.Errorf("git init 失败: %w", err)
		}
		// 添加 .gitignore 排除 process/
		gitignore := `/process/
/dashboard.html
/entries/
*.db
`
		if err := os.WriteFile(filepath.Join(rootDir, ".gitignore"), []byte(gitignore), 0o644); err != nil {
			return fmt.Errorf("创建 .gitignore 失败: %w", err)
		}
		fmt.Println("已初始化 Git 仓库并创建 .gitignore")
	}

	// 检查/配置 remote
	remoteURL := os.Getenv("DIARY_REMOTE_URL")
	remoteName := "origin"
	if remoteURL == "" {
		// 尝试从现有 remote 读取
		cmd := exec.Command("git", "remote", "get-url", "origin")
		cmd.Dir = rootDir
		out, err := cmd.Output()
		if err == nil && len(out) > 0 {
			remoteURL = strings.TrimSpace(string(out))
		} else {
			return fmt.Errorf("未设置 DIARY_REMOTE_URL 环境变量，且本地没有配置 origin remote\n\n请设置环境变量:\n  export DIARY_REMOTE_URL=ssh://pb/var/lib/dear-diary\n\n或在日记目录中手动配置:\n  cd %s && git remote add origin ssh://pb/var/lib/dear-diary", rootDir)
		}
	} else if strings.Contains(remoteURL, "://") || strings.Contains(remoteURL, "@") {
		// remoteURL 是一个 URL，需要配置 remote
		cmd := exec.Command("git", "remote", "add", "origin", remoteURL)
		cmd.Dir = rootDir
		if err := cmd.Run(); err != nil {
			// 可能已经存在，尝试更新
			cmd = exec.Command("git", "remote", "set-url", "origin", remoteURL)
			cmd.Dir = rootDir
			_ = cmd.Run()
		}
	} else {
		remoteName = remoteURL
	}

	branch := "main"

	// 如果是 pull 命令
	if len(args) > 0 && args[0] == "pull" {
		fmt.Println("正在从远程拉取日记...")
		if err := execGit(rootDir, "pull", remoteName, branch); err != nil {
			// 尝试 master 分支
			if err := execGit(rootDir, "pull", remoteName, "master"); err != nil {
				return fmt.Errorf("git pull 失败: %w", err)
			}
		}
		fmt.Println("同步完成：已从远程拉取最新日记")
		return nil
	}

	// 默认 push
	fmt.Println("正在同步日记到远程服务器...")
	if err := execGit(rootDir, "add", "."); err != nil {
		return fmt.Errorf("git add 失败: %w", err)
	}

	// 检查是否有变更要提交
	hasChanges, err := gitHasChanges(rootDir)
	if err != nil {
		return err
	}
	if !hasChanges {
		fmt.Println("没有变更需要提交，直接推送...")
		if err := execGit(rootDir, "push", remoteName, branch); err != nil {
			if err := execGit(rootDir, "push", remoteName, "master"); err != nil {
				return fmt.Errorf("git push 失败: %w", err)
			}
		}
		fmt.Println("同步完成")
		return nil
	}

	commitMsg := fmt.Sprintf("sync: %s", time.Now().Format("2006-01-02 15:04:05"))
	if err := execGit(rootDir, "commit", "-m", commitMsg); err != nil {
		return fmt.Errorf("git commit 失败: %w", err)
	}
	if err := execGit(rootDir, "push", remoteName, branch); err != nil {
		if err := execGit(rootDir, "push", remoteName, "master"); err != nil {
			return fmt.Errorf("git push 失败: %w", err)
		}
	}
	fmt.Println("同步完成：已推送日记到远程服务器")
	return nil
}

func execGit(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func gitHasChanges(dir string) (bool, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("git status 失败: %w", err)
	}
	return len(strings.TrimSpace(string(out))) > 0, nil
}

// runWebServer 启动 HTTP 服务并打开浏览器。
// page 可以是 "dashboard" 或 "calendar"。
func runWebServer(args []string, page string) error {
	addr := "0.0.0.0:8765"
	noOpen := false

	for _, a := range args {
		switch a {
		case "--no-open":
			noOpen = true
		case "--calendar":
			page = "calendar"
		case "--dashboard":
			page = "dashboard"
		case "--port":
			// handled below
		}
	}

	// Check if already running
	if !noOpen {
		resp, err := http.Get("http://" + addr + "/api/stats")
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			// Already running, just open browser
			url := "http://localhost:8765/#/" + page
			fmt.Printf("Web 服务已在运行: %s\n", url)
			return web.OpenBrowser(url)
		}
	}

	server := web.NewServer(addr)
	if !noOpen {
		go func() {
			// Wait a moment for server to start
			time.Sleep(600 * time.Millisecond)
			url := "http://localhost:8765/#/" + page
			if err := web.OpenBrowser(url); err != nil {
				fmt.Fprintf(os.Stderr, "打开浏览器失败: %v\n", err)
			}
		}()
	}
	fmt.Println("正在启动 Dear Diary Web 服务...")
	fmt.Printf("  看板: http://localhost:8765/#/dashboard\n")
	fmt.Printf("  日历: http://localhost:8765/#/calendar\n")
	fmt.Printf("  搜索: http://localhost:8765/#/search\n")
	fmt.Println("按 Ctrl+C 停止服务")
	return server.Start()
}
