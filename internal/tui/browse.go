package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/borankux/dear-diary/internal/editor"
	"github.com/borankux/dear-diary/internal/stats"
	"github.com/borankux/dear-diary/internal/storage"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// BrowseModel 是月历浏览模式的 bubbletea Model。
type BrowseModel struct {
	store       *storage.Storage
	year        int
	month       time.Month
	cursorDay   int
	writtenDays map[int]bool
	width       int
	height      int
	status      string
	quitting    bool
}

func NewBrowseModel(s *storage.Storage) *BrowseModel {
	now := time.Now()
	m := &BrowseModel{
		store:     s,
		year:      now.Year(),
		month:     now.Month(),
		cursorDay: now.Day(),
	}
	m.refreshWrittenDays()
	return m
}

func (m *BrowseModel) refreshWrittenDays() {
	m.writtenDays = m.store.WrittenDaysInMonth(m.year, m.month)
}

func (m BrowseModel) Init() tea.Cmd { return nil }

func (m BrowseModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		key := msg.String()
		m.status = ""
		switch key {
		case "q", "esc", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "?":
			m.status = "键位: hjkl 移动 · Enter 打开 · H/L 切月 · t 今天 · / 搜索 · q 退出"
			return m, nil
		case "/":
			m.status = "提示: 退出后用 diary search <关键词>"
			return m, nil
		case "h", "left":
			m.moveCursor(-1)
		case "l", "right":
			m.moveCursor(1)
		case "j", "down":
			m.moveCursor(7)
		case "k", "up":
			m.moveCursor(-7)
		case "g":
			m.cursorDay = 1
		case "G":
			m.cursorDay = daysInMonth(m.year, m.month)
		case "t":
			now := time.Now()
			m.year = now.Year()
			m.month = now.Month()
			m.cursorDay = now.Day()
			m.refreshWrittenDays()
		case "H", "<", "p":
			m.prevMonth()
		case "L", ">", "n":
			m.nextMonth()
		case "enter":
			return m, m.openEditor()
		}
	}
	return m, nil
}

func (m *BrowseModel) moveCursor(offset int) {
	t := time.Date(m.year, m.month, m.cursorDay, 0, 0, 0, 0, time.Local)
	t = t.AddDate(0, 0, offset)
	if t.Year() == m.year && t.Month() == m.month {
		m.cursorDay = t.Day()
	} else if offset < 0 {
		m.cursorDay = 1
	} else {
		m.cursorDay = daysInMonth(m.year, m.month)
	}
}

func (m *BrowseModel) prevMonth() {
	if m.month == 1 {
		m.year--
		m.month = 12
	} else {
		m.month--
	}
	m.cursorDay = 1
	m.refreshWrittenDays()
}

func (m *BrowseModel) nextMonth() {
	if m.month == 12 {
		m.year++
		m.month = 1
	} else {
		m.month++
	}
	m.cursorDay = 1
	m.refreshWrittenDays()
}

// openEditor 启动 Vim 打开当前光标日期。
// 命令准备好后由 bubbletea 用 tea.ExecProcess 暂停 TUI 执行。
func (m BrowseModel) openEditor() tea.Cmd {
	t := time.Date(m.year, m.month, m.cursorDay, 0, 0, 0, 0, time.Local)
	path := m.store.PathFor(t)
	now := time.Now()
	isExist := m.writtenDays[m.cursorDay]
	isToday := sameDay(t, now)

	// 同步准备好文件
	if !isExist {
		if _, err := m.store.EnsureFile(path, t); err != nil {
			return func() tea.Msg { return errMsg{err} }
		}
	} else if isToday {
		if !endsWithRecentTimestamp(path, now) {
			if err := m.store.AppendTimestamp(path, now); err != nil {
				return func() tea.Msg { return errMsg{err} }
			}
		}
	}

	cmd := editor.CmdFor(path, isExist)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			return errMsg{err}
		}
		// Vim 退出后刷新当月已写天数据
		return editorDoneMsg{}
	})
}

