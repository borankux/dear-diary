# Dear-Diary 云端部署配置指南

> 部署服务器: `47.83.215.224` (阿里云香港)  
> 域名: `https://diary.wakitsoft.com`  
> 版本: v0.6.1-server

---

## 一、访问地址

| 环境 | URL | 状态 |
|------|-----|------|
| 线上 | `https://diary.wakitsoft.com` | ✅ HTTPS + 登录 |
| 健康检查 | `https://diary.wakitsoft.com/health` | ✅ |

**登录密码**: 只保存在服务器 `/opt/dear-diary/.env`，不要写入仓库文档。

---

## 二、服务器架构

```
┌─────────────────────────────────────────────────────────┐
│  Nginx (80 → 301 → 443)                                  │
│  ├── /.well-known/acme-challenge/  (Let's Encrypt)      │
│  └── / → proxy_pass → 127.0.0.1:8765                     │
├─────────────────────────────────────────────────────────┤
│  Dear-Diary Server (Go)                                  │
│  ├── /auth/login          (公开)                         │
│  ├── /health              (公开)                         │
│  ├── /api/*               (需 JWT)                       │
│  └── /api/events          (SSE 实时推送)                │
├─────────────────────────────────────────────────────────┤
│  Auto-Process Engine                                     │
│  ├── fsnotify 监听日记文件变更                           │
│  ├── Runner 运行 AI 处理流水线                           │
│  ├── TodoCompletionDetector 自动检测完成                 │
│  └── SSE Hub 推送实时更新                                │
├─────────────────────────────────────────────────────────┤
│  数据存储                                                │
│  ├── Git: /var/lib/dear-diary/       (日记 Markdown)    │
│  ├── Git Bare: /var/lib/dear-diary.git/ (同步用)       │
│  └── SQLite: /var/lib/dear-diary/process.db             │
└─────────────────────────────────────────────────────────┘
```

---

## 三、多设备同步配置

### 3.1 本地设备（Mac/PC）

在 `~/.zshrc` 或 `~/.bashrc` 中添加：

```bash
# Dear Diary 远程同步地址
export DIARY_REMOTE_URL="ssh://pb/var/lib/dear-diary.git"
```

> `pb` 是 SSH config 中的 host 别名，指向 `47.83.215.224`（使用 `~/.ssh/progress.pem` 密钥）

### 3.2 初始化本地 Git 仓库（首次）

如果本地日记目录还没有 Git 仓库：

```bash
cd ~/Documents/dear-diary
git init
git remote add origin ssh://pb/var/lib/dear-diary.git
git branch -m main

# 创建 .gitignore
cat > .gitignore << 'EOF'
/process/
/dashboard.html
/entries/
*.db
EOF
git add .gitignore
git commit -m "init: local diary"
```

### 3.3 日常使用

**写完日记推送：**
```bash
diary           # 写今天日记
diary sync      # 自动 git commit + push 到服务器
```

**从服务器拉取：**
```bash
diary sync pull
```

### 3.3 另一台设备（设备 B）

```bash
# 克隆服务器日记
git clone ssh://pb/var/lib/dear-diary.git ~/Documents/dear-diary

# 配置环境变量
export DIARY_REMOTE_URL="ssh://pb/var/lib/dear-diary.git"

# 以后日常同步
diary sync
diary sync pull
```

---

## 四、服务器管理

### 4.1 服务控制

```bash
ssh -i ~/.ssh/progress.pem root@47.83.215.224

# 查看状态
systemctl status dear-diary

# 重启
systemctl restart dear-diary

# 查看日志
journalctl -u dear-diary -f

# 查看最近日志
journalctl -u dear-diary --no-pager -n 50
```

### 4.2 环境变量配置

配置文件: `/opt/dear-diary/.env`

```bash
DIARY_PASSWORD=<server-login-password>  # 登录密码
DIARY_DATA_DIR=/var/lib/dear-diary      # 日记根目录
DIARY_DB_PATH=/var/lib/dear-diary/process.db
DIARY_PORT=0.0.0.0:8765
DIARY_AUTO_PROCESS=true
DIARY_WATCH_INTERVAL=30s
DIARY_LLM_API_KEY=<llm-api-key>
DIARY_LLM_BASE_URL=https://api.deepseek.com
DIARY_LLM_MODEL=deepseek-chat
```

修改后重启：
```bash
systemctl restart dear-diary
```

### 4.3 手动触发处理

```bash
# 在服务器上
export DIARY_DIR=/var/lib/dear-diary
export DIARY_DB_PATH=/var/lib/dear-diary/process.db
/opt/dear-diary/diary process
```

