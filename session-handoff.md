# Session Handoff — 亲爱的日记 (dear-diary)

> 最后更新：2026-06-30 v0.6.1 AI Inbox semantics

## 项目目标
Vim-first、本地优先的 Markdown 日记 + AI 提炼闭环：原始日记保持纯文本 source of truth，LLM 输出先进入 AI Inbox，默认只看摘要，显式 triage 时才把候选提升为 Todo / Memory，并能通过 Todo done/archive/status/priority 关闭行动项。

## 当前状态
- v0.6.1 AI Inbox 语义已实现，已推送 GitHub、安装本机 CLI，并部署到 `https://diary.wakitsoft.com`。
- `diary process` 现在写入 pending `ai_candidates`，不再直接写 final todos/memories。
- `diary inbox` 默认显示 AI Inbox 摘要，不强迫逐条处理。
- `diary inbox triage` 支持 promote / dismiss / defer / quit；旧 `diary review` 仍可用作兼容别名。
- `diary todo` 支持 list / done / archive。
- `diary dashboard` 是只读阅读视图：显示今日概览、Web 月历入口、单日日记页、最近日记、注意力队列和长期记忆，列表限量展示。
- Search / process / dashboard 都使用 canonical diary-file filter：只认 `YYYY-MM/YYYY-MM-DD.md`。
- LLM 配置已 provider-neutral：优先 `DIARY_LLM_*`，兼容 `DEEPSEEK_*`。

## 核心产出文件
| 文件 | 状态 | 版本 | 说明 |
|------|------|------|------|
| cmd/diary/main.go | ✅ | v0.6.1 | CLI + process/inbox/todo/dashboard |
| internal/storage/ | ✅ | v0.4.0 | canonical diary-file filter |
| internal/search/ | ✅ | v0.4.0 | diary-only search |
| internal/process/store.go | ✅ | v0.4.0 | additive schema migration + candidates + todo lifecycle |
| internal/process/runner.go | ✅ | v0.4.0 | process writes pending candidates |
| internal/process/extractor.go | ✅ | v0.4.0 | provider-neutral OpenAI-compatible extraction |
| internal/process/html.go | ✅ | v0.4.0 | read-only dashboard + generated per-day diary pages, no AI call |
| internal/version/ | ✅ | v0.6.1 | shared CLI / server version constants |
| README.md | ✅ | v0.6.1 | AI Inbox workflow documented |
| docs/prd/main.md | ✅ | v0.6.1 | AI Inbox PRD/status |

## Verification
- ✅ `go test ./...`
- ✅ `go vet ./...`
- ✅ `npm --prefix web ci`
- ✅ `npm --prefix web run build`
- ✅ `make build`
- ✅ `diary --version` → `0.6.1`
- ✅ `https://diary.wakitsoft.com/health` → `0.6.1-server`
- ✅ Browser smoke: desktop/mobile dashboard renders `AI Inbox` / `提升` / `丢弃`, no console/API errors, no horizontal overflow.

## 关键设计决策
| # | 决策 | 理由 |
|---|------|------|
| 1 | v0.4 只做 Closure Core，不加 questions/decisions/weekly review | 避免继续堆未闭环功能 |
| 2 | AI output 先写 `ai_candidates` / AI Inbox | 防止 AI 误判污染长期 Todo/Memory |
| 3 | dismissed/rejected candidate 也参与去重 | 防止同一来源/内容反复 resurfacing |
| 4 | 保留 v0.3 todos/memories 表并 additive migrate | 保护现有数据 |
| 5 | DeepSeek 只是兼容配置，产品边界是 OpenAI-compatible LLM provider | 未来可切本地模型/gateway |
| 6 | dashboard 不调用 AI、不写数据 | dashboard 是阅读和判断重点的页面，不是交互工作台 |

## 下一步
1. 用真实日记跑一轮 `diary process`，确认新 candidates 质量。
2. 用 `diary inbox` 看摘要，再用 `diary inbox triage` 提升/丢弃几条，观察 triage 是否太慢。
3. 用 `diary todo` 关闭 active todos，观察 done/archive 是否足够。
4. 如果人工 triage 卡住，再做 `inbox edit`；如果 memory 重复明显，再做轻量 merge。
