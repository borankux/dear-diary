# Session 2026-06-25 — Init + v0.1/v0.2 MVP + 项目记忆栈

## Session Goal

从 0 构建 `dear-diary`：一个 Vim-first 的 macOS 终端日记应用。需要全局命令 `diary` 直接进 Vim 写今天，TUI 月历浏览，同一天多次打开自动追加时间戳段落。

## Key Actions (Chronological)

### Phase 1: Brainstorming
- 调用 `superpowers:brainstorming` 走完设计流程
- 关键决策（逐个 AskUserQuestion 确认）：
  - 技术栈：**Go + Bubbletea**（vs Python+Textual / Node+Ink / 纯 Shell）
  - 存储：`~/Documents/dear-diary/`（iCloud 同步）
  - 追加格式：带时间戳分段（`## HH:MM` 段落）
  - 文件格式：Markdown
  - TUI 主界面：月历视图
  - 命令名：`diary`
  - 默认行为：直接打开今天 Vim
  - 子命令集：`browse` / `<date>` / `yesterday` / `search`（全选）
  - 加密：不做
  - 目录组织：按年月分组 `YYYY-MM/`
- 提 3 个总体方向，选定 **A. 月历 TUI + 系统 Vim**

### Phase 2: v0.1.0 基础 MVP（中断后加速）
- Frank 中断 brainstorming 后段："别问我行不行，直接做完一个版本给我看"
- 一次性写完所有代码：`cmd/diary/main.go` + 4 个 internal 包（storage / editor / search / tui）
- 单元测试：storage / search 共 13 个用例全过
- 编译为单二进制（4.9MB arm64），部署到 `~/bin/diary`（PATH 已包含）
- 写 Makefile / README / .gitignore / docs/spec.md
- git commit `8d37bd3`

### Phase 3: v0.2.0 streak + 回顾提醒
- Frank 改了 import path 为 `github.com/borankux/dear-diary/...`（准备开源）
- Frank 自己重写 README 为双语开源风格
- 实现：
  - `internal/stats/streak.go`：CurrentStreak（温和口径，今天没写从昨天算）+ LongestStreak + TotalWritten
  - `internal/memory/onthisday.go`：OnThisDay（X 年前的今天，跳过纯模板无内容）
- 集成：
  - `diary` 默认行为打开 Vim 前打印 🔥 streak + 📅 历史同日预览（无数据静默）
  - `diary browse` 月历标题加 `🔥 N 天 · 12/30 天`
- 测试 stats + memory 共 9 个用例
- git commit v0.2.0

### Phase 4: 项目记忆栈初始化
- Frank 调用 `/project-butler`
- 只问 2 个真正模糊的问题（Cursor 规则 + 文档类型），其他用已知信息默认
- 创建 11 个文件：CLAUDE.md / PROJECT.md / STRUCTURE.md / session-handoff.md / TODO.md / UPDATE_LOG.md / DOCS.md / .claude/candidates.md / .claude/.file-snapshot.json / .cursor/rules/project-system.mdc / docs/{prd,tech-design}/main.md
- 都填了实际内容（不是空模板），CLAUDE.md 包含项目特定规则 + Karpathy Guidelines + 10 项关键设计决策

## Decisions & Rationale

| # | 决策 | 理由 |
|---|------|------|
| 1 | Go + Bubbletea | 单二进制、启动零延迟、TUI 框架成熟 |
| 2 | 文件名是唯一真相来源 | 避免解析 markdown 内容推断日期的脆弱性 |
| 3 | 路径 `YYYY-MM/YYYY-MM-DD.md` | iCloud 同步 + 按月分组直观 |
| 4 | 不加密、不云同步、不数据库 | YAGNI；让 macOS FileVault + iCloud ADP 处理 |
| 5 | 5 分钟内不重复追加时间戳 | 防误触产生空段落 |
| 6 | streak 温和口径 | 不打 "0 天" 打击信心 |
| 7 | 周一作为月历开头 | 中国习惯 |
| 8 | 不引入 cobra CLI 框架 | YAGNI；标准库 + 手写 dispatch 足够 |
| 9 | `~/bin/diary` 部署 | PATH 已包含，无需 sudo |
| 10 | bilingual 项目语言 | 准备开源到 borankux/dear-diary |

## Output Files

**代码**：
- `cmd/diary/main.go`
- `internal/storage/storage.go` + `_test.go`
- `internal/editor/editor.go`
- `internal/search/search.go` + `_test.go`
- `internal/tui/browse.go` + `search.go` + `helpers.go`
- `internal/stats/streak.go` + `_test.go`
- `internal/memory/onthisday.go` + `_test.go`

**配置 / 文档**：
- `go.mod` (module: `github.com/borankux/dear-diary`)
- `Makefile` / `.gitignore` / `README.md`（Frank 自己改成双语风格）
- `docs/spec.md` / `docs/prd/main.md` / `docs/tech-design/main.md`

**项目记忆栈**（11 个文件）：
- `CLAUDE.md` / `PROJECT.md` / `STRUCTURE.md` / `session-handoff.md`
- `TODO.md` / `UPDATE_LOG.md` / `DOCS.md`
- `.claude/candidates.md` / `.claude/.file-snapshot.json`
- `.cursor/rules/project-system.mdc`
- `log/.gitkeep` + `docs/{prd,tech-design,research}/.gitkeep`

**部署**：
- `~/bin/diary` v0.2.0（全局可用，MD5 与项目 bin/diary 一致）

## Unfinished Items / Next Session Pickup

按优先级（来自 TODO.md）：

**第二档（减少写作阻力）**：
- [ ] 天气自动填表（wttr.in）
- [ ] 心情 emoji 自动填表
- [ ] 每日提醒（launchd + terminal-notifier）

**第三档（长期价值）**：
- [ ] 标签系统（`#tag` 解析 + `diary search --tag`）
- [ ] AI 周报/月报（`diary recap week`）
- [ ] 导出 HTML/PDF（`diary export 2026-06`）

**长期/可选**：
- [ ] 加密（age）
- [ ] Homebrew formula
- [ ] 图片/附件支持

详见 `TODO.md` 和 `session-handoff.md`。

## CLAUDE.md Candidates

已收集到 `.claude/candidates.md`：
1. **Frank 不喜欢逐条问"OK 吗"** — 已临时写入 CLAUDE.md `跨会话协作偏好`，待 review 后正式保留
2. **bilingual 项目语言设置** — Frank 自己改 README 双语开源风格
3. **`github.com/borankux/dear-diary` 仓库路径** — Frank 改了所有 import path

下次触发 `review claude` 时逐条确认是否正式写入 CLAUDE.md。

## Verification

- ✅ `go build ./...` exit 0
- ✅ `go test ./...` 22+ 用例全过
- ✅ `go vet ./...` 无问题
- ✅ `~/bin/diary -v` → `0.2.0`
- ✅ fake editor 测试 streak + 回顾提醒 输出正确
- ✅ 项目记忆栈所有 11 个文件已创建并填充实际内容
- ✅ git 状态干净，2 个 commit 已提交（8d37bd3 v0.1.0 / v0.2.0 commit hash）
