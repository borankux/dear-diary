# 亲爱的日记 (dear-diary) 项目指令

> 本文件由 Codex 自动加载，定义项目协作规则。

## 项目概况
- **产品：** Vim-first macOS 终端日记应用 — TUI 月历浏览、同一天追加时间戳段落、全文搜索
- **当前阶段：** v0.2.0 已发布（基础 MVP + streak 连续天数 + X 年前的今天回顾）
- **GitHub：** borankux/dear-diary

## Language / 语言
- **Language:** bilingual

## 项目管理系统

本项目使用 6 组件管理系统。

- **Log Compaction Threshold:** 10（每积累 10 个日志文件压缩为 1 个 summary）

### 触发词

| Intent | AI Action |
|--------|-----------|
| End session / wrap up — any expression of "we're done for now" (end session, 结束会话, 收工, wrap up, done for today, etc.) | Write log + update handoff + sync Wiki + check TODO + collect constitution candidates + file reorganization + document archiving + evaluate update log + version bump + output summary in configured language |
| Review constitution — any expression of "check/update rules" (review Codex, 更新宪法, check rules, etc.) | Show .Codex/candidates.md for confirmation one by one |
| Sync wiki — any expression of "update project overview" (sync wiki, 同步项目, refresh overview, etc.) | Force rescan and update PROJECT.md |
| Check status — any expression of "what's the current state" (status, 项目现状, where are we, etc.) | Read PROJECT.md + session-handoff.md summary aloud |
| Organize files — any expression of "clean up files" (organize files, 整理文件, clean up, sort files, etc.) | Scan project files, organize per STRUCTURE.md rules |
| Change language — any expression of "switch language" (切换语言, change language, switch to English, 换成中文, etc.) | Execute Language Change Protocol |
| Continue — any expression of "pick up where we left off" (接着上次, continue, 上次做到哪了, etc.) | Read last session log + session-handoff.md + PROJECT.md to recover context |
| Continue full context — any expression of "full project review" (全面回顾, full context, 项目全景, etc.) | Full project trajectory recovery across all sessions |

### 文件职责

| File | Who writes | When |
|------|-----------|------|
| AGENTS.md | 人工确认 | review Codex 时 |
| PROJECT.md | AI 自动 | end session + 文件结构变化时 |
| session-handoff.md | AI 自动 | end session 时 |
| TODO.md | AI + 人 | 随时 |
| log/session-*.md | AI | end session 时 |
| .Codex/candidates.md | AI 自动 | 过程中识别到稳定规则时 |
| STRUCTURE.md | AI 自动 | end session + 文件结构变化时 |
| .Codex/.file-snapshot.json | AI 自动 | end session 时 |
| UPDATE_LOG.md | AI 自动 | end session + 重大更新时 |
| DOCS.md | AI 自动 | end session + 文档归档时 |

### Session Start Protocol

At session start:

1. Read `PROJECT.md` for project overview and `session-handoff.md` for current progress / next steps. Check the Language setting in AGENTS.md to determine output language.
2. **Read logs (bounded):**
   - Find the highest level with summaries in `log/summaries/` — read all summaries at that level.
   - Read all unarchived raw logs in `log/` (exclude `summaries/` and `archive/`).
   - Total files read: at most 2 × (threshold − 1), regardless of project age.
   - If `log/` doesn't exist yet, skip this step.

### Session End Protocol

当用户说 "end session" / "结束会话" / "收工" 时，按顺序执行：

1. **写会话日志** → `log/session-YYYY-MM-DD-{主题slug}.md`
   - 同日多次会话用 slug 区分（如 `session-2026-04-21-prd-draft.md`）
2. **Log Compaction** → 检查未归档 raw logs 数量，若 ≥ threshold 则执行压缩（见 Log Compaction Protocol）
3. **更新 session-handoff.md** → 刷新"当前进度 / 下一步"
4. **更新 PROJECT.md** → 如有结构/模块状态变化，同步更新
5. **更新 TODO.md** → 标记本次已完成的任务
6. **收集宪法候选** → 识别本次会话中的规则/偏好/边界，追加到 `.Codex/candidates.md`
7. **整理文件结构（增量模式）** → 只处理新增/变更文件，按 STRUCTURE.md 规则快速归类
   - 若 STRUCTURE.md 不存在：先建立规则表（深度模式），再整理
   - 若 STRUCTURE.md 已存在：只匹配新增文件，不重读已有文件
   - 更新 `.Codex/.file-snapshot.json`
7.5. **文档归档** → read `references/document-archiving.md`，扫描本次会话产出的文档
   - 识别并分类文档（PRD / 技术设计 / 设计文档 / 调研 / 会议纪要 / 实验记录）
   - 归档到 `docs/` 对应子目录 + 更新 `DOCS.md` 索引元数据
   - 若 DOCS.md 不存在：创建（升级兼容）
