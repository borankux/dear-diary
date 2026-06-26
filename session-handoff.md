# Session Handoff — 亲爱的日记 (dear-diary)

> 最后更新：2026-06-26 v0.4.0 Closure Core

## 项目目标
Vim-first、本地优先的 Markdown 日记 + AI 提炼闭环：原始日记保持纯文本 source of truth，LLM 输出先进入 AI candidates，人工 review 后才进入 Todo / Memory，并能通过 Todo done/archive 关闭行动项。

## 当前状态
- v0.4.0 Closure Core 已实现并通过验证。
- `diary process` 现在写入 pending `ai_candidates`，不再直接写 final todos/memories。
- `diary review` 支持 accept / reject / skip / quit。
- `diary todo` 支持 list / done / archive。
- `diary dashboard` 是只读阅读视图：显示今日概览、Web 月历入口、单日日记页、最近日记、注意力队列和长期记忆，列表限量展示。
- Search / process / dashboard 都使用 canonical diary-file filter：只认 `YYYY-MM/YYYY-MM-DD.md`。
- LLM 配置已 provider-neutral：优先 `DIARY_LLM_*`，兼容 `DEEPSEEK_*`。

## 核心产出文件
| 文件 | 状态 | 版本 | 说明 |
|------|------|------|------|
| cmd/diary/main.go | ✅ | v0.4.0 | CLI + process/review/todo/dashboard |
| internal/storage/ | ✅ | v0.4.0 | canonical diary-file filter |
| internal/search/ | ✅ | v0.4.0 | diary-only search |
| internal/process/store.go | ✅ | v0.4.0 | additive schema migration + candidates + todo lifecycle |
| internal/process/runner.go | ✅ | v0.4.0 | process writes pending candidates |
| internal/process/extractor.go | ✅ | v0.4.0 | provider-neutral OpenAI-compatible extraction |
| internal/process/html.go | ✅ | v0.4.0 | read-only dashboard + generated per-day diary pages, no AI call |
| README.md | ✅ | v0.4.0 | Local Mode / AI Mode / workflow documented |
| docs/prd/main.md | ✅ | v0.4.0 | Closure Core PRD/status |

## Verification
- ✅ `go test ./...`
- ✅ `go vet ./...`
- ✅ `make build`
- ✅ `./bin/diary --version` → `0.4.0`

## 关键设计决策
| # | 决策 | 理由 |
|---|------|------|
| 1 | v0.4 只做 Closure Core，不加 questions/decisions/weekly review | 避免继续堆未闭环功能 |
| 2 | AI output 先写 `ai_candidates` | 防止 AI 误判污染长期 Todo/Memory |
| 3 | rejected candidate 也参与去重 | 防止同一来源/内容反复 resurfacing |
| 4 | 保留 v0.3 todos/memories 表并 additive migrate | 保护现有数据 |
| 5 | DeepSeek 只是兼容配置，产品边界是 OpenAI-compatible LLM provider | 未来可切本地模型/gateway |
| 6 | dashboard 不调用 AI、不写数据 | dashboard 是阅读和判断重点的页面，不是交互工作台 |

## 下一步
1. 用真实日记跑一轮 `diary process`，确认新 candidates 质量。
2. 用 `diary review` 接受/拒绝几条，观察 review 是否太慢。
3. 用 `diary todo` 关闭 active todos，观察 done/archive 是否足够。
4. 如果人工 review 卡住，再做 `review edit`；如果 memory 重复明显，再做轻量 merge。
