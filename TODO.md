# 亲爱的日记 (dear-diary) — TODO

> 每条任务必须包含：负责人 / 截止时间 / 依赖项
> 完成的任务勾选保留（不删除），作为执行历史

## 第二档功能（减少写作阻力）
- [ ] 天气自动填表（wttr.in 调用 + 模板插入 `> 杭州 18°C 晴`）
  Owner: Frank | Deadline: 待定 | Dependencies: 无
- [ ] 心情 emoji 自动填表（光标选 😐 → 😀 → 😢 等）
  Owner: Frank | Deadline: 待定 | Dependencies: 无
- [ ] 每日提醒（`diary remind 22:00` 注册 launchd，到点 terminal-notifier）
  Owner: Frank | Deadline: 待定 | Dependencies: 无

## 第三档功能（长期价值）
- [ ] 标签系统（`#tag` 解析 + `diary search --tag 工作`）
  Owner: Frank | Deadline: 待定 | Dependencies: 无
- [ ] AI 周报/月报（`diary recap week` 调 Claude/GPT 总结）
  Owner: Frank | Deadline: 待定 | Dependencies: API key
- [ ] 导出 HTML/PDF（`diary export 2026-06`）
  Owner: Frank | Deadline: 待定 | Dependencies: 无

## 长期/可选
- [ ] 加密支持（age 加密 .md 文件）
  Owner: Frank | Deadline: 待定 | Dependencies: 无
- [ ] Homebrew formula 或 GitHub Release 二进制
  Owner: Frank | Deadline: 待定 | Dependencies: GitHub repo 公开
- [ ] 图片/附件支持（Markdown 内嵌图片，Vim 集成）
  Owner: Frank | Deadline: 待定 | Dependencies: 无

## 已完成（v0.2.0 及之前）
- [x] v0.1.0 基础 MVP（diary/browse/search/yesterday + TUI 月历 + 同日追加）
  Owner: Claude + Frank | Deadline: 2026-06-25 | Dependencies: 无
- [x] v0.2.0 streak 连续天数（月历 header + CLI 启动）
  Owner: Claude + Frank | Deadline: 2026-06-25 | Dependencies: v0.1.0
- [x] v0.2.0 X 年前的今天回顾提醒（无数据静默）
  Owner: Claude + Frank | Deadline: 2026-06-25 | Dependencies: v0.1.0
- [x] 项目管理系统初始化（CLAUDE.md / PROJECT.md / STRUCTURE.md 等）
  Owner: Claude + Frank | Deadline: 2026-06-25 | Dependencies: 无
