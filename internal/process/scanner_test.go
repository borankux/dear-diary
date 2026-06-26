package process

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestScannerAllFiles(t *testing.T) {
	dir := t.TempDir()

	// Valid diary file.
	valid := filepath.Join(dir, "2026-06", "2026-06-25.md")
	if err := os.MkdirAll(filepath.Dir(valid), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(valid, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Invalid names should be ignored.
	invalid1 := filepath.Join(dir, "notes.md")
	invalid2 := filepath.Join(dir, "2026-06", "random.txt")
	invalid3 := filepath.Join(dir, "process", "2026-06-25.md")
	for _, p := range []string{invalid1, invalid2, invalid3} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	sc := NewScanner(dir)
	files, err := sc.AllFiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if files[0].Path != valid {
		t.Fatalf("expected %s, got %s", valid, files[0].Path)
	}
	if files[0].Hash != HashContent([]byte("hello")) {
		t.Fatal("hash mismatch")
	}
}

func TestScannerRecentFiles(t *testing.T) {
	dir := t.TempDir()

	oldFile := filepath.Join(dir, "2026-06", "2026-06-01.md")
	newFile := filepath.Join(dir, "2026-06", "2026-06-25.md")
	for _, p := range []string{oldFile, newFile} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Backdate the old file.
	oldTime := time.Now().AddDate(0, 0, -10)
	if err := os.Chtimes(oldFile, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	sc := NewScanner(dir)
	recent, err := sc.RecentFiles(3)
	if err != nil {
		t.Fatal(err)
	}
	if len(recent) != 1 {
		t.Fatalf("expected 1 recent file, got %d", len(recent))
	}
	if recent[0].Path != newFile {
		t.Fatalf("expected %s, got %s", newFile, recent[0].Path)
	}
}
