# AI 代码审查 Agent - 详细设计文档

## 1. 数据流设计

```
[GitHub] → [Webhook] → [Parser] → [Diff Fetcher] → [AI Analyzer] → [Comment Formatter] → [GitHub API] → [GitHub PR]
```

---

## 2. 核心流程时序图

```
┌─────────┐    ┌─────────┐    ┌─────────┐    ┌─────────┐    ┌─────────┐    ┌─────────┐
│ GitHub  │    │ Webhook │    │ Parser  │    │ Diff    │    │ AI      │    │ Comment│
│         │───▶│ Server  │───▶│         │───▶│ Fetcher │───▶│ Analyzer│───▶│        │
└─────────┘    └─────────┘    └─────────┘    └─────────┘    └─────────┘    └─────────┘
   Event         Validate    Extract      Fetch        Analyze       Format &
                 Signature   PR Info      Files        with LLM      Post
```

---

## 3. 数据结构

### 3.1 Webhook Payload
```javascript
{
  action: 'opened' | 'synchronize',
  number: 1,
  pull_request: {
    id: 123456,
    title: 'feat: add login',
    user: { login: 'developer' },
    head: { sha: 'abc123' },
    base: { sha: 'def456' }
  },
  repository: {
    owner: { login: 'owner' },
    name: 'repo'
  }
}
```

### 3.2 File Change
```javascript
{
  filename: 'src/index.js',
  status: 'modified',
  additions: 10,
  deletions: 5,
  patch: '@@ -1,5 +1,10 @@\n...'
}
```

### 3.3 AI Request
```javascript
{
  model: 'gpt-4',
  messages: [
    {
      role: 'system',
      content: '你是一个代码审查专家...'
    },
    {
      role: 'user',
      content: '请审查以下代码变更...\n\n文件: src/index.js\n\n```diff\n...```'
    }
  ]
}
```

### 3.4 Review Result
```javascript
{
  summary: '整体评价',
  issues: [
    {
      severity: 'warning',
      category: 'best-practice',
      location: 'src/index.js:10',
      message: '建议使用 const 替代 let',
      suggestion: 'const value = ...'
    }
  ]
}
```

---

## 4. 模块详细设计

### 4.1 Webhook Handler

```javascript
// src/webhook/handler.js
async function handlePullRequest(payload) {
  // 1. 过滤非目标事件
  if (!['opened', 'synchronize'].includes(payload.action)) {
    return { skipped: true, reason: 'non-target-action' };
  }

  // 2. 获取配置
  const config = await getRepoConfig(payload.repository);

  // 3. 跳过黑名单
  if (isBlacklisted(payload, config)) {
    return { skipped: true, reason: 'blacklisted' };
  }

  // 4. 获取文件变更
  const files = await github.getChangedFiles(payload);

  // 5. 过滤大文件
  const validFiles = files.filter(f => f.size < MAX_FILE_SIZE);

  // 6. AI 分析
  const review = await analyzer.analyze(validFiles);

  // 7. 发布评论
  await comment.post(payload, review);

  return { success: true };
}
```

### 4.2 AI Analyzer

```javascript
// src/analyzer/openai.js
const SYSTEM_PROMPT = `你是一个资深的代码审查专家。
请分析以下代码变更，从以下维度审查：
1. 代码规范
2. 潜在 bug
3. 性能问题
4. 安全漏洞
5. 代码重复
6. 最佳实践

请以 JSON 格式返回审查结果：
{
  "summary": "总体评价",
  "issues": [
    {
      "severity": "error|warning|info",
      "category": "bug|performance|security|best-practice",
      "location": "文件:行号",
      "message": "问题描述",
      "suggestion": "修复建议（可选）"
    }
  ]
}`;

async function analyze(files) {
  const content = buildPrompt(files);
  const response = await openai.chat.completions.create({
    model: 'gpt-4',
    messages: [
      { role: 'system', content: SYSTEM_PROMPT },
      { role: 'user', content }
    ],
    temperature: 0.3
  });

  return parseResponse(response.choices[0].message.content);
}
```

### 4.3 Comment Formatter

```javascript
// src/comment/formatter.js
function formatReviewComment(review) {
  const lines = [];

  // 总体评价
  lines.push(`## 🤖 AI 代码审查\n`);
  lines.push(`**总体评价**: ${review.summary}\n`);

  // 问题统计
  const stats = countBySeverity(review.issues);
  lines.push(`📊 问题统计: ${stats.error} 个错误, ${stats.warning} 个警告, ${stats.info} 个建议\n`);

  // 详细问题
  if (review.issues.length > 0) {
    lines.push(`---\n`);
    lines.push(`### 详细问题\n`);

    for (const issue of review.issues) {
      const emoji = getSeverityEmoji(issue.severity);
      lines.push(`#### ${emoji} ${issue.category} - ${issue.location}\n`);
      lines.push(`${issue.message}\n`);
      if (issue.suggestion) {
        lines.push(`**建议**: \`\`\`\n${issue.suggestion}\n\`\`\`\n`);
      }
    }
  }

  lines.push(`\n---\n`);
  lines.push(`*此评论由 AI 自动生成*\n`);

  return lines.join('\n');
}
```

---

## 5. 错误处理

| 错误场景 | 处理方式 |
|----------|----------|
| Webhook 签名验证失败 | 返回 401，跳过处理 |
| GitHub API 限流 | 指数退避重试，最多 3 次 |
| LLM API 超时 | 返回 504，记录日志 |
| LLM 返回格式错误 | 降级为简单分析 |
| 评论发布失败 | 重试 2 次，记录日志 |

---

## 6. 配置设计

### 6.1 仓库配置（可选）
可以在仓库中添加 `.codereviewbot.json`：

```json
{
  "enabled": true,
  "languages": ["javascript", "python"],
  "maxFiles": 10,
  "reviewDepth": "standard",
  "commentPosition": "pr",
  "ignoreUsers": ["bot", "dependabot"]
}
```

### 6.2 全局默认配置
```javascript
const DEFAULT_CONFIG = {
  enabled: true,
  languages: ['*'],  // * 表示所有
  maxFiles: 10,
  maxFileSize: 100 * 1024,  // 100KB
  reviewDepth: 'standard',   // simple | standard | deep
  commentPosition: 'pr',     // pr | files
  ignoreUsers: ['bot', 'dependabot', 'renovate'],
  severityThreshold: 'warning'  // error | warning | info
};
```

---

## 7. 扩展设计

### 7.1 多语言支持
- Python: Pylint 规则集成
- Go: golint 规则集成
- Java: Checkstyle 规则集成

### 7.2 规则引擎
```javascript
// 可扩展的规则系统
const rules = {
  'no-console': {
    enabled: true,
    severity: 'warning',
    check: (code) => code.includes('console.log')
  },
  'no-eval': {
    enabled: true,
    severity: 'error',
    check: (code) => code.includes('eval(')
  }
};
```

### 7.3 缓存设计
- 使用 LRU 缓存已审查的文件
- 相同 diff 不重复调用 LLM

---

## 8. 监控指标

| 指标 | 说明 |
|------|------|
| request_total | 总请求数 |
| request_success | 成功数 |
| request_skipped | 跳过数 |
| request_error | 错误数 |
| llm_latency | LLM 响应时间 |
| comment_posted | 评论发布数 |
