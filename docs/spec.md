# 亲爱的日记 — 设计 Spec

**版本**: 0.1.0 · **日期**: 2026-06-25

## 目标

一个 macOS 终端的 TUI 日记应用：直接进 Vim 写，月历浏览，同一天多次写不覆盖。

## 核心需求

1. **全局命令 `diary`** — 在任何地方敲一下直接进 Vim 写今天的日记
2. **TUI 月历浏览** — `diary browse` 进月历，看哪天写了哪天没写
3. **同一天多次写** — 自动追加时间戳段落，不覆盖之前内容
4. **按日期打开** — `diary 2026-06-24` / `diary 6/24` / `diary yesterday`
5. **搜索** — `diary search <关键词>`，结果列表 TUI

## 技术栈

- Go 1.26+
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) — TUI 框架
- [Lipgloss](https://github.com/charmbracelet/lipgloss) — 样式
- 编译为单二进制，无运行时依赖（Vim 除外）

## 架构

```
cmd/diary/main.go              # CLI 入口 + 子命令分发
internal/storage/               # 文件系统抽象（路径、追加、扫描）
internal/editor/                # $EDITOR 调用（vim 光标定位）
internal/search/                # ripgrep / 纯 Go 回退
internal/tui/                   # Bubbletea Models
  browse.go                     # 月历视图
  search.go                     # 搜索结果视图
  helpers.go                    # 终端宽度 / ANSI / 字符宽度
```

## 文件格式

路径：`~/Documents/dear-diary/YYYY-MM/YYYY-MM-DD.md`

内容：
```markdown
# YYYY-MM-DD 周X

## HH:MM

正文……

## HH:MM

再次打开后追加的段落……
```

## 子命令

| 命令 | 行为 |
|------|------|
| `diary` | 今天（默认） |
| `diary browse` | TUI 月历 |
| `diary today` | 今天（显式） |
| `diary yesterday` / `diary y` | 昨天 |
| `diary <date>` | 指定日期，支持 `2026-06-24` / `06-24` / `6/24` |
| `diary search <kw>` | 搜索 |
| `diary -h` / `--help` | 帮助 |
| `diary -v` / `--version` | 版本 |

## TUI 月历（browse）

- 月历开头：**周一**（中国习惯）
- 视觉：`◆` 今天，`●` 已写日记的天
- 键位：`hjkl` 移动 · `H/L` 切月 · `t` 跳今天 · `g/G` 月初/末 · `Enter` 打开 · `q` 退出
- 月份切换时重新扫描目录（毫秒级，不缓存）
- 终端 resize 自动重绘

## 编辑器集成

- 优先级：`$DIARY_EDITOR` > `$EDITOR` > `vim`
- Vim 光标定位：`vim '+normal Go' path` → 跳到末行 + 向下开新行进入插入模式
- TUI 模式用 `tea.ExecProcess` 让 bubbletea 释放 raw mode 给 Vim
- CLI 模式直接 `exec.Command.Run()`

## 追加逻辑

**首次创建**：写入 `# YYYY-MM-DD 周X` + 空行 + `## HH:MM` + 空行

**同一天再次打开**：
- 读取文件最后非空行
- 如果是 `## HH:MM` 形式且与当前时间相差 < 5 分钟 → 跳过追加（防误触）
- 否则追加 `\n\n## HH:MM\n\n`

**显式非今天**（如 `diary yesterday`）：不追加，直接打开

## 搜索

- 优先 ripgrep（`rg` 已安装时）
- 回退纯 Go 实现（`filepath.Walk` + `bufio.Scanner`）
- 大小写不敏感
- 结果按日期倒序

## 错误处理

- 文件系统错误（权限、磁盘满）→ stderr 报错 + 退出 1
- 编辑器异常退出 → stderr 报错 + 退出 1
- 不识别的命令/日期 → stderr 提示正确用法 + 退出 2
- Ctrl-C → 让 Vim / bubbletea 自然处理

## 测试

`internal/storage/` 单元测试：
- 路径计算（跨月）
- 文件创建幂等
- 时间戳追加
- 月份扫描
- 文件列表排序

`internal/search/` 单元测试：
- 关键词匹配
- 大小写不敏感
- 日期倒序
- 空关键词安全

## 不做的（YAGNI）

- 加密、多用户、云同步（让 macOS / iCloud 处理）
- 内嵌编辑器（Vim 才是想要的）
- 心情 / 天气 metadata（MVP 不做）
- 数据库、配置文件（约定优于配置）

## 未来扩展（非 MVP）

- `diary stats` — 写作统计（字数、坚持天数）
- `diary export <format>` — 导出 HTML / PDF
- 标签系统 `#tag`
- 心情 / 天气自动填表
- 加密支持（age / GPG）
