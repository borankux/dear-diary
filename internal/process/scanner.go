package process

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/borankux/dear-diary/internal/storage"
)

// Scanner discovers diary markdown files and computes their content hashes.
type Scanner struct {
	rootDir string
}

// NewScanner creates a scanner for the given diary root directory.
func NewScanner(rootDir string) *Scanner {
	return &Scanner{rootDir: rootDir}
}

// AllFiles walks the diary root and returns all .md files sorted by path.
func (sc *Scanner) AllFiles() ([]FileInfo, error) {
	var files []FileInfo

	err := filepath.Walk(sc.rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// Ignore permission errors and continue.
			return nil
		}
		if info.IsDir() || !storage.IsDiaryFilePath(path) {
			return nil
		}

		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return fmt.Errorf("read %s: %w", path, readErr)
		}

		files = append(files, FileInfo{
			Path:    path,
			Hash:    HashContent(content),
			ModTime: info.ModTime().UTC(),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].Path < files[j].Path
	})
	return files, nil
}

// RecentFiles returns files modified within the last N days.
func (sc *Scanner) RecentFiles(days int) ([]FileInfo, error) {
	all, err := sc.AllFiles()
	if err != nil {
		return nil, err
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -days)
	var recent []FileInfo
	for _, f := range all {
		if f.ModTime.After(cutoff) {
			recent = append(recent, f)
		}
	}
	return recent, nil
}
