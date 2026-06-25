# 亲爱的日记 (dear-diary)

[![CI](https://github.com/borankux/dear-diary/actions/workflows/ci.yml/badge.svg)](https://github.com/borankux/dear-diary/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/borankux/dear-diary.svg)](https://pkg.go.dev/github.com/borankux/dear-diary)

亲爱的日记是一个 Vim-first 的 macOS 终端日记工具：输入 `diary` 直接用 Vim 写今天，输入 `diary browse` 用 TUI 月历回看，输入 `diary search <keyword>` 全文搜索 Markdown 日记。

Dear Diary is a Vim-first terminal journal app for macOS. It stores plain Markdown files, opens entries in your editor, provides a Bubble Tea calendar browser, and searches journal history from the command line.

## Why dear-diary

- **Vim-native writing**: reuse your `~/.vimrc`, plugins, muscle memory, and terminal workflow.
- **TUI calendar browsing**: see written days and jump through months without leaving the terminal.
- **Safe same-day append**: reopening today adds a new timestamp section instead of overwriting earlier notes.
- **Plain Markdown storage**: readable, portable, git-friendly, and compatible with Obsidian or any text editor.
- **Fast single binary**: Go CLI with no runtime service, database, account, or cloud dependency.
- **Private by default**: journal data lives locally under `~/Documents/dear-diary/`.

## Use cases

- Daily journaling from the terminal
- Work log and engineering diary
- Vim-based personal notes
- Local-first Markdown journal archive
- Lightweight command-line diary search

## Install

```bash
git clone https://github.com/borankux/dear-diary.git
cd dear-diary
make install
```

Or build locally:

```bash
make build
./bin/diary --version
```

Once the module is indexed by Go tooling, you can also install with:

```bash
go install github.com/borankux/dear-diary/cmd/diary@latest
```

## Usage

| Command | Behavior |
|---|---|
| `diary` | Open today's journal in Vim |
| `diary browse` | Open the TUI calendar browser |
| `diary today` | Open today explicitly |
| `diary yesterday` or `diary y` | Open yesterday |
| `diary 2026-06-24` | Open a specific date |
| `diary 6/24` | Open month/day in the current year |
| `diary search keyword` | Search all journal entries |
| `diary -h` | Show help |

## Daily writing model

The first `diary` call creates today's file:

```markdown
# 2026-06-25 周四

## 09:00

今天开始做 TUI 日记应用。
```

Opening `diary` again later appends a fresh section:

```markdown
## 22:30

下班前把 search 功能也加上了。
```

If you reopen within five minutes, dear-diary avoids creating duplicate empty timestamp sections.

## TUI calendar

```text
hjkl / arrow keys    Move cursor
H / L or < / >       Previous / next month
t                    Jump to today
g / G                Jump to month start / month end
Enter                Open selected day
/                    Show search hint
?                    Toggle help
q / Esc              Quit
```

Visual markers:

- `◆` today
- `●` day with a journal entry
- highlighted cell: current cursor

## Editor selection

Priority:

1. `$DIARY_EDITOR`
2. `$EDITOR`
3. `vim`

Examples:

```bash
export DIARY_EDITOR=nvim
export DIARY_EDITOR="code -w"
```

Vim-compatible editors get cursor positioning at the append point. Other editors still open the Markdown file normally.

## Storage

```text
~/Documents/dear-diary/
  2026-06/
    2026-06-23.md
    2026-06-24.md
    2026-06-25.md
  2026-07/
    2026-07-01.md
```

- Default path: `~/Documents/dear-diary/YYYY-MM/YYYY-MM-DD.md`
- Override root path with `$DIARY_DIR`
- Works well with iCloud Drive, Dropbox, Time Machine, git, or any file backup flow

## Search

Search uses `rg` when ripgrep is installed and falls back to pure Go scanning when it is not.

```bash
diary search bubbletea
```

Results are shown in a TUI list. Press `Enter` to open a matching entry, `j/k` to move, and `q` to quit.

## Development

```bash
make test
make build
make fmt vet
```

Tech stack:

- Go 1.26+
- [Bubble Tea](https://github.com/charmbracelet/bubbletea)
- [Lipgloss](https://github.com/charmbracelet/lipgloss)

Project layout:

```text
cmd/diary/          CLI entrypoint
internal/storage/   Markdown file paths, creation, append, and scanning
internal/editor/    Editor integration
internal/search/    ripgrep and pure Go search
internal/tui/       Bubble Tea calendar and search views
docs/spec.md        Product and implementation spec
```

## Roadmap

- `diary stats`: word count, streaks, and writing cadence
- `diary export`: HTML or PDF export
- Tag extraction from Markdown content
- Optional encrypted archive support
- Homebrew formula or release binaries

## FAQ

### Is dear-diary a cloud journal app?

No. It is a local-first terminal journal. Sync and backup are intentionally delegated to file-system tools such as iCloud Drive, Dropbox, Time Machine, or git.

### Does it replace Vim?

No. It launches your editor and keeps the app focused on date routing, calendar browsing, and search.

### Can I use it outside macOS?

The code is Go and should be portable, but the current product target is macOS terminal usage with Vim-style workflows.

### Where are my private diary entries stored?

Journal entries are not stored in this repository. They are written to `~/Documents/dear-diary/` by default.

## Keywords

terminal diary, command-line journal, CLI journal, Markdown journal, Vim journal, TUI diary, Bubble Tea app, Go CLI, local-first notes, developer journal, personal knowledge management, macOS journal app.

## License

Personal use. No open-source license has been selected yet.
