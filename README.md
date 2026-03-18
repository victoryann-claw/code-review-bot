# CodeReviewBot - AI 代码审查 Agent

一个自动化的 AI 代码审查工具，通过 GitHub Webhook 接收 PR 事件，调用 LLM 进行智能代码审查。

## 功能特性

- 🤖 **AI 驱动**：基于 GPT-4 / MiniMax 的智能代码审查
- 🔔 **自动触发**：GitHub Webhook 自动触发
- 📊 **多维度审查**：代码规范、潜在 bug、性能优化、安全漏洞
- 💬 **智能评论**：Markdown 格式，代码高亮
- ⚙️ **可配置**：支持黑名单、审查深度、语言过滤
- 🐳 **容器化**：Docker / Kubernetes 部署支持

## 快速开始

### 1. 克隆项目

```bash
git clone https://github.com/victoryann-claw/code-review-bot.git
cd code-review-bot
```

### 2. 配置文件

```bash
cp config.yaml.example config.yaml
# 编辑 config.yaml 填入配置
```

或使用环境变量：

```bash
cp .env.example .env
# 编辑 .env 填入配置
```

### 3. 本地运行

```bash
# 直接运行
go run main.go

# 或使用 Docker
docker-compose up
```

### 4. 部署

```bash
# Docker 部署
docker build -t code-review-bot .
docker run -d -p 3000:3000 --env-file .env code-review-bot

# Kubernetes 部署
kubectl apply -f deploy/k8s/
```

## 配置说明

### 配置文件 (config.yaml)

```yaml
app:
  host: "0.0.0.0"
  port: 3000

github:
  webhook_secret: "your_webhook_secret"
  token: "your_github_token"

llm:
  provider: "minimax"  # openai 或 minimax
  api_key: "your_api_key"
  model: "MiniMax-Text-01"

log:
  level: "debug"
```

### 环境变量

| 变量 | 必填 | 说明 |
|------|------|------|
| GITHUB_TOKEN | ✅ | GitHub Personal Access Token |
| GITHUB_WEBHOOK_SECRET | ✅ | Webhook 签名密钥 |
| LLM_PROVIDER | ❌ | `openai` 或 `minimax`，默认 openai |
| OPENAI_API_KEY | ❌ | OpenAI API Key |
| MINIMAX_API_KEY | ❌ | MiniMax API Key |
| MINIMAX_MODEL | ❌ | MiniMax 模型，默认 MiniMax-Text-01 |

## GitHub Webhook 配置

1. 进入仓库 Settings → Webhooks
2. 添加 Webhook：
   - URL: `http://<your-server>/webhook`
   - Secret: 与配置中的 GITHUB_WEBHOOK_SECRET 一致
   - 事件: Pull requests

## 项目结构

```
code-review-bot/
├── main.go                    # 程序入口
├── go.mod                    # Go 模块
├── Dockerfile                 # Docker 镜像
├── docker-compose.yml         # 本地开发
├── config.yaml               # 配置文件
├── internal/
│   ├── handler/              # Webhook 处理
│   ├── github/              # GitHub API 客户端
│   ├── analyzer/            # LLM 分析
│   ├── formatter/           # 评论格式化
│   ├── config/             # 配置管理
│   └── types/              # 类型定义
├── deploy/
│   └── k8s/                # Kubernetes 部署
└── docs/                    # 文档
```

## 技术栈

- Go 1.21+
- Gin Web 框架
- go-github
- go-openai / MiniMax SDK
- Docker / Kubernetes

## 文档

- [需求文档](./docs/requirements.md)
- [技术文档](./docs/technical.md)
- [详细设计](./docs/design.md)
- [测试文档](./docs/test.md)
- [技术解读](./docs/code-guide.md)

## License

MIT
