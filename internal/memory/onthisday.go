// Package memory 提供"X 年前的今天"回顾提醒。
package memory

import (
	"os"
	"strings"
	"time"

	"github.com/borankux/dear-diary/internal/storage"
)

// Memory 表示一条历史回顾。
type Memory struct {
	YearsAgo  int       // 距今天几年
	Date      time.Time // 那一天
	FirstLine string    // 第一段非空内容（去除标题/时间戳后）
	Path      string    // 文件路径
}

// OnThisDay 查找历史中"X 年前的今天"的日记。
// 从 1 年前开始向前找，最多查 maxYears 年。
// 仅返回有内容的日记（纯模板的不算）。
func OnThisDay(s *storage.Storage, today time.Time, maxYears int) []Memory {
	if maxYears < 1 {
		maxYears = 1
	}
	var memories []Memory
	for years := 1; years <= maxYears; years++ {
		past := today.AddDate(-years, 0, 0)
		path := s.PathFor(past)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		line := firstMeaningfulLine(string(data))
		if line == "" {
			continue
		}
		memories = append(memories, Memory{
			YearsAgo:  years,
			Date:      past,
			FirstLine: line,
			Path:      path,
		})
	}
	return memories
}

// firstMeaningfulLine 从 Markdown 内容里提取第一段有意义的文字。
// 跳过: # 标题、## 时间戳、空行。
func firstMeaningfulLine(content string) string {
	lines := strings.Split(content, "\n")
	pastTitle := false
	for _, line := range lines {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "# ") {
			pastTitle = true
			continue
		}
		if !pastTitle {
			continue
		}
		if strings.HasPrefix(t, "## ") {
			continue
		}
		if t == "" {
			continue
		}
		return t
	}
	return ""
}
