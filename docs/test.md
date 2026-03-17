# AI 代码审查 Agent - 测试文档

## 1. 测试策略

### 1.1 测试金字塔
```
       ╱ E2E 测试 ╲
      ╱───────────╲
     ╱  集成测试    ╲
    ╱───────────────╲
   ╱    单元测试      ╲
  ╱───────────────────╲
```

- **单元测试**：70% - 核心函数、工具类
- **集成测试**：20% - 模块间交互
- **E2E 测试**：10% - 完整流程

### 1.2 测试框架
```javascript
// package.json
{
  "devDependencies": {
    "jest": "^29.0.0",
    "supertest": "^6.0.0",
    "nock": "^13.0.0"
  }
}
```

---

## 2. 单元测试

### 2.1 Webhook 验证

```javascript
// tests/webhook/validator.test.js
const { validateSignature } = require('../../src/webhook/validator');

describe('Webhook Validator', () => {
  const secret = 'test-secret';

  test('should validate correct signature', () => {
    const payload = '{"action":"opened"}';
    const signature = 'sha256=abc123...';
    // 生成正确的签名
    const crypto = require('crypto');
    const hmac = crypto.createHmac('sha256', secret);
    const validSignature = 'sha256=' + hmac.update(payload).digest('hex');

    expect(validateSignature(payload, validSignature, secret)).toBe(true);
  });

  test('should reject invalid signature', () => {
    expect(validateSignature('{}', 'sha256=invalid', secret)).toBe(false);
  });

  test('should reject missing signature', () => {
    expect(validateSignature('{}', '', secret)).toBe(false);
  });
});
```

### 2.2 数据解析

```javascript
// tests/webhook/parser.test.js
const { parsePullRequestEvent } = require('../../src/webhook/parser');

describe('PR Event Parser', () => {
  test('should parse opened event', () => {
    const payload = {
      action: 'opened',
      number: 1,
      pull_request: {
        user: { login: 'developer' },
        head: { sha: 'abc123' }
      },
      repository: {
        owner: { login: 'owner' },
        name: 'repo'
      }
    };

    const result = parsePullRequestEvent(payload);

    expect(result.action).toBe('opened');
    expect(result.prNumber).toBe(1);
    expect(result.author).toBe('developer');
    expect(result.owner).toBe('owner');
    expect(result.repo).toBe('repo');
  });

  test('should filter non-target actions', () => {
    const payload = { action: 'closed', ... };
    const result = parsePullRequestEvent(payload);
    expect(result.isTarget).toBe(false);
  });
});
```

### 2.3 AI 分析器

```javascript
// tests/analyzer/prompt.test.js
const { buildPrompt } = require('../../src/analyzer/prompt');

describe('Prompt Builder', () => {
  test('should build valid prompt', () => {
    const files = [
      {
        filename: 'src/index.js',
        patch: '+ const a = 1;\n- var a = 1;'
      }
    ];

    const prompt = buildPrompt(files);

    expect(prompt).toContain('src/index.js');
    expect(prompt).toContain('+ const a = 1;');
  });

  test('should handle empty files', () => {
    const prompt = buildPrompt([]);
    expect(prompt).toContain('没有文件变更');
  });
});
```

### 2.4 评论格式化

```javascript
// tests/comment/formatter.test.js
const { formatReviewComment } = require('../../src/comment/formatter');

describe('Comment Formatter', () => {
  test('should format review with issues', () => {
    const review = {
      summary: '代码总体良好',
      issues: [
        {
          severity: 'warning',
          category: 'best-practice',
          location: 'src/index.js:5',
          message: '建议使用 const',
          suggestion: 'const value = 1;'
        }
      ]
    };

    const comment = formatReviewComment(review);

    expect(comment).toContain('## 🤖 AI 代码审查');
    expect(comment).toContain('代码总体良好');
    expect(comment).toContain('src/index.js:5');
    expect(comment).toContain('建议使用 const');
  });

  test('should handle empty issues', () => {
    const review = { summary: '很好', issues: [] };
    const comment = formatReviewComment(review);

    expect(comment).toContain('很好');
    expect(comment).not.toContain('####');
  });
});
```

---

## 3. 集成测试

### 3.1 Webhook 端到端

