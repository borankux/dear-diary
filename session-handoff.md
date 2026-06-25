# Session Handoff — 亲爱的日记 (dear-diary)

> 最后更新：2026-06-25 v0.2.0

## 项目目标
Vim-first macOS 终端日记应用：TUI 月历浏览 + 同一天追加时间戳 + 全文搜索。

## 核心产出文件
| 文件 | 状态 | 版本 | 说明 |
|------|------|------|------|
| cmd/diary/main.go | ✅ | v0.2.0 | CLI 入口 + 子命令分发 + 回顾打印 |
| internal/storage/ | ✅ | v0.1.0 | 路径/创建/追加/扫描 |
| internal/editor/ | ✅ | v0.1.0 | $EDITOR 调用 + Vim 光标定位 |
| internal/search/ | ✅ | v0.1.0 | rg/Go fallback + 日期倒序 |
| internal/tui/ | ✅ | v0.2.0 | 月历（含 streak）+ 搜索结果 |
| internal/stats/ | ✅ | v0.2.0 | streak 计算 |
| internal/memory/ | ✅ | v0.2.0 | X 年前的今天 |
| docs/spec.md | ✅ | v0.1.0 | 产品和实现 spec |
| ~/bin/diary | ✅ | v0.2.0 | 已全局部署 |

## 当前进度
- v0.2.0 已发布（基础 MVP + streak + 回顾提醒）
- 项目管理系统初始化完成
- 测试覆盖：storage / search / stats / memory 4 个包 22+ 用例全过
- 全局命令已部署到 `~/bin/diary`（在 PATH 里）
- 用户已开始实际使用（2026-06-24 补写昨天，2026-06-25 写今天）

## 关键设计决策
| # | 决策 | 理由 | 日期 |
|---|------|------|------|
| 1 | Go + Bubbletea + Lipgloss | 单二进制、启动快、TUI 成熟 | 2026-06-25 |
| 2 | 文件名是唯一真相，不解析内容推断日期 | 避免 markdown 解析脆弱性 | 2026-06-25 |
| 3 | 路径 `~/Documents/dear-diary/YYYY-MM/YYYY-MM-DD.md` | iCloud 自动同步、按月分组直观 | 2026-06-25 |
| 4 | 不加密、不云同步、不数据库 | YAGNI；让 macOS FileVault + iCloud ADP 处理 | 2026-06-25 |
| 5 | 5 分钟内不重复追加时间戳段落 | 防误触产生空段落 | 2026-06-25 |
| 6 | streak 温和口径（今天没写从昨天算） | 不打 "0 天" 打击信心，养成习惯友好 | 2026-06-25 |
| 7 | 周一作为月历开头 | 中国习惯 | 2026-06-25 |
| 8 | 不引入 cobra 等 CLI 框架 | YAGNI；标准库 flag + 手写 dispatch 足够 | 2026-06-25 |
| 9 | 采用 6 组件项目管理系统 | Log + Wiki + Structure + Constitution + TODO + Docs | 2026-06-25 |
| 10 | 项目语言：bilingual | 开源准备，README 已双语 | 2026-06-25 |

## 迭代历史
| 版本 | 日期 | 变更 |
|------|------|------|
| v0.1.0 | 2026-06-25 | 基础 MVP — diary/browse/search/yesterday + TUI 月历 + 同日追加 |
| v0.2.0 | 2026-06-25 | streak 连续天数 + X 年前的今天回顾 |

## 下一步
- [ ] 第二档功能：天气自动填表（wttr.in）+ 心情 emoji
- [ ] 第二档功能：每日提醒（`diary remind 22:00` → launchd + terminal-notifier）
- [ ] 第三档功能：标签系统（`#tag` 解析 + `diary search --tag`）
- [ ] 第三档功能：导出 HTML/PDF（`diary export 2026-06`）
- [ ] 长期：加密（age）+ Homebrew formula
- [ ] 完善 docs/prd/main.md（产品需求文档）
- [ ] 完善 docs/tech-design/main.md（技术设计总览）
