package stats

import (
	"testing"
	"time"

	"github.com/borankux/dear-diary/internal/storage"
)

func makeDays(t *testing.T, s *storage.Storage, dates []string) {
	t.Helper()
	for _, d := range dates {
		tt, err := time.Parse("2006-01-02", d)
		if err != nil {
			t.Fatal(err)
		}
		path := s.PathFor(tt)
		if _, err := s.EnsureFile(path, tt); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCurrentStreakTodayWritten(t *testing.T) {
	s := storage.NewWithRoot(t.TempDir())
	makeDays(t, s, []string{"2026-06-23", "2026-06-24", "2026-06-25"})
	today, _ := time.Parse("2006-01-02", "2026-06-25")
	if got := CurrentStreak(s, today); got != 3 {
		t.Errorf("CurrentStreak = %d, want 3", got)
	}
}

func TestCurrentStreakTodayMissingButYesterdayWritten(t *testing.T) {
	// 今天还没写，昨天起连续 3 天
	s := storage.NewWithRoot(t.TempDir())
	makeDays(t, s, []string{"2026-06-22", "2026-06-23", "2026-06-24"})
	today, _ := time.Parse("2006-01-02", "2026-06-25")
	if got := CurrentStreak(s, today); got != 3 {
		t.Errorf("CurrentStreak = %d, want 3", got)
	}
}

func TestCurrentStreakGap(t *testing.T) {
	// 今天写了，但前天空了
	s := storage.NewWithRoot(t.TempDir())
	makeDays(t, s, []string{"2026-06-25", "2026-06-22"})
	today, _ := time.Parse("2006-01-02", "2026-06-25")
	if got := CurrentStreak(s, today); got != 1 {
		t.Errorf("CurrentStreak = %d, want 1", got)
	}
}

func TestCurrentStreakEmpty(t *testing.T) {
	s := storage.NewWithRoot(t.TempDir())
	today, _ := time.Parse("2006-01-02", "2026-06-25")
	if got := CurrentStreak(s, today); got != 0 {
		t.Errorf("CurrentStreak = %d, want 0", got)
	}
}

func TestLongestStreak(t *testing.T) {
	s := storage.NewWithRoot(t.TempDir())
	// 6/20-6/22 (3 days), gap, 6/24-6/25 (2 days)
	makeDays(t, s, []string{"2026-06-20", "2026-06-21", "2026-06-22", "2026-06-24", "2026-06-25"})
	first, _ := time.Parse("2006-01-02", "2026-06-20")
	today, _ := time.Parse("2006-01-02", "2026-06-25")
	if got := LongestStreak(s, first, today); got != 3 {
		t.Errorf("LongestStreak = %d, want 3", got)
	}
}

func TestTotalWritten(t *testing.T) {
	s := storage.NewWithRoot(t.TempDir())
	makeDays(t, s, []string{"2026-06-20", "2026-06-22", "2026-06-25"})
	first, _ := time.Parse("2006-01-02", "2026-06-20")
	today, _ := time.Parse("2006-01-02", "2026-06-25")
	if got := TotalWritten(s, first, today); got != 3 {
		t.Errorf("TotalWritten = %d, want 3", got)
	}
}
