package search

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/borankux/dear-diary/internal/storage"
)

// Result 表示一条搜索匹配。
type Result struct {
	File string // 完整路径
	Date string // YYYY-MM-DD (从文件名提取)
	Line int    // 行号
	Text string // 匹配行的原文
}

// Search 在 root 下搜索包含 keyword 的所有 .md 文件。
// 优先用 ripgrep（如果系统有），否则回退到 Go 实现。
// 结果按日期倒序（最新的在前）。
func Search(root, keyword string) ([]Result, error) {
	if keyword == "" {
		return nil, nil
	}
	if _, err := exec.LookPath("rg"); err == nil {
		return searchWithRg(root, keyword)
	}
	return searchWithGo(root, keyword)
}

func searchWithRg(root, keyword string) ([]Result, error) {
	cmd := exec.Command("rg",
		"--line-number",
		"--no-heading",
		"--color=never",
		"--with-filename",
		"--glob=*.md",
		keyword,
		root,
	)
	out, err := cmd.Output()
	if err != nil {
		// rg exit code 1 = 没匹配，不算错误
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return nil, nil
		}
		return nil, err
	}
	return sortByDate(parseRgOutput(string(out))), nil
}

func parseRgOutput(s string) []Result {
	var results []Result
	for _, line := range strings.Split(s, "\n") {
		if line == "" {
			continue
		}
		idx1 := strings.Index(line, ":")
		if idx1 < 0 {
			continue
		}
		path := line[:idx1]
		if !storage.IsDiaryFilePath(path) {
			continue
		}
		rest := line[idx1+1:]
		idx2 := strings.Index(rest, ":")
		if idx2 < 0 {
			continue
		}
		var lineNum int
		for _, c := range rest[:idx2] {
			if c >= '0' && c <= '9' {
				lineNum = lineNum*10 + int(c-'0')
			}
		}
		content := rest[idx2+1:]
		results = append(results, Result{
			File: path,
			Date: dateFromPath(path),
			Line: lineNum,
			Text: content,
		})
	}
	return results
}

func searchWithGo(root, keyword string) ([]Result, error) {
	var results []Result
	kw := strings.ToLower(keyword)
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !storage.IsDiaryFilePath(path) {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			if strings.Contains(strings.ToLower(scanner.Text()), kw) {
				results = append(results, Result{
					File: path,
					Date: dateFromPath(path),
					Line: lineNum,
					Text: scanner.Text(),
				})
			}
		}
		return nil
	})
	return sortByDate(results), err
}

func sortByDate(results []Result) []Result {
	sort.Slice(results, func(i, j int) bool {
		if results[i].Date != results[j].Date {
			return results[i].Date > results[j].Date
		}
		return results[i].Line < results[j].Line
	})
	return results
}

func dateFromPath(path string) string {
	return strings.TrimSuffix(filepath.Base(path), ".md")
}
