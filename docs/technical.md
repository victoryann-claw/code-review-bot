# AI 代码审查 Agent - 技术文档

## 1. 技术栈

### 1.1 后端
| 技术 | 版本 | 用途 |
|------|------|------|
| Node.js | >=18.0.0 | 运行时 |
| Express | ^4.18.0 | Web 框架 |
| Octokit | ^3.0.0 | GitHub API |
| dotenv | ^16.0.0 | 环境变量 |

### 1.2 AI
| 技术 | 用途 |
|------|------|
| OpenAI API (GPT-4) | 代码分析 |
| Claude API | 可选备用 |

### 1.3 部署
| 技术 | 用途 |
|------|------|
| Vercel / Railway | 托管服务 |
| PM2 | 进程管理 |

---

## 2. 系统架构

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│   GitHub        │────▶│   CodeReviewBot │────▶│   OpenAI API    │
│   Webhook       │     │   Server        │     │   (LLM)         │
└─────────────────┘     └────────┬────────┘     └─────────────────┘
                                 │
                                 ▼
                        ┌─────────────────┐
                        │  GitHub API     │
                        │  (评论 PR)      │
                        └─────────────────┘
```

---

## 3. 核心模块

### 3.1 Webhook 处理模块
```
src/
└── webhook/
    ├── index.js        # Webhook 入口
    ├── handler.js      # 事件分发
    ├── validator.js   # 签名验证
    └── parser.js      # PR 数据解析
```

**关键点**：
- 验证 X-Hub-Signature-256 签名
- 过滤非 PR 事件
- 异常处理与日志

### 3.2 代码获取模块
```
src/
└── github/
    ├── client.js       # Octokit 封装
    ├── diff.js         # 获取文件 diff
    └── files.js        # 文件列表处理
```

**关键点**：
- 递归获取多文件
- 大文件处理（超过 100KB 跳过）
- 速率限制处理

### 3.3 AI 分析模块
```
src/
└── analyzer/
    ├── index.js       # 分析入口
    ├── prompt.js       # Prompt 模板
    ├── openai.js      # OpenAI 调用
    └── parser.js      # 响应解析
```

**关键点**：
- 构建结构化 Prompt
- JSON 响应解析
- 超时与重试

### 3.4 评论模块
```
src/
└── comment/
    ├── index.js       # 评论入口
    ├── formatter.js   # 格式化审查结果
    └── github.js     # GitHub API 评论
```

**关键点**：
- Markdown 渲染
- 代码高亮
- 错误处理

---

## 4. 环境变量

```env
# GitHub
GITHUB_APP_ID=xxx
GITHUB_APP_PRIVATE_KEY=xxx
GITHUB_WEBHOOK_SECRET=xxx

# OpenAI
OPENAI_API_KEY=sk-xxx

# 可选
OPENAI_MODEL=gpt-4
MAX_FILES=10
```

---

## 5. API 接口

### 5.1 Webhook 端点
```
POST /webhook/github
```

### 5.2 健康检查
```
GET /health
```

### 5.3 手动触发审查（可选）
```
POST /review
Body: { owner, repo, pull_number }
```

---

## 6. 知识点清单

| 知识点 | 用途 |
|--------|------|
| GitHub Webhook | 接收事件 |
| HMAC SHA-256 | 签名验证 |
| REST API | GitHub API 调用 |
| Markdown | 评论格式 |
| Prompt Engineering | AI 分析质量 |
| 异步处理 | 并发审查 |
| 错误重试 | 稳定性 |

---

## 7. 部署架构

### 开发环境
```
本地 Node.js + ngrok (Webhook 测试)
```

### 生产环境
```
Vercel / Railway
├── Serverless Function (Webhook)
└── 定时任务 (可选)
```

或

```
自有服务器
├── PM2 进程管理
├── Nginx 反向代理
└── Systemd 服务
```

---

## 8. 安全考虑

1. **签名验证**：必须验证 Webhook 签名
2. **敏感信息**：代码不上传，仅在内存处理
3. **API 密钥**：存储在环境变量
4. **速率限制**：防止滥用
