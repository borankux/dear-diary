// Package stats 计算日记写作的连续天数等统计。
package stats

import (
	"time"

	"github.com/borankux/dear-diary/internal/storage"
)

// CurrentStreak 计算从 today 倒推的连续写作天数。
//
// 规则（温和版）:
//   - 如果今天已写 → 从今天开始往前数，连续几天有日记就算几天
//   - 如果今天还没写 → 从昨天开始算（让用户在今晚写之前看到昨天的 streak，
//     不至于 "0 天" 打击信心）
//   - 一旦遇到空缺就停
//
// 这是对养成习惯友好的口径，跟 GitHub contribution 的逻辑一致。
func CurrentStreak(s *storage.Storage, today time.Time) int {
	cursor := today
	if !s.Exists(cursor) {
		cursor = cursor.AddDate(0, 0, -1)
	}
	streak := 0
	for s.Exists(cursor) {
		streak++
		cursor = cursor.AddDate(0, 0, -1)
	}
	return streak
}

// LongestStreak 扫描从 firstDay 到 today 之间的最长连续写作天数。
// firstDay 一般是第一篇日记的日期。
func LongestStreak(s *storage.Storage, firstDay, today time.Time) int {
	if firstDay.After(today) {
		return 0
	}
	best := 0
	cur := 0
	for d := firstDay; !d.After(today); d = d.AddDate(0, 0, 1) {
		if s.Exists(d) {
			cur++
			if cur > best {
				best = cur
			}
		} else {
			cur = 0
		}
	}
	return best
}

// TotalWritten 统计从 firstDay 到 today 之间写过的总天数。
func TotalWritten(s *storage.Storage, firstDay, today time.Time) int {
	if firstDay.After(today) {
		return 0
	}
	total := 0
	for d := firstDay; !d.After(today); d = d.AddDate(0, 0, 1) {
		if s.Exists(d) {
			total++
		}
	}
	return total
}
