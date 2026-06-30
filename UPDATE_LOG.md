# Update Log

> 记录项目的重大更新（AI 在 end session 时自动判断是否写入）。

<!-- version-style: semantic -->

## v0.6.1 (2026-06-30)

### Patch: AI Inbox semantics

- 新增 `diary inbox`：默认只显示 AI Inbox 摘要和少量候选，不强迫逐条处理。
- 新增 `diary inbox triage`：显式进入 promote / dismiss / defer / quit 流程。
- 保留 `diary review` 以及 accept / reject 动作为兼容别名。
- Web Dashboard 将候选列改为 `AI Inbox`，按钮文案改为“提升 / 丢弃”，并限制首屏展示数量。
- 远程 CLI 新增 `diary remote promote <id>` / `diary remote dismiss <id>`，旧 accept / reject 保持可用。
- CLI 和 Web health 现在共用同一版本常量，避免 `/health` 返回旧版本。
- 对齐 README / PRD / 项目交接文档，并修复 `web/package-lock.json` 与 `package.json` 不同步导致 `npm ci` 失败的问题。
- 验证：`npm --prefix web ci`、`npm --prefix web run build`、`go test ./...`、`go vet ./...`、`make build`、临时数据库 CLI smoke test。

## v0.4.0 (2026-06-26)

### Minor: Closure Core

- 新增 AI candidate layer：LLM 输出先进入 pending `ai_candidates`
- `diary process` 不再直接写 final todos/memories
- 新增 `diary review`：accept / reject / skip / quit
- 新增 `diary todo`：list / done / archive
- Todo / Memory final records 支持 source evidence 相关字段（additive migration）
- LLM provider 配置改为 `DIARY_LLM_*` 优先，兼容 `DEEPSEEK_*`
- 明确 Local Mode / AI Mode 隐私边界
- Search / process / dashboard 共用 canonical diary-file 规则，排除 `process/` 生成 Markdown
- Dashboard 改为只读阅读视图：今日概览、Web 月历入口、单日日记页、最近日记、注意力队列、长期记忆
- Fatal state transition 写入持久 transition log
- 验证：`go test ./...`、`go vet ./...`、`make build`、`./bin/diary --version`

## v0.3.0 (2026-06-26)

### Minor: AI processing POC + dashboard

- 新增 `diary process`，基于 LLM 提炼日记中的 Todo / Memory
- 新增 SQLite `process.db`：processing runs、file snapshots、todos、memories
- 新增 process state machine 和 transition logs
- 新增 Markdown summaries：`process/todos.md`、`process/memories.md`
- 新增 `diary dashboard`，生成本地 HTML dashboard
- 引入 `internal/process` 包和对应测试

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
