# 技术设计总览

> 最后更新：2026-06-25

## 概述
（待填充详细技术架构）

## 技术栈
- Go 1.26+
- Bubble Tea（TUI 框架）
- Lipgloss（样式）
- 单二进制，无运行时依赖（除 Vim）

## 模块划分

```
cmd/diary/          CLI 入口
internal/storage/   文件系统抽象（路径、创建、追加、扫描）
internal/editor/    $EDITOR 调用，Vim 光标定位
internal/search/    ripgrep / 纯 Go 回退
internal/stats/     streak 计算
internal/memory/    OnThisDay 回顾
internal/tui/       Bubbletea Models（月历 + 搜索结果）
```

## 关键约定

### 存储格式
- 路径：`~/Documents/dear-diary/YYYY-MM/YYYY-MM-DD.md`
- 文件名是唯一真相来源（不解析内容推断日期）
- 模板：`# YYYY-MM-DD 周X\n\n## HH:MM\n\n正文`
- 同日追加：`\n\n## HH:MM\n\n`（5 分钟内不重复）

### 编辑器集成
- 优先级：`$DIARY_EDITOR` > `$EDITOR` > `vim`
- Vim 系列：`+normal Go` 让光标停在末尾追加位置
- TUI 模式：`tea.ExecProcess` 释放 raw mode 给 Vim

### 月历视图
- 周一开头（中国习惯）
- `◆` 今天，`●` 已写日记的天，光标反色
- 月份切换不缓存（每次切月重扫，毫秒级）

详细见 [spec.md](../spec.md)。
