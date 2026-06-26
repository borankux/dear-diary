# Dear Diary AI 提炼系统 — POC 规划

> 把日记作为 Inbox，由 AI 自动提炼为结构化资产：Todos、Memories、Questions。
> 文件创建时间：2026-06-26

---

## 1. 核心想法

Dear Diary 现在是一个 Vim-first 的本地日记工具。下一步希望增加一个 `diary process` 命令，让 AI 自动阅读新增/修改的日记内容，从中提炼出：

- **Todos**：要做的事情、已完成的事情、状态发生变化的事情
- **Memories**：重要知识、发现、经验、关系型记忆
- **Questions**：暂时没空深入研究的问题（前期先合并进 Todo 管理）

目标是让日记保持为"纯个人输入"，而 AI 提炼出来的结构化资产单独存放。

---

## 2. 关键约束

- **增量处理**：日记会越积越多，不能每次全盘扫描。要根据文件变化（mtime / hash）只处理新增或修改过的日记。
- **本地优先**：先不依赖 OpenClaw、线上服务或第三方工具。
- **命令行入口**：从 `diary process` 开始验证。
- **模型**：先用 DeepSeek（便宜），走 OpenAI 兼容 API。
- **多 Agent**：考虑用 Python + CrewAI 做 processing core，Go 的 `dear-diary` 调用 Python script。
- **可视化最后做**：先保证数据和状态机正确，Web UI 是后续阶段。

---

## 3. 设计决策确认

### 3.1 承载系统：SQLite + Markdown 双轨

- **SQLite**：作为系统主数据库，支撑后续 Web Application 和查询。
- **Markdown**：作为人类可直接阅读的可视化输出，比如当前活跃 Todo 列表、Memory 摘要等。
- 两者由 processing core 同步生成。

### 3.2 Todo 存储：独立目录 + Archive 机制

- 不放在 diary 日记文件里，也不放在 `todos.md` 这种会无限膨胀的全局文件中。
- 活跃 Todo 单独维护，保证列表始终是"当前需要关注"的事情。
- 已完成或过期的 Todo 可以归档到 archive，必要时可以主动翻出来找活干。
- 不与 Apple Reminders、飞书任务等第三方工具联动，打造自己的工具生态。

### 3.3 Memory 形式：理想是知识图谱 + 时序图，MVP 从主题式开始

- 长期理想形态：卡片有 tag、有关联、有时间线，可以追踪一个主题的发展过程。
- MVP 阶段：可以先按主题聚合（比如"AI 工具"、"项目管理"），验证提取和合并逻辑。
- 后续再引入关系图谱、时间线可视化。

### 3.4 CrewAI / Multi-agent：先做 Python POC

- Processing core 用 Python 实现，不一定是 Go。
- Go 的 `dear-diary` 负责扫描文件、调用 Python script、接收结果、写入 SQLite/Markdown。
- 先做一个最简单的 sequential pipeline 验证效果，再考虑是否升级为 CrewAI multi-agent。

### 3.5 Questions：前期合并进 Todo

- 日记中提炼出的问题先作为 Todo 的一种（类型标记为 `question`）。
- 后续如果问题积累多了，再拆成独立的 Questions 资产。

---

## 4. POC 范围（第一阶段）

POC 目标：验证"从日记提取结构化资产"这件事是否可行、稳定、可增量。

### 4.1 输入

- 只处理最近 1-3 天的日记文件。
- 通过 mtime 判断哪些文件需要处理。
- 每个日记片段传给 AI 时，附带现有活跃 Todo / Memory 的摘要，方便 AI 判断是新增、更新还是重复。

### 4.2 处理流程

```
扫描日记文件变化
    ↓
读取新增/修改的日记内容
    ↓
调用 Python processing script（DeepSeek）
    ↓
提取 Todo / Memory / Question 建议
    ↓
去重/合并（AI + 确定性代码）
    ↓
写入 SQLite
    ↓
生成 Markdown 摘要供人工查看
    ↓
输出处理报告到命令行
```

### 4.3 输出

- SQLite 表：`todos`, `memories`, `processing_logs`
- Markdown：当前活跃 Todo 列表、Memory 主题摘要
- 命令行报告：处理了多少文件、新增/更新/忽略了多少实体

---

## 5. 下一步行动

1. 在 `dear-diary` 项目里新增一个 `process` 子命令或独立入口。
2. 创建 Python processing core 的初始脚本。
3. 设计 SQLite schema（最小化）。
4. 写第一个 DeepSeek prompt，从日记片段提取 Todo / Memory。
5. 跑一轮真实日记，人工检查提取质量。

---

## 6. 备注

- 这个文件是项目规划，不是日记。日记里只放个人内容，不放 AI/项目设计。
- 当前磁盘清理任务优先，此 POC 等清理完成后再启动。
