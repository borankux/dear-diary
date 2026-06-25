# 亲爱的日记 (dear-diary) — 文件管理结构

> 最后更新：2026-06-25

## 项目类型
Go CLI 工具 — 标准布局 `cmd/ + internal/`，单二进制无依赖。

## 排除规则
以下目录/文件不参与整理：
- .git/
- node_modules/
- bin/
- vendor/
- .claude/
- .cursor/
- .github/
- docs/
- log/
- log/summaries/
- log/archive/
- *.md 根级文档（CLAUDE.md/PROJECT.md 等管理系统文件）
- go.sum

## 目录规则

| 路径 | 用途 | 匹配条件 | 命名规范 | 优先级 |
|------|------|----------|----------|--------|
| `cmd/<binary>/` | 二进制入口 main.go | `cmd/*/main.go` | kebab-case 目录名 | 1 |
| `internal/<package>/` | 内部业务包 | `internal/*/*.go`（非 `_test.go`） | kebab-case 包名，对应 Go package 名 | 1 |
| `internal/<package>/` | 内部包测试 | `internal/*/*_test.go` | 同包内 `*_test.go` | 1 |
| `docs/<type>/` | 文档归档 | `docs/{prd,tech-design,research}/*.md` | kebab-case 文件名（双语项目优先英文） | 2 |
| `docs/` | 项目级 spec / 设计文档 | `docs/*.md`（非子目录内） | kebab-case，含日期前缀更佳 | 2 |
| `log/` | 会话日志 | `log/session-YYYY-MM-DD-*.md` | `session-YYYY-MM-DD-{slug}.md` 严格格式 | 3 |
| `log/archive/` | 归档日志 | 旧 session logs | 同上 | 3 |
| `log/summaries/` | 日志压缩摘要 | 累积 10 个 raw log 后压缩 | `summary-level-N-YYYY-MM.md` | 3 |

## 待分类
以下文件尚未归类（下次整理时处理）：
- （暂无）

## 整理历史
| 日期 | 操作 | 文件数 |
|------|------|--------|
| 2026-06-25 | 初始化结构 | 0 |