func (m BrowseModel) View() string {
	width := clampInt(m.width, 40, 80)

	// 标题
	now := time.Now()
	todayStr := ""
	if m.year == now.Year() && m.month == now.Month() {
		todayStr = fmt.Sprintf(" (今天 %d 月 %d 日)", int(now.Month()), now.Day())
	}
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("12")).
		Render(fmt.Sprintf("◀  %d 年 %d 月  ▶", m.year, int(m.month))) +
		lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(todayStr)

	writtenCount := len(m.writtenDays)
	totalDays := daysInMonth(m.year, m.month)

	// 当前 streak（基于今天，跟当前显示月份无关）
	streak := stats.CurrentStreak(m.store, time.Now())

	var statsLine string
	if streak > 0 {
		statsLine = fmt.Sprintf("🔥 %d 天  ·  %d/%d 天", streak, writtenCount, totalDays)
	} else {
		statsLine = fmt.Sprintf("%d/%d 天", writtenCount, totalDays)
	}
	statsRender := lipgloss.NewStyle().
		Foreground(lipgloss.Color("11")).
		Render(statsLine)

	// 把 title 和 stats 推到左右两端
	header := padBetween(title, statsRender, width)

	// 分隔线
	rule := strings.Repeat("─", width)

	// 星期表头（周一开头）
	weekdays := []string{"一", "二", "三", "四", "五", "六", "日"}
	headerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	cells := make([]string, 7)
	for i, w := range weekdays {
		cells[i] = headerStyle.Render(centerString(w, 6))
	}
	weekdayRow := strings.Join(cells, "")

	// 月历网格
	dayStyle := lipgloss.NewStyle()
	writtenStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	todayStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	cursorStyle := lipgloss.NewStyle().Bold(true).Background(lipgloss.Color("8")).Foreground(lipgloss.Color("15"))

	firstDay := time.Date(m.year, m.month, 1, 0, 0, 0, 0, time.Local)
	firstWeekday := (int(firstDay.Weekday()) + 6) % 7

	isCurrentMonth := now.Year() == m.year && now.Month() == m.month
	today := now.Day()

	var grid strings.Builder
	pos := 0
	for i := 0; i < firstWeekday; i++ {
		grid.WriteString(strings.Repeat(" ", 6))
		pos++
	}
	for d := 1; d <= totalDays; d++ {
		var s string
		switch {
		case isCurrentMonth && d == today:
			s = todayStyle.Render(fmt.Sprintf("%4d◆", d))
		case m.writtenDays[d]:
			s = writtenStyle.Render(fmt.Sprintf("%4d●", d))
		default:
			s = dayStyle.Render(fmt.Sprintf("%4d ", d))
		}
		if d == m.cursorDay {
			s = cursorStyle.Render(fmt.Sprintf("%5s", fmt.Sprintf("%d", d))) +
				renderMarker(d, isCurrentMonth, today, m.writtenDays, false)
		}
		grid.WriteString(s)
		grid.WriteString(" ")
		pos++
		if pos%7 == 0 {
			grid.WriteString("\n")
		}
	}

	// 帮助行
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	help := helpStyle.Render("hjkl/方向键 移动 · ⏎ 打开 · H/L 切月 · t 今天 · / 搜索 · ? 帮助 · q 退出")

	var out strings.Builder
	out.WriteString(header)
	out.WriteString("\n")
	out.WriteString(rule)
	out.WriteString("\n")
	out.WriteString(weekdayRow)
	out.WriteString("\n")
	out.WriteString(grid.String())
	out.WriteString("\n")
	out.WriteString(rule)
	out.WriteString("\n")
	out.WriteString(help)
	if m.status != "" {
		out.WriteString("\n")
		out.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Render(m.status))
	}
	return out.String()
}

func renderMarker(day int, isCurrentMonth bool, today int, written map[int]bool, _ bool) string {
	if isCurrentMonth && day == today {
		return "◆"
	}
	if written[day] {
		return "●"
	}
	return " "
}

type errMsg struct{ err error }

func (e errMsg) Error() string { return e.err.Error() }

type editorDoneMsg struct{}

func daysInMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.Local).Day()
}

func sameDay(a, b time.Time) bool {
	return a.Year() == b.Year() && a.Month() == b.Month() && a.Day() == b.Day()
}

func endsWithRecentTimestamp(path string, now time.Time) bool {
	data, err := osReadFile(path)
	if err != nil {
		return false
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "## ") {
			return false
		}
		ts := strings.TrimSpace(strings.TrimPrefix(line, "## "))
		t, err := time.ParseInLocation("15:04", ts, time.Local)
		if err != nil {
			return false
		}
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
