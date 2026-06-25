package editor

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// CmdFor 构造打开指定文件的 *exec.Cmd。
// appendMode=true 时（仅对 vim 系列编辑器生效）会自动跳到文件末尾的追加位置。
// 调用方负责设置 Stdin/Stdout/Stderr (tea.ExecProcess 会自动接管)。
func CmdFor(path string, appendMode bool) *exec.Cmd {
	ed := firstNonEmpty(
		os.Getenv("DIARY_EDITOR"),
		os.Getenv("EDITOR"),
		"vim",
	)
	args := splitEditorArgs(ed)
	if appendMode && isVimLike(args[0]) {
		// "+normal Go" 让 Vim 启动后: G 跳到末行, o 向下开新行并进入插入模式
		args = append(args, "+normal Go")
	}
	args = append(args, path)

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}

// Open 同步运行编辑器。适用于非 TUI 模式（CLI 直接打开）。
// TUI 模式请用 tea.ExecProcess(editor.CmdFor(...)) 让 bubbletea 释放 raw mode。
func Open(path string, appendMode bool) error {
	return CmdFor(path, appendMode).Run()
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// splitEditorArgs 支持 "code -w" / "vim" / "/usr/local/bin/nvim" 等形式。
func splitEditorArgs(editor string) []string {
	parts := strings.Fields(editor)
	if len(parts) == 0 {
		return []string{"vim"}
	}
	return parts
}

func isVimLike(cmd string) bool {
	base := filepath.Base(cmd)
	return base == "vim" || base == "nvim" ||
		strings.HasSuffix(base, "vim") || strings.HasSuffix(base, "vi")
}
