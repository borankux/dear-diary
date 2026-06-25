# 亲爱的日记 (dear-diary)

一个 macOS 终端的 TUI 日记应用，用 Vim 写日记，用月历浏览。

## 为什么

- **Vim 原生写作**：100% 复用你的 `~/.vimrc`、插件、键位
- **TUI 月历导航**：一眼看到哪天写了 / 哪天没写
- **同一天多次打开 = 同一篇日记**：自动追加时间戳段落，连续记录不会覆盖
- **纯 Markdown 文件**：可读、可备份、可被 Obsidian 等工具读取
- **零运行时依赖**：单二进制，启动几乎零延迟

## 安装

```bash
git clone <this repo> dear-diary
cd dear-diary
make install        # 会问 sudo，装到 /usr/local/bin/diary
```

确认装好：
```bash
diary --version
```

## 使用

| 命令 | 行为 |
|------|------|
| `diary` | 打开今天，直接进 Vim |
| `diary browse` | 进入 TUI 月历浏览 |
| `diary today` | 同 `diary` |
| `diary yesterday` | 打开昨天（别名 `diary y`） |
| `diary 2026-06-24` | 打开指定日期 |
| `diary 6/24` | 月/日（默认今年） |
| `diary search 关键词` | 全文搜索所有日记 |
| `diary -h` | 帮助 |

### 同一天多次写

第一次：创建文件 + 写入标题 + 第一个时间段 → 进入 Vim

```markdown
# 2026-06-25 周四

## 09:00

今天开始做 TUI 日记应用。
```

几小时后再次 `diary`：自动在末尾追加新时间段

```markdown
# 2026-06-25 周四

## 09:00

今天开始做 TUI 日记应用。

## 22:30

下班前把 search 功能也加上了。
```

5 分钟内重复打开不会重复追加（避免误触产生空段落）。

### TUI 月历键位

```
hjkl / 方向键    移动光标
H / L 或 < / >   切换上一月 / 下一月
t                跳到今天
g / G            跳到月初 / 月末
Enter            打开当天 Vim
/                提示用 search 子命令
?                显示键位帮助
q / Esc          退出
```

视觉标记：
- `◆` 今天
- `●` 已写日记的天（绿色）
- 光标位置反色高亮

## 编辑器选择

优先级：
1. `$DIARY_EDITOR`（本项目专用）
2. `$EDITOR`（通用）
3. `vim`（默认）

设成其他编辑器：
```bash
export DIARY_EDITOR=nvim        # Neovim
export DIARY_EDITOR="code -w"   # VS Code (等待关闭)
```

> 提示：非 vim 系列编辑器不会自动追加时间戳光标定位（仍能正常打开文件）。

## 文件存储

```
~/Documents/dear-diary/
  2026-06/
    2026-06-23.md
    2026-06-24.md
    2026-06-25.md
  2026-07/
    2026-07-01.md
```

- 路径：`~/Documents/dear-diary/YYYY-MM/YYYY-MM-DD.md`
- iCloud Drive 开启时自动跨设备同步
- 可设 `$DIARY_DIR` 覆盖根目录（测试用）

## 搜索

优先用 `rg`（ripgrep），没装则自动回退到纯 Go 实现：

```bash
diary search Bubbletea
```

结果列表 TUI，`Enter` 打开、`j/k` 滚动、`q` 退出。

## 开发

```bash
make test         # 跑所有测试
make build        # 编译到 ./bin/diary
make fmt vet      # 格式化 + 静态检查
```

技术栈：Go 1.26+ · [Bubble Tea](https://github.com/charmbracelet/bubbletea) · [Lipgloss](https://github.com/charmbracelet/lipgloss)

## 不做的（YAGNI）

- 多用户、云同步（iCloud / Dropbox 已经够）
- 富文本 / 所见即所得（Markdown 就够）
- 加密（macOS FileVault + iCloud ADP 已经够，需要时单独加）
- 内嵌编辑器（Vim 才是想要的）
- 心情 / 天气 metadata（MVP 不做，未来扩展）
- 自动备份（让 iCloud / git 来）

## License

Personal use.
