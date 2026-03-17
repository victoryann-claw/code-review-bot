# CodeReviewBot - AI 代码审查 Agent

一个自动化的 AI 代码审查工具，通过 GitHub Webhook 接收 PR 事件，调用 LLM 进行智能代码审查。

## 功能特性

- 🤖 **AI 驱动**：基于 GPT-4 的智能代码审查
- 🔔 **自动触发**：GitHub Webhook 自动触发
- 📊 **多维度审查**：代码规范、潜在 bug、性能优化、安全漏洞
- 💬 **智能评论**：Markdown 格式，代码高亮
- ⚙️ **可配置**：支持黑名单、审查深度、语言过滤

## 快速开始

### 1. 克隆项目

```bash
git clone https://github.com/your-username/code-review-bot.git
cd code-review-bot
```

### 2. 安装依赖

```bash
npm install
```

### 3. 配置环境变量

```bash
cp .env.example .env
# 编辑 .env 填入配置
```

### 4. 本地开发

```bash
# 方式一：使用 ngrok 测试 Webhook
ngrok http 3000

# 方式二：直接运行
npm run dev
```

### 5. 部署

```bash
# Vercel 部署
vercel deploy

# 或 Docker 部署
docker build -t code-review-bot .
docker run -d -p 3000:3000 --env-file .env code-review-bot
```

## 配置说明

| 环境变量 | 必填 | 说明 |
|---------|------|------|
| GITHUB_WEBHOOK_SECRET | ✅ | GitHub Webhook 密钥 |
| GITHUB_APP_ID | ✅ | GitHub App ID |
| GITHUB_APP_PRIVATE_KEY | ✅ | GitHub App 私钥 |
| OPENAI_API_KEY | ✅ | OpenAI API Key |
| OPENAI_MODEL | ❌ | 模型，默认 gpt-4 |

## GitHub App 配置

1. 创建 GitHub App
2. 设置 Webhook URL
3. 权限配置：
   - Pull requests: Read & Write
   - Repository contents: Read
4. 安装到仓库

## 项目结构

```
code-review-bot/
├── src/
│   ├── webhook/       # Webhook 处理
│   ├── github/       # GitHub API
│   ├── analyzer/     # AI 分析
│   ├── comment/      # 评论生成
│   └── config/       # 配置
├── tests/            # 测试
├── docs/             # 文档
└── index.js          # 入口
```

## 文档

- [需求文档](./docs/requirements.md)
- [技术文档](./docs/technical.md)
- [详细设计](./docs/design.md)
- [测试文档](./docs/test.md)

## License

MIT
