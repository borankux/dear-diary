# Dear-Diary Server Architecture Plan v1.0

> 将本地 CLI 工具升级为云端多设备同步服务

## 一、总体架构

```
┌──────────────────────────────────────────────────────────────────────────┐
│                         Dear-Diary 云端服务器                              │
├──────────────────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐    │
│  │  Auth MW    │  │  API Router │  │  Watcher    │  │  SSE Hub    │    │
│  │  (JWT)      │  │  (Protected)│  │  (fsnotify) │  │  (Realtime) │    │
│  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────┘    │
│         │               │                  │                  │          │
│         ▼               ▼                  ▼                  ▼          │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────────┐     │
│  │  /auth/*    │  │  /api/*     │  │  Auto-Process Engine        │     │
│  │  (public)   │  │  (JWT验证)  │  │  └── watcher → process     │     │
│  └─────────────┘  └─────────────┘  │  └── dedup → conflict res  │     │
│                                     │  └── todo completion detect│     │
│                                     └─────────────────────────────┘     │
│                           │                                              │
│                           ▼                                              │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │  SQLite Database (服务器主数据库)                                │   │
│  │  ├── todos, memories, ai_candidates                             │   │
│  │  ├── file_snapshots, processing_runs                             │   │
│  │  ├── transition_logs, sync_logs                                │   │
│  │  └── users (认证相关)                                           │   │
│  └─────────────────────────────────────────────────────────────────┘   │
│                           │                                              │
│                           ▼                                              │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │  Git Repository (日记文件)  ←── 设备A/设备B/Android 通过Git同步   │   │
│  │  ~/diary-data/YYYY-MM/YYYY-MM-DD.md                             │   │
│  └─────────────────────────────────────────────────────────────────┘   │
└──────────────────────────────────────────────────────────────────────────┘
```

## 二、设计原则

1. **Git 同步日记文件**：Markdown 是最适合 Git 的格式，保留完整版本历史
2. **SQLite 仅存服务器**：process.db 是服务器主数据库，设备通过 API 获取
3. **JWT 认证**：API 和 Dashboard 统一使用 JWT token
4. **Server-Sent Events**：实时推送更新到 Dashboard 和连接的设备
5. **自动化闭环**：文件变更 → 自动检测 → AI 处理 → 去重 → 生成输出 → 推送通知
6. **单密码模式**：环境变量 `DIARY_PASSWORD` 设置，首次登录创建 JWT

## 三、模块划分

### 模块 A: 认证系统 (`internal/server/auth/`)
- `auth.go` — JWT 生成、验证、中间件
- 环境变量：`DIARY_PASSWORD`（服务器密码）
- API：`POST /auth/login` 返回 JWT
- 前端：登录页面 `/login` → React 路由

### 模块 B: 服务器核心 (`internal/server/`)
- `server.go` — 扩展现有 web.Server，添加认证、SSE、Watcher
- `middleware.go` — JWT 认证中间件、CORS、日志
- `handlers.go` — 扩展 API，添加 diary CRUD、sync API

### 模块 C: 自动处理引擎 (`internal/server/watcher/`)
- `watcher.go` — fsnotify 监听日记文件变更
- `autoprocess.go` — 变更后自动触发 process runner
- `tododetector.go` — AI 检测新日记中已完成的 todo

### 模块 D: 同步系统 (`internal/server/sync/`)
- `sync.go` — 设备同步 API，增量同步日记元数据
- `sse.go` — Server-Sent Events hub，推送实时更新

### 模块 E: 智能去重 (`internal/process/` 扩展)
- `dedup.go` — 已有基础，增强跨设备/跨批次去重
- `conflict.go` — 自动冲突检测与解决（Git merge 冲突 + 数据冲突）

### 模块 F: Dashboard 更新 (`web/`)
- `Login.tsx` — 登录页面
- `api.ts` — 更新 API 客户端支持认证 header
- `App.tsx` — 路由保护，未登录跳转 login
- Dashboard 添加 SSE 连接，实时更新

### 模块 G: CLI 更新 (`cmd/diary/`)
- `sync` 子命令 — git push/pull 同步日记文件
- `remote` 配置 — 支持 `DIARY_REMOTE_URL` 环境变量
- `sync` 自动在 `diary` 命令后执行（可选）

## 四、数据流

### 正常写入流程（设备 A）
```
用户写日记 → 本地文件写入 → git add → git commit → git push → 服务器
                                                    ↓
服务器 watcher 检测 → git pull → 扫描变更 → AI process → 去重
 → 生成 candidates → 自动检测 todo 完成 → 更新数据库 → SSE 推送
 → 设备 B Dashboard 实时更新
```

