package tui

import (
	"fmt"
	"strings"

	"github.com/borankux/dear-diary/internal/editor"
	"github.com/borankux/dear-diary/internal/search"
	"github.com/borankux/dear-diary/internal/storage"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type SearchModel struct {
	store    *storage.Storage
	results  []search.Result
	keyword  string
	cursor   int
	offset   int
	width    int
	height   int
	status   string
	quitting bool
}

func NewSearchModel(s *storage.Storage, results []search.Result, keyword string) *SearchModel {
	return &SearchModel{
		store:   s,
		results: results,
		keyword: keyword,
	}
}

func (m SearchModel) Init() tea.Cmd { return nil }

func (m SearchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		m.status = ""
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "j", "down":
			if m.cursor < len(m.results)-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "g":
			m.cursor = 0
		case "G":
			m.cursor = len(m.results) - 1
		case "n":
			if m.cursor+10 < len(m.results) {
				m.cursor += 10
			} else {
				m.cursor = len(m.results) - 1
			}
		case "N":
			if m.cursor-10 > 0 {
				m.cursor -= 10
			} else {
				m.cursor = 0
			}
		case "enter":
			return m, m.openResult()
		}
		m.adjustOffset()
	}
	return m, nil
}

func (m *SearchModel) adjustOffset() {
	visibleRows := m.height - 6
	if visibleRows < 5 {
		visibleRows = 5
	}
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+visibleRows {
		m.offset = m.cursor - visibleRows + 1
	}
}

func (m SearchModel) openResult() tea.Cmd {
	if len(m.results) == 0 {
		return nil
	}
	r := m.results[m.cursor]
	cmd := editor.CmdFor(r.File, true)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			return errMsg{err}
		}
		return nil
	})
}

func (m SearchModel) View() string {
	width := clampInt(m.width, 40, 100)
	rule := strings.Repeat("─", width)

	var b strings.Builder

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	rightInfo := lipgloss.NewStyle().Foreground(lipgloss.Color("11")).
		Render(fmt.Sprintf("结果 %d/%d", m.cursor+1, len(m.results)))
	header := padBetween(
		titleStyle.Render(fmt.Sprintf("搜索: %q", m.keyword)),
		rightInfo,
		width,
	)
	b.WriteString(header)
	b.WriteString("\n")
	b.WriteString(rule)
	b.WriteString("\n")

	if len(m.results) == 0 {
		b.WriteString("\n  没有匹配的日记\n")
		b.WriteString(rule)
		return b.String()
	}

	visibleRows := m.height - 6
	if visibleRows < 5 {
		visibleRows = 5
	}
	end := m.offset + visibleRows
	if end > len(m.results) {
		end = len(m.results)
	}

	dateStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Width(12)
	lineStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Width(5).Align(lipgloss.Right)
	cursorStyle := lipgloss.NewStyle().Bold(true).Background(lipgloss.Color("8")).Foreground(lipgloss.Color("15"))
	normalStyle := lipgloss.NewStyle()

	for i := m.offset; i < end; i++ {
		r := m.results[i]
		datePart := dateStyle.Render(r.Date)
		linePart := lineStyle.Render(fmt.Sprintf("L%d", r.Line))
		text := truncate(r.Text, width-25)
		row := fmt.Sprintf("  %s %s  %s", datePart, linePart, text)
		if i == m.cursor {
			row = cursorStyle.Render(row)
		} else {
			row = normalStyle.Render(row)
		}
		b.WriteString(row)
		b.WriteString("\n")
	}

	b.WriteString(rule)
	b.WriteString("\n")
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	b.WriteString(helpStyle.Render("j/k 上下 · ⏎ 打开 · n/N 翻页 · g/G 首/末 · q 退出"))
	if m.status != "" {
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Render(m.status))
	}
	return b.String()
}

func truncate(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	w := runeWidth(s)
	if maxLen > 0 && w > maxLen {
		// 简单按 rune 截断
		out := ""
		curW := 0
		for _, r := range s {
			rw := 1
			if r > 0x2E80 {
				rw = 2
			}
			if curW+rw > maxLen-1 {
				break
			}
			out += string(r)
			curW += rw
		}
		return out + "…"
	}
	return s
}