8. **评估并写入 Update Log** → 评估本次会话是否包含重大更新（新功能、重大修改、3+ 文件变更、用户声明里程碑、重要 TODO 完成）
   - 若是重大更新：判断版本递增级别（major/minor/patch），计算新版本号，在 `UPDATE_LOG.md` 顶部追加版本化条目，可选创建 GitHub Release
   - 若不是：静默跳过
9. **Output summary** → A brief summary of what was done this session, in the configured language

### Session Log Format

Session log headers adapt to the configured language. Use the Session Log entries from the Key Terms Glossary.

写入 `log/` 的每条日志遵循以下格式：

```markdown
# Session YYYY-MM-DD — {topic}

## Session Goal
## Key Actions (Chronological)
## Decisions & Rationale
## Output Files
## Unfinished Items / Next Session Pickup
## AGENTS.md Candidates (if any)
```

### Constitution Candidate Rules

AI 在工作过程中，遇到以下情况时自动追加条目到 `.Codex/candidates.md`：
- 用户明确说"以后都这么做" / "这是规则" / "不要再…"
- 同一类决策在多次会话中连续出现
- 涉及命名规范、文件分层、协作流程的决定
- 涉及技术栈选择、架构约束的决定

**绝对不要直接修改 AGENTS.md。** 所有候选条目必须经用户 review 后才写入。

### TODO Format

TODO.md 中每条任务必须包含三要素：
```
- [ ] {task description}
  Owner: {name} | Deadline: {date} | Dependencies: {prerequisite}
```
If a user provides a task missing required fields, ask them to fill in. Completed tasks are checked and kept (not deleted).

## Coding Guidelines

Behavioral guidelines to reduce common LLM coding mistakes, derived from [Andrej Karpathy's observations](https://x.com/karpathy/status/2015883857489522876) on LLM coding pitfalls.

**Tradeoff:** These guidelines bias toward caution over speed. For trivial tasks, use judgment.

### 1. Think Before Coding

**Don't assume. Don't hide confusion. Surface tradeoffs.**

Before implementing:
- State your assumptions explicitly. If uncertain, ask.
- If multiple interpretations exist, present them - don't pick silently.
- If a simpler approach exists, say so. Push back when warranted.
- If something is unclear, stop. Name what's confusing. Ask.

### 2. Simplicity First

**Minimum code that solves the problem. Nothing speculative.**

- No features beyond what was asked.
- No abstractions for single-use code.
- No "flexibility" or "configurability" that wasn't requested.
- No error handling for impossible scenarios.
- If you write 200 lines and it could be 50, rewrite it.

Ask yourself: "Would a senior engineer say this is overcomplicated?" If yes, simplify.

### 3. Surgical Changes

**Touch only what you must. Clean up only your own mess.**

When editing existing code:
- Don't "improve" adjacent code, comments, or formatting.
- Don't refactor things that aren't broken.
- Match existing style, even if you'd do it differently.
- If you notice unrelated dead code, mention it - don't delete it.

When your changes create orphans:
- Remove imports/variables/functions that YOUR changes made unused.
- Don't remove pre-existing dead code unless asked.

The test: Every changed line should trace directly to the user's request.

### 4. Goal-Driven Execution

**Define success criteria. Loop until verified.**

Transform tasks into verifiable goals:
- "Add validation" → "Write tests for invalid inputs, then make them pass"
- "Fix the bug" → "Write a test that reproduces it, then make it pass"
- "Refactor X" → "Ensure tests pass before and after"

For multi-step tasks, state a brief plan:
```
1. [Step] → verify: [check]
2. [Step] → verify: [check]
3. [Step] → verify: [check]
```

Strong success criteria let you loop independently. Weak criteria ("make it work") require constant clarification.

## Project-Specific Rules

- **技术栈**: Go 1.26+ + Bubble Tea + Lipgloss. 单二进制，零运行时依赖（除 Vim）
- **存储约定**: `~/Documents/dear-diary/YYYY-MM/YYYY-MM-DD.md`，纯 Markdown，文件名是唯一真相来源
- **YAGNI 严格**: 不做加密、云同步、数据库、配置文件 — 让 macOS/iCloud/git 处理
- **测试要求**: 每个新 package 必须有单元测试；改动 storage/search/stats/memory 后必须跑 `go test ./...`
- **构建安装**: `make build` 出 `bin/diary`，`cp bin/diary ~/bin/diary` 全局安装（`~/bin` 在 PATH）
- **导入路径**: `github.com/borankux/dear-diary/...`，go.mod module name 已锁定
- **Markdown 文件格式约定**: `# YYYY-MM-DD 周X\n\n## HH:MM\n\n正文`；同日追加 `\n\n## HH:MM\n\n`；5 分钟内不重复追加
- **编辑器优先级**: `$DIARY_EDITOR` > `$EDITOR` > `vim`，Vim 系列自动追加光标定位 `+normal Go`
- **TUI 月历**: 周一开头，`◆` 今天，`●` 已写日记，光标反色；`[/]` 切月，`t` 跳今天
- **跨会话协作偏好**: Frank 不喜欢逐条问"OK 吗"，应当沿着推荐方向一次做完一个可演示版本
