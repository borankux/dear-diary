# 亲爱的日记 (dear-diary) — TODO

> 每条任务必须包含：负责人 / 截止时间 / 依赖项
> 完成的任务勾选保留（不删除），作为执行历史

## 当前重点：v0.6 AI Inbox 真实使用验证
- [ ] 用真实日记连续跑 `diary process`，评估 pending candidates 质量
  Owner: Frank | Deadline: 待定 | Dependencies: v0.4.0
- [ ] 用 `diary inbox` 看摘要，再用 `diary inbox triage` 提升/丢弃至少一轮候选，记录 triage 是否过慢
  Owner: Frank | Deadline: 待定 | Dependencies: pending candidates
- [ ] 用 `diary todo done/archive` 关闭一批 active todos，验证 Todo 闭环是否足够
  Owner: Frank | Deadline: 待定 | Dependencies: promoted todos
- [ ] 观察 dashboard 是否能清楚暴露 pending candidates / active todos / recent memories
  Owner: Frank | Deadline: 待定 | Dependencies: v0.4.0 dashboard

## 后续候选（只有真实使用证明需要时再做）
- [ ] `diary inbox edit`：提升前编辑 candidate 内容
  Owner: Frank | Deadline: 待定 | Dependencies: inbox triage 真实使用反馈
- [ ] `diary inbox merge`：把 candidate 合并到已有 Todo / Memory
  Owner: Frank | Deadline: 待定 | Dependencies: 出现明显重复 memory/todo
- [ ] `diary memory`：只读查看 recent memories
  Owner: Frank | Deadline: 待定 | Dependencies: Memory 使用量增长

## 暂缓（非 v0.4）
- [ ] 本地模型或 LLM gateway
  Owner: Frank | Deadline: 暂缓 | Dependencies: provider 切换需求明确
- [ ] questions / decisions 独立表
  Owner: Frank | Deadline: 暂缓 | Dependencies: Todo/Memory 闭环稳定
- [ ] weekly review
  Owner: Frank | Deadline: 暂缓 | Dependencies: 30 天数据积累
- [ ] 天气 / 心情 / 提醒 / 标签 / 导出
  Owner: Frank | Deadline: 暂缓 | Dependencies: Closure Core 稳定

## 已完成
- [x] v0.1.0 基础 MVP（diary/browse/search/yesterday + TUI 月历 + 同日追加）
  Owner: Claude + Frank | Deadline: 2026-06-25 | Dependencies: 无
- [x] v0.2.0 streak 连续天数（月历 header + CLI 启动）
  Owner: Claude + Frank | Deadline: 2026-06-25 | Dependencies: v0.1.0
- [x] v0.2.0 X 年前的今天回顾提醒（无数据静默）
  Owner: Claude + Frank | Deadline: 2026-06-25 | Dependencies: v0.1.0
- [x] v0.3.0 AI processing POC（process.db + todo/memory summaries + dashboard）
  Owner: Claude + Frank | Deadline: 2026-06-26 | Dependencies: v0.2.0
- [x] v0.4.0 Closure Core（AI candidates + review + todo lifecycle + diary-only filtering）
  Owner: Codex | Deadline: 2026-06-26 | Dependencies: v0.3.0
- [x] Dashboard read-only readability pass（今日概览 + Web 月历入口 + 单日日记页 + 注意力队列）
  Owner: Codex | Deadline: 2026-06-26 | Dependencies: v0.4.0 dashboard
- [x] v0.6.1 AI Inbox 语义修正（summary first + triage promote/dismiss + review compatibility）
  Owner: Codex | Deadline: 2026-06-30 | Dependencies: v0.6.0 remote dashboard
- [x] 项目管理系统初始化（CLAUDE.md / PROJECT.md / STRUCTURE.md 等）
  Owner: Claude + Frank | Deadline: 2026-06-25 | Dependencies: 无
