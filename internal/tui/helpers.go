package tui

import (
	"os"
	"strings"
)

func osReadFile(path string) ([]byte, error) { return os.ReadFile(path) }

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// padBetween 把 left 和 right 推到一行两端，用空格填充。
func padBetween(left, right string, width int) string {
	visibleLen := visibleWidth(left) + visibleWidth(right)
	pad := width - visibleLen
	if pad < 1 {
		pad = 1
	}
	return left + strings.Repeat(" ", pad) + right
}

// centerString 把 s 居中到 width 个字符宽度（中文按 2 计）。
func centerString(s string, width int) string {
	w := runeWidth(s)
	if w >= width {
		return s
	}
	left := (width - w) / 2
	right := width - w - left
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", right)
}

// 显示宽度估算：CJK 字符按 2，其他按 1。
func runeWidth(s string) int {
	w := 0
	for _, r := range s {
		if r > 0x2E80 {
			w += 2
		} else {
			w++
		}
	}
	return w
}

func visibleWidth(s string) int {
	// 去掉 ANSI 转义后的可见宽度
	return runeWidth(stripAnsi(s))
}

func stripAnsi(s string) string {
	var b strings.Builder
	in := false
	for _, r := range s {
		if r == '\x1b' {
			in = true
			continue
		}
		if in {
			if r == 'm' {
				in = false
			}
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