### 设备 B 查看流程
```
Dashboard 打开 → 检查 JWT → 无则跳转登录 → 登录成功获取 token
 → 加载当前数据 → 建立 SSE 连接 → 实时接收更新
```

### 自动化引擎流程
```
watcher 检测到文件变更 → 触发 AutoProcess
  → 1. 读取新/变更的日记文件
  → 2. 运行现有 runner 流水线 (scan → extract → dedup → merge → persist)
  → 3. 新步骤：AI TodoCompletionDetector
      → 将新日记 + 所有 active todos 发给 AI
      → AI 返回哪些 todo 被提到已完成
      → 自动标记这些 todo 为 done
  → 4. 生成 summaries (todos.md, memories.md, dashboard.html)
  → 5. SSE 广播：{type: "process_complete", stats: {...}}
  → 6. 记录 processing_runs，确保幂等性
```

## 五、API 设计

### 公开 API（无需认证）
- `POST /auth/login` — 登录，返回 JWT
- `GET /health` — 健康检查

### 保护 API（需 JWT）
保留现有 API，全部添加 JWT 验证：
- `GET /api/stats`
- `GET /api/todos`, `POST /api/todos/{id}/status`
- `GET /api/candidates`, `POST /api/candidates/{id}/accept|reject`
- `GET /api/memories`
- `GET /api/diaries`, `GET /api/diaries/{date}`
- `GET /api/calendar`
- `GET /api/search?q=`

### 新增 API
- `POST /api/diaries` — 创建/更新日记（供 Android APP 使用）
- `GET /api/sync` — 增量同步（返回最近变更的日记列表和元数据）
- `GET /api/events` — SSE 连接端点

## 六、冲突解决策略

### Git 级别冲突
1. 日记文件由不同设备同时修改 → Git 自动 merge
2. 若无法自动 merge → 使用 Git 的 conflict markers
3. 自定义解析：取并集（两个时间段的日记都保留），在 conflict marker 处添加两个版本内容

### 数据级别冲突
1. 同一 candidate 被多个设备同时处理 → 数据库唯一约束 + 事务保护
2. Todo 状态冲突（A 标记 done，B 标记 in_progress）→ 最后写入者获胜（updated_at 时间戳）
3. 重复 candidate → content_key 去重（已存在）

## 七、实现顺序

1. **Phase 1**: 认证系统 + 服务器核心改造
2. **Phase 2**: 自动处理引擎（watcher + auto-process）
3. **Phase 3**: Todo 自动完成检测
4. **Phase 4**: 同步 API + SSE 实时推送
5. **Phase 5**: 智能去重与冲突解决增强
6. **Phase 6**: Dashboard 登录 + 实时更新
7. **Phase 7**: CLI sync 命令
8. **Phase 8**: 整合测试 + GitHub 推送

## 八、环境变量配置

```bash
# 服务器端
DIARY_PASSWORD="your-secure-password"     # 登录密码（必须）
DIARY_PORT="8765"                          # 服务器端口（默认8765）
DIARY_DATA_DIR="/var/lib/dear-diary"       # 数据目录（默认~/Documents/dear-diary）
DIARY_DB_PATH="/var/lib/dear-diary/process.db" # SQLite 路径
DIARY_LLM_API_KEY="..."                    # AI API Key（服务器端调用）
DIARY_LLM_BASE_URL="https://api.deepseek.com"
DIARY_LLM_MODEL="deepseek-chat"
DIARY_JWT_SECRET="..."                     # JWT 签名密钥（随机生成）
DIARY_AUTO_PROCESS="true"                  # 是否启用自动处理（默认true）
DIARY_WATCH_INTERVAL="30s"                 # 文件扫描间隔（默认30秒）

# 客户端
DIARY_REMOTE_URL="https://your-server.com" # 服务器地址（用于 sync）
DIARY_TOKEN="..."                          # JWT token（登录后保存）
```

## 九、安全考虑

1. **JWT 签名**：使用随机生成的 HMAC 密钥，服务器启动时若未设置则自动生成
2. **密码存储**：环境变量 `DIARY_PASSWORD`，不存储在任何文件中
3. **API Key**：LLM API key 仅存在于服务器环境变量，不返回给客户端
4. **HTTPS**：生产环境必须配置 HTTPS（反向代理如 Nginx/Caddy）
5. **CORS**：配置允许的域名，默认只允许同域
6. **Git 仓库**：日记文件仓库可以是私有仓库（GitHub），通过 deploy key 访问

## 十、未来扩展（Android APP）

- API 已支持 `POST /api/diaries` 创建日记
- 语音输入 → 文本 → 自动添加时间戳 → POST 到服务器
- 服务器 watcher 检测 → 自动处理 → 推送到所有设备
- 未来可考虑 Inbox 分离：增加 `inbox` 表区分个人日记和外部信息
