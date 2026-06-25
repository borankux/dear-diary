package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const appName = "dear-diary"

// Storage 封装日记文件系统的所有操作。
// 路径约定: rootDir/YYYY-MM/YYYY-MM-DD.md
type Storage struct {
	rootDir string
}

// New 返回使用 ~/Documents/dear-diary 的 Storage。
// 如果 $DIARY_DIR 设置，则使用该路径（便于测试和自定义）。
func New() *Storage {
	if dir := os.Getenv("DIARY_DIR"); dir != "" {
		return &Storage{rootDir: dir}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		// 极端情况退化到当前工作目录
		home = "."
	}
	return &Storage{rootDir: filepath.Join(home, "Documents", appName)}
}

// NewWithRoot 用指定根目录构造（测试用）。
func NewWithRoot(root string) *Storage {
	return &Storage{rootDir: root}
}

func (s *Storage) RootDir() string { return s.rootDir }

// PathFor 返回日期对应的完整文件路径，不保证文件存在。
func (s *Storage) PathFor(t time.Time) string {
	monthDir := filepath.Join(s.rootDir, t.Format("2006-01"))
	return filepath.Join(monthDir, t.Format("2006-01-02")+".md")
}

// MonthDir 返回年月对应的目录路径。
func (s *Storage) MonthDir(year int, month time.Month) string {
	return filepath.Join(s.rootDir, fmt.Sprintf("%04d-%02d", year, int(month)))
}

// Exists 检查某天的日记是否存在。
func (s *Storage) Exists(t time.Time) bool {
	_, err := os.Stat(s.PathFor(t))
	return err == nil
}

// EnsureFile 创建文件（如果不存在）并写入初始模板。
// 返回 (isNew, error)，isExist 时 isNew=false。
func (s *Storage) EnsureFile(path string, t time.Time) (bool, error) {
	if _, err := os.Stat(path); err == nil {
		return false, nil
	} else if !os.IsNotExist(err) {
		return false, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	content := BuildInitialContent(t)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return false, err
	}
	return true, nil
}

// AppendTimestamp 在文件末尾追加新的时间戳段落。
func (s *Storage) AppendTimestamp(path string, t time.Time) error {
	content := "\n\n## " + t.Format("15:04") + "\n\n"
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(content)
	return err
}

// WrittenDaysInMonth 扫描月份目录，返回该月哪些天写了日记。
// 目录不存在时返回空 map（不报错）。
func (s *Storage) WrittenDaysInMonth(year int, month time.Month) map[int]bool {
	days := make(map[int]bool)
	entries, err := os.ReadDir(s.MonthDir(year, month))
	if err != nil {
		return days
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".md")
		parts := strings.Split(name, "-")
		if len(parts) != 3 {
			continue
		}
		var d int
		fmt.Sscanf(parts[2], "%d", &d)
		if d >= 1 && d <= 31 {
			days[d] = true
		}
	}
	return days
}

// AllMarkdownFiles 返回所有 .md 文件，按字典序（=时间序）排列。
func (s *Storage) AllMarkdownFiles() ([]string, error) {
	var files []string
	err := filepath.Walk(s.rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		files = append(files, path)
		return nil
	})
	return files, err
}

// BuildInitialContent 生成新日记的初始内容。
// 格式:
//
//	# YYYY-MM-DD 周X
//
//	## HH:MM
func BuildInitialContent(t time.Time) string {
	weekdayCN := []string{"日", "一", "二", "三", "四", "五", "六"}
	return fmt.Sprintf("# %s 周%s\n\n## %s\n\n",
		t.Format("2006-01-02"),
		weekdayCN[int(t.Weekday())],
		t.Format("15:04"))
}