### 4.4 数据库查看

```bash
sqlite3 /var/lib/dear-diary/process.db
.tables
SELECT * FROM todos WHERE status='active';
.exit
```

---

## 五、Nginx 配置

路径: `/www/server/panel/vhost/nginx/diary.wakitsoft.com.conf`

```nginx
server {
    listen 80;
    server_name diary.wakitsoft.com;

    location /.well-known/acme-challenge/ {
        root /www/wwwroot/diary.wakitsoft.com;
    }

    location / {
        return 301 https://$host$request_uri;
    }
}

server {
    listen 443 ssl;
    http2 on;
    server_name diary.wakitsoft.com;

    ssl_certificate /etc/letsencrypt/live/diary.wakitsoft.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/diary.wakitsoft.com/privkey.pem;
    ssl_protocols TLSv1.2 TLSv1.3;

    location / {
        proxy_pass http://127.0.0.1:8765;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_read_timeout 3600s;
        proxy_send_timeout 3600s;
        proxy_buffering off;
        proxy_cache off;
    }
}
```

---

## 六、API 接口（供 Android APP 使用）

| 接口 | 方法 | 认证 | 说明 |
|------|------|------|------|
| `POST /auth/login` | 公开 | 登录获取 JWT |
| `GET /health` | 公开 | 健康检查 |
| `GET /api/stats` | JWT | 统计数据 |
| `GET /api/todos` | JWT | Todo 列表 |
| `GET /api/candidates` | JWT | AI 候选 |
| `GET /api/memories` | JWT | 记忆列表 |
| `GET /api/diaries` | JWT | 日记列表 |
| `GET /api/diaries/{date}` | JWT | 单日日记 |
| `POST /api/diaries` | JWT | 创建/追加日记 |
| `GET /api/sync` | JWT | 增量同步 |
| `GET /api/events` | JWT | SSE 实时连接 |

### 请求示例（Android）

```bash
# 登录
curl -X POST https://diary.wakitsoft.com/auth/login \
  -H "Content-Type: application/json" \
  -d '{"password":"<server-login-password>"}'

# 获取数据（使用返回的 token）
curl -H "Authorization: Bearer <token>" \
  https://diary.wakitsoft.com/api/stats

# 创建日记（手机语音输入后调用）
curl -X POST https://diary.wakitsoft.com/api/diaries \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"date":"2026-06-29","content":"# 2026-06-29\n\n[语音转文字]..."}'
```

---

## 七、SSL 证书

- **颁发机构**: Let's Encrypt
- **到期**: 2026-09-26
- **自动续期**: Certbot 已配置定时任务
- **证书路径**:
  - `/etc/letsencrypt/live/diary.wakitsoft.com/fullchain.pem`
  - `/etc/letsencrypt/live/diary.wakitsoft.com/privkey.pem`

手动续期：
```bash
certbot renew --force-renewal -d diary.wakitsoft.com
```

---

## 八、常见问题

### Q: 登录页面显示"密码错误"但密码是对的
A: 检查 `api.ts` 中 `login()` 函数是否调用 `/auth/login`（不是 `/api/auth/login`）。已修复。

### Q: `diary sync` 提示 Host key verification failed
A: 确保 SSH config 中有 `pb` host 配置，且 `~/.ssh/progress.pem` 权限为 600：
```bash
chmod 600 ~/.ssh/progress.pem
```

### Q: `diary sync` 提示分支不存在
A: 确保服务器 Git 仓库是 bare 仓库（已配置），且本地分支名为 `main`：
```bash
git branch -m main
```

### Q: 服务器上自动处理不触发
A: 检查 `DIARY_LLM_API_KEY` 是否设置：
```bash
grep DIARY_LLM_API_KEY /opt/dear-diary/.env
```

### Q: 如何更新服务器代码
A: 本地重新编译后上传：
```bash
# 本地
cd dear-diary
GOOS=linux GOARCH=amd64 go build -o bin/diary-linux ./cmd/diary
scp bin/diary-linux root@47.83.215.224:/opt/dear-diary/diary

# 服务器
systemctl restart dear-diary
```

---

## 九、安全提醒

- 服务器到期日: **2026-07-20**（需续费）
- 域名到期日: **2027-01-23**
- SSL 到期日: **2026-09-26**（自动续期）
- 登录密码建议定期更换。
- LLM API key 仅存储在服务器环境变量，不提交到 Git。
- 如果真实 key 曾经进入提交历史，立即在 provider 侧轮换。

---

*部署时间: 2026-06-29*  
*部署者: Thirdbot*
