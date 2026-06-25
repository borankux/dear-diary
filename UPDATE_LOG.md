# Update Log

> 记录项目的重大更新（AI 在 end session 时自动判断是否写入）。

<!-- version-style: semantic -->

## v0.2.0 (2026-06-25)

### Minor: streak + 回顾提醒

- 新增 `internal/stats`：CurrentStreak / LongestStreak / TotalWritten
- 新增 `internal/memory`：OnThisDay 查找 X 年前的今天
- `diary` 默认行为打开 Vim 前打印回顾（🔥 连续 N 天 + 📅 历史同日预览）
- `diary browse` 月历标题加 `🔥 N 天` 显示
- streak 温和口径：今天没写时从昨天算，不打 "0 天"
- 回顾最多查 10 年前，跳过纯模板无内容的日记
- 无数据时静默不输出
- 单元测试：stats / memory 包 9 个用例

## v0.1.0 (2026-06-25)

### Minor: 初始发布

- 全局命令 `diary` 直接进 Vim 写今天
- TUI 月历浏览（Bubble Tea）：hjkl 移动 / H/L 切月 / t 跳今天
- 同一天多次打开自动追加时间戳段落（5 分钟内不重复）
- 子命令：`today` / `yesterday` (`y`) / `<date>` / `browse` / `search`
- 日期格式：`2026-06-24` / `06-24` / `6/24`
- 全文搜索：ripgrep 优先，纯 Go 回退，结果按日期倒序
- 文件存储：`~/Documents/dear-diary/YYYY-MM/YYYY-MM-DD.md`
- 编辑器优先级：`$DIARY_EDITOR` > `$EDITOR` > `vim`
- Vim 自动追加光标定位（`+normal Go`）
- 单元测试：storage / search 包 13 个用例
- 技术栈：Go 1.26 + Bubble Tea + Lipgloss，单二进制无依赖

---
