# 亲爱的日记 (dear-diary) — Project Wiki

> 最后同步：2026-06-25（自动）

## 一句话定义
Vim-first macOS 终端日记应用 — `diary` 直接进 Vim 写今天，`diary browse` TUI 月历浏览，`diary search` 全文搜索。同一天多次打开自动追加时间戳段落。

## 当前阶段
v0.2.0 已发布 — 基础 MVP + streak 连续天数 + X 年前的今天回顾提醒。

## 模块/章节地图

| Package | 职责 | 状态 |
|---------|------|------|
| `cmd/diary/main.go` | CLI 入口、子命令分发、回顾提醒打印 | ✅ v0.2.0 |
| `internal/storage` | 文件系统抽象：路径计算、文件创建/追加、月份扫描 | ✅ v0.1.0 |
| `internal/editor` | `$EDITOR` 调用，Vim 光标定位 (`+normal Go`) | ✅ v0.1.0 |
| `internal/search` | ripgrep 优先，纯 Go 回退，结果按日期倒序 | ✅ v0.1.0 |
| `internal/tui` | Bubbletea Models：browse 月历 + search 结果列表 | ✅ v0.2.0 (streak in header) |
| `internal/stats` | CurrentStreak / LongestStreak / TotalWritten | ✅ v0.2.0 |
| `internal/memory` | OnThisDay 查找 X 年前的今天 | ✅ v0.2.0 |

## 文件结构
> 详细目录规则见 STRUCTURE.md

```
.
├── CLAUDE.md                   ← 项目宪法（人工确认）
├── PROJECT.md                  ← 本文件（AI 自动同步）
├── STRUCTURE.md                ← 文件管理规则（AI 自动维护）
├── session-handoff.md          ← 接手指引（AI 自动）
├── TODO.md                     ← 执行清单
├── UPDATE_LOG.md               ← 更新日志（重大更新时写入）
├── DOCS.md                     ← 文档索引（AI 自动归档）
├── README.md                   ← 用户文档（双语开源风格）
├── Makefile                    ← build/install/test/run 目标
├── llms.txt                    ← LLM 友好的项目描述
├── go.mod / go.sum             ← Go module: github.com/borankux/dear-diary
├── cmd/
│   └── diary/main.go           ← 入口
├── internal/                   ← 业务包（不对外暴露）
│   ├── storage/                ← 文件系统操作
│   ├── editor/                 ← Vim 集成
│   ├── search/                 ← ripgrep / Go fallback
│   ├── stats/                  ← streak 计算
│   ├── memory/                 ← 回顾提醒
│   └── tui/                    ← Bubbletea 月历 + 搜索
├── docs/                       ← 文档仓库
│   ├── spec.md                 ← 产品 + 实现 spec
│   ├── prd/                    ← 产品需求文档
│   ├── tech-design/            ← 技术设计
│   └── research/               ← 调研
├── log/                        ← 会话日志
├── bin/                        ← 编译产物（.gitignore）
├── .github/                    ← CI 配置
├── .claude/
│   ├── candidates.md           ← 宪法候选池
│   └── .file-snapshot.json     ← 文件整理快照
└── .cursor/rules/              ← Cursor 规则
```

## 关键文件索引

| 文件 | 说明 |
|------|------|
| CLAUDE.md | 项目宪法，定义规则和边界 |
| PROJECT.md | 本文件，项目百科全貌 |
| session-handoff.md | 跨会话接手指引 |
| TODO.md | 执行任务清单 |
| .claude/candidates.md | 待确认的宪法候选条目 |
| STRUCTURE.md | 文件管理规则，定义目录组织和匹配条件 |
| UPDATE_LOG.md | 更新日志，记录重大更新 |
| DOCS.md | 文档索引，记录所有文档的元数据和层级关系 |
| docs/spec.md | 产品和实现 spec |
| README.md | 双语开源 README |

## 当前进度快照

| 功能 | 状态 | 版本 | 备注 |
|------|------|------|------|
| 默认 `diary` 进 Vim | ✅ | v0.1.0 | 首次创建 + 同日追加逻辑 |
| TUI 月历浏览 | ✅ | v0.1.0 | hjkl + H/L + t |
| 日期参数解析 | ✅ | v0.1.0 | YYYY-MM-DD / MM-DD / M/D |
| yesterday 子命令 | ✅ | v0.1.0 | 别名 y |
| 全文搜索 | ✅ | v0.1.0 | rg 优先 / 纯 Go 回退 |
| 🔥 streak 连续天数 | ✅ | v0.2.0 | 月历 header + CLI 启动 |
| 📅 X 年前的今天回顾 | ✅ | v0.2.0 | 启动时预览，无数据静默 |
| 测试覆盖 | ✅ | v0.2.0 | storage/search/stats/memory 22+ 用例 |
| 浏览安装 | ✅ | v0.2.0 | `~/bin/diary` 已部署 |
| 天气/心情自动填表 | ⏳ 待开发 | - | 减少写作阻力 |
| 每日提醒 (launchd) | ⏳ 待开发 | - | terminal-notifier |
| 标签系统 | ⏳ 待开发 | - | `#tag` markdown 解析 |
| 导出 HTML/PDF | ⏳ 待开发 | - | `diary export` |
| 加密支持 | ⏳ 待开发 | - | age 加密 .md |

## 相关链接
- GitHub: https://github.com/borankux/dear-diary