```javascript
// tests/integration/webhook.test.js
const request = require('supertest');
const nock = require('nock');
const app = require('../../src/index');

describe('Webhook E2E', () => {
  beforeAll(() => {
    // Mock GitHub API
    nock('https://api.github.com')
      .persist()
      .get('/repos/owner/repo/pulls/1/files')
      .reply(200, [
        {
          filename: 'src/index.js',
          patch: '+ const a = 1;'
        }
      ])
      .post('/repos/owner/repo/issues/1/comments')
      .reply(201, { id: 123 });
  });

  test('should process PR opened event', async () => {
    const payload = {
      action: 'opened',
      number: 1,
      pull_request: {
        user: { login: 'developer' },
        head: { sha: 'abc123' }
      },
      repository: {
        owner: { login: 'owner' },
        name: 'repo'
      }
    };

    const response = await request(app)
      .post('/webhook/github')
      .set('X-GitHub-Event', 'pull_request')
      .set('X-Hub-Signature-256', 'sha256=valid')
      .send(payload);

    expect(response.status).toBe(200);
  });
});
```

### 3.2 GitHub API 集成

```javascript
// tests/integration/github.test.js
const { getChangedFiles } = require('../../src/github/diff');

describe('GitHub API Integration', () => {
  test('should fetch files successfully', async () => {
    nock('https://api.github.com')
      .get('/repos/owner/repo/pulls/1/files')
      .reply(200, [
        { filename: 'a.js', patch: '...' },
        { filename: 'b.js', patch: '...' }
      ]);

    const files = await getChangedFiles({
      owner: 'owner',
      repo: 'repo',
      pullNumber: 1
    });

    expect(files).toHaveLength(2);
  });

  test('should handle rate limit', async () => {
    nock('https://api.github.com')
      .get('/repos/owner/repo/pulls/1/files')
      .reply(403, { message: 'API rate limit exceeded' });

    await expect(getChangedFiles({
      owner: 'owner',
      repo: 'repo',
      pullNumber: 1
    })).rejects.toThrow('rate_limit');
  });
});
```

---

## 4. E2E 测试

### 4.1 完整流程测试

```javascript
// tests/e2e/full-flow.test.js
const { execSync } = require('child_process');

describe('Full E2E Flow', () => {
  test('should run webhook to comment flow', () => {
    // 1. 启动服务
    execSync('npm start', { detached: true });

    // 2. 模拟 Webhook 请求
    const response = await fetch('http://localhost:3000/webhook/github', {
      method: 'POST',
      headers: {
        'X-GitHub-Event': 'pull_request',
        'Content-Type': 'application/json'
      },
      body: JSON.stringify({
        action: 'opened',
        number: 1,
        pull_request: { user: { login: 'dev' }, head: { sha: 'abc' } },
        repository: { owner: { login: 'owner' }, name: 'repo' }
      })
    });

    expect(response.status).toBe(200);

    // 3. 清理
    execSync('pkill -f "node index.js"');
  });
});
```

---

## 5. 测试数据

### 5.1 Mock 数据

```javascript
// tests/fixtures/
// ├── payload-opened.json
// ├── payload-synchronize.json
// ├── payload-closed.json
// ├── file-js.json
// ├── file-python.json
// └── review-result.json
```

---

## 6. 测试覆盖率目标

| 模块 | 覆盖率目标 |
|------|-----------|
| webhook/ | 80% |
| github/ | 75% |
| analyzer/ | 70% |
| comment/ | 80% |
| config/ | 90% |
| **总体** | **70%** |

---

## 7. CI/CD 集成

```yaml
# .github/workflows/test.yml
name: Test

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-node@v3
        with:
          node-version: '18'
      - run: npm ci
      - run: npm test -- --coverage
      - uses: codecov/codecov-action@v3
```

---

## 8. 测试检查清单

### 功能测试
- [ ] Webhook 签名验证正确
- [ ] PR 事件解析正确
- [ ] 文件 diff 获取成功
- [ ] AI 分析返回结果
- [ ] 评论正确发布
- [ ] 错误处理正确

### 边界测试
- [ ] 空 PR（无文件变更）
- [ ] 超大文件跳过
- [ ] 文件数量限制
- [ ] 黑名单用户跳过
- [ ] API 限流处理
- [ ] 网络超时处理

### 安全测试
- [ ] 非法签名拒绝
- [ ] 恶意 payload 处理
- [ ] 敏感信息过滤
