# 亲爱的日记 (dear-diary) — Project Wiki

> 最后同步：2026-06-26（自动）

## 一句话定义
Vim-first、本地优先的 macOS 终端日记操作台：`diary` 快速写 Markdown 日记，`diary browse/search` 回看，`diary process` 把日记提炼为待确认 AI 候选，`diary review/todo/dashboard` 形成闭环。

## 当前阶段
v0.4.0 Closure Core 已实现 — AI 输出先进入 pending candidates，人工 review 后才进入 Todo / Memory；Todo 支持 done/archive；搜索、process、dashboard 只认 canonical diary files；dashboard 已改为只读阅读视图，包含 Web 月历入口、单日日记页、最近日记和注意力队列。

## 模块/章节地图

| Package | 职责 | 状态 |
|---------|------|------|
| `cmd/diary/main.go` | CLI 入口、子命令分发、review/todo/process/dashboard 命令 | ✅ v0.4.0 |
| `internal/storage` | 路径计算、文件创建/追加、月份扫描、canonical diary-file filter | ✅ v0.4.0 |
| `internal/editor` | `$EDITOR` 调用，Vim 光标定位 (`+normal Go`) | ✅ v0.1.0 |
| `internal/search` | ripgrep 优先，纯 Go 回退，仅搜索 `YYYY-MM/YYYY-MM-DD.md` | ✅ v0.4.0 |
| `internal/tui` | Bubbletea browse 月历 + search 结果列表 | ✅ v0.2.0 |
| `internal/stats` | CurrentStreak / LongestStreak / TotalWritten | ✅ v0.2.0 |
| `internal/memory` | OnThisDay 查找 X 年前的今天 | ✅ v0.2.0 |
| `internal/process` | LLM provider、SQLite、AI candidates、review promotion、todo lifecycle、dashboard | ✅ v0.4.0 |

## 文件结构
> 详细目录规则见 STRUCTURE.md

```
.
├── README.md                   ← 用户文档（含 Local Mode / AI Mode）
├── PROJECT.md                  ← 本文件（AI 自动同步）
├── session-handoff.md          ← 跨会话接手指引
├── TODO.md                     ← 执行清单
├── UPDATE_LOG.md               ← 版本化更新日志
├── DOCS.md                     ← 文档索引
├── cmd/diary/main.go           ← CLI 入口
├── internal/
│   ├── storage/                ← Markdown diary source boundary
│   ├── editor/                 ← Vim / editor integration
│   ├── search/                 ← diary-only search
│   ├── stats/                  ← streak stats
│   ├── memory/                 ← on-this-day recall
│   ├── process/                ← AI candidates, review, todos, dashboard
│   └── tui/                    ← browse + search TUI
├── docs/
│   ├── prd/main.md             ← v0.4 PRD
│   ├── spec.md                 ← original product + implementation spec
│   └── tech-design/            ← technical design docs
├── log/                        ← session logs
├── bin/                        ← build output（.gitignore）
└── .claude/                    ← candidates + file snapshot
```

## 当前进度快照

| 功能 | 状态 | 版本 | 备注 |
|------|------|------|------|
| 默认 `diary` 进 Vim | ✅ | v0.1.0 | 首次创建 + 同日追加逻辑 |
| TUI 月历浏览 | ✅ | v0.1.0 | hjkl + H/L + t |
| 日期参数解析 | ✅ | v0.1.0 | YYYY-MM-DD / MM-DD / M/D |
| yesterday 子命令 | ✅ | v0.1.0 | 别名 y |
| 全文搜索 | ✅ | v0.4.0 | 仅 canonical diary files，排除 process 输出 |
| streak 连续天数 | ✅ | v0.2.0 | 月历 header + CLI 启动 |
| X 年前的今天回顾 | ✅ | v0.2.0 | 启动时预览，无数据静默 |
| LLM provider boundary | ✅ | v0.4.0 | `DIARY_LLM_*` 优先，`DEEPSEEK_*` 兼容 |
| AI candidate layer | ✅ | v0.4.0 | pending / accepted / rejected |
| `diary review` | ✅ | v0.4.0 | accept / reject / skip / quit |
| `diary todo` lifecycle | ✅ | v0.4.0 | list / done / archive |
| Dashboard read-only reading view | ✅ | v0.4.0 | 今日概览 / Web 月历入口 / 单日日记页 / 最近日记 / 注意力队列 |
| 测试覆盖 | ✅ | v0.4.0 | storage/search/process lifecycle tests |

## 隐私边界

- Local Mode：`diary` / `browse` / `search` / `review` / `todo` / `dashboard` 只读取本地 Markdown 和 SQLite。
- AI Mode：`diary process` 会把待处理日记内容发送给配置的 LLM provider。
- Source of truth：原始日记仍是 `~/Documents/dear-diary/YYYY-MM/YYYY-MM-DD.md`。
- Structured data：`~/.local/share/dear-diary/process.db`。

## 下一步

1. 连续自用 v0.4 Closure Core，观察 30 天闭环是否真的减少悬空状态。
2. 如 review 过慢，再做 `review edit` / `review merge`。
3. 如 provider 切换频繁，再考虑本地模型或 LLM gateway；当前不做。

## 相关链接
- GitHub: https://github.com/borankux/dear-diary
