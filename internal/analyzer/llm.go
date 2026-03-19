package analyzer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/sashabaranov/go-openai"
	"github.com/victoryann-claw/code-review-bot/internal/types"
)

// LLMAnalyzer handles code analysis using LLM
type LLMAnalyzer struct {
	client      *openai.Client
	provider    string
	model       string
	temperature float32
	maxTokens   int
}

// NewLLMAnalyzer creates a new LLM analyzer
func NewLLMAnalyzer() *LLMAnalyzer {
	provider := os.Getenv("LLM_PROVIDER")
	if provider == "" {
		provider = "openai"
	}

	var model string
	var apiKey string
	var baseURL string

	switch provider {
	case "bailian", "aliyun", "qwen":
		// Alibaba Cloud Bailian (阿里云百炼)
		apiKey = os.Getenv("DASHSCOPE_API_KEY")
		if apiKey == "" {
			apiKey = os.Getenv("OPENAI_API_KEY") // fallback
		}
		model = os.Getenv("DASHSCOPE_MODEL")
		if model == "" {
			model = "qwen3.5-plus"
		}
		baseURL = "https://dashscope.aliyuncs.com/compatible-mode/v1"

	case "minimax":
		apiKey = os.Getenv("MINIMAX_API_KEY")
		if apiKey == "" {
			apiKey = os.Getenv("OPENAI_API_KEY")
		}
		model = os.Getenv("MINIMAX_MODEL")
		if model == "" {
			model = "MiniMax-Text-01"
		}
		baseURL = "https://api.minimax.chat/v1"

	default: // openai
		apiKey = os.Getenv("OPENAI_API_KEY")
		model = os.Getenv("OPENAI_MODEL")
		if model == "" {
			model = "gpt-4"
		}
		baseURL = os.Getenv("LLM_BASE_URL")
	}

	cfg := openai.DefaultConfig(apiKey)
	if baseURL != "" {
		cfg.BaseURL = baseURL
	}

	client := openai.NewClientWithConfig(cfg)

	return &LLMAnalyzer{
		client:      client,
		provider:    provider,
		model:       model,
		temperature: 0.3,
		maxTokens:   8000,
	}
}

// AnalyzeCode analyzes code diff using LLM
func (a *LLMAnalyzer) AnalyzeCode(ctx context.Context, diff string, prDetails *types.PRDetails) ([]types.Issue, error) {
	log.Printf("[DEBUG] Analyzing code with LLM (%s, provider: %s)", a.model, a.provider)

	systemPrompt := `你是一位资深的代码审查专家。请分析以下 GitHub Pull Request 的代码差异，识别潜在的问题、bug、安全漏洞、代码质量问题或改进建议。

**重要限制 - 请严格遵守：**
- **只报告在 diff 中可以直接确认的问题** - 你只能看到本次提交的代码变动
- **不要推测文件其他部分的状态** - 例如无法判断删除的 import 是否还在其他地方使用
- **不要报告依赖全量代码才能判断的问题** - 如"变量可能未定义"、"函数可能在其他地方被调用"
- **只报告在 diff 中明确看到的问题** - 语法错误、逻辑错误、硬编码密码、明显的 bug 等

请用中文回复。description（问题描述）和 suggestion（修复建议）必须使用中文。

**⚠️ 关键审查原则 - 请严格遵守：**

1. **关于 import 包使用的检查（最重要！）**：
   - 步骤1：先看 diff 中新增的 import 语句（如 import "strings"、import "log"）
   - 步骤2：在整个 diff 的代码变更部分搜索该包名+点号（如 strings.、fmt.、log.、os.）
   - 步骤3：只有当步骤2找不到任何该包的函数调用时，才能判定为"未使用"
   - 特别注意：diff 显示的是变更的部分，但函数体内部可能也有变更，必须仔细检查
   
2. **关于代码优化/重构类 PR**：
   - 如果 PR 只是简单的代码优化（如 fmt.Printf→log.Printf、用标准库替换第三方库）
   - 且没有引入任何功能性变更或潜在 bug
   - 那么应该判定为"无问题"，返回空数组 []
   
3. **严格的问题判定标准**：
   - 只有真正影响功能、安全或性能的问题才需要报告
   - 轻微的代码风格问题（如可以忽略的格式化建议）不应报告
   - 不要吹毛求疵，过度审查

**重要：JSON 键名必须保持英文**，否则后端无法解析。返回格式如下：
- type: 问题类型，如 "bug"、"security"、"performance"、"style"、"suggestion"（必须英文）
- severity: 严重程度，如 "high"、"medium"、"low"（必须英文）
- file: 文件名（如适用）
- line: 行号（如适用）
- description: 问题的简要描述（必须使用中文）
- suggestion: 如何修复或改进（必须使用中文）

请只返回一个有效的 JSON 数组。如果没有发现问题，返回空数组 []。

响应示例（系统会自动处理 Markdown 代码块标记，请直接返回 JSON）：
[
  {
    "type": "bug",
    "severity": "high",
    "file": "internal/handler/webhook.go",
    "line": 42,
    "description": "空指针解引用风险：user 变量可能为 nil",
    "suggestion": "在使用前检查 user 是否为 nil"
  }
]`

	userPrompt := buildUserPrompt(diff, prDetails)

	// Build request
	req := openai.ChatCompletionRequest{
		Model: a.model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: systemPrompt,
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: userPrompt,
			},
		},
		Temperature: a.temperature,
		MaxTokens:   a.maxTokens,
	}

	// Add response format for Bailian and OpenAI to ensure JSON output
	if a.provider == "bailian" || a.provider == "aliyun" || a.provider == "qwen" || a.provider == "openai" || a.provider == "minimax" {
		req.ResponseFormat = openai.ChatCompletionResponseFormat{
			Type: "json_object",
		}
	}

	resp, err := a.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return nil, err
	}

	content := resp.Choices[0].Message.Content

	// Parse JSON response
	issues, err := parseIssues(content)
	if err != nil {
		log.Printf("[DEBUG] Failed to parse LLM response: %v", err)
		// Return empty issues with error to indicate parsing failure
		return nil, fmt.Errorf("failed to parse LLM response: %w", err)
	}

	log.Printf("[DEBUG] LLM found %d issues", len(issues))
	return issues, nil
}

func buildUserPrompt(diff string, prDetails *types.PRDetails) string {
	return fmt.Sprintf(`Pull Request #%d: %s
Author: %s
Branch: %s -> %s

Diff:
%s`, prDetails.Number, prDetails.Title, prDetails.Author, prDetails.Head, prDetails.Base, diff)
}

func parseIssues(content string) ([]types.Issue, error) {
	// Trim whitespace first
	content = strings.TrimSpace(content)

	// Try direct parse first (in case LLM returns clean JSON without markdown)
	var issues []types.Issue
	if err := json.Unmarshal([]byte(content), &issues); err == nil {
		return issues, nil
	}

	// Try to remove markdown code blocks and parse again
	jsonStr := removeMarkdownCodeBlocks(content)
	jsonStr = strings.TrimSpace(jsonStr)

	if err := json.Unmarshal([]byte(jsonStr), &issues); err == nil {
		return issues, nil
	}

	// Fallback: try to find JSON array in the original content using bracket matching
	// This handles cases where content contains nested backticks
	startIdx := -1
	endIdx := -1
	depth := 0

	for i, c := range content {
		if c == '[' && startIdx == -1 {
			startIdx = i
			depth = 1
		} else if c == '[' && startIdx != -1 {
			depth++
		} else if c == ']' && depth > 0 {
			depth--
			if depth == 0 {
				endIdx = i + 1
				break
			}
		}
	}

	if startIdx != -1 && endIdx != -1 {
		jsonStr = content[startIdx:endIdx]
		if err := json.Unmarshal([]byte(jsonStr), &issues); err == nil {
			return issues, nil
		}
	}

	// Log warning if all parsing methods fail
	log.Printf("[WARN] Failed to parse LLM response after all attempts (length: %d)", len(content))
	return nil, fmt.Errorf("failed to parse LLM response: invalid JSON format")
}

// removeMarkdownCodeBlocks removes only the code block markers (```json and ```)
// and returns the content between them. Returns empty string if content is not valid JSON.
func removeMarkdownCodeBlocks(content string) string {
	// Try to find ```json first, then ```
	var startIdx int
	var marker string

	if idx := strings.Index(content, "```json"); idx != -1 {
		startIdx = idx
		marker = "```json"
	} else if idx := strings.Index(content, "```"); idx != -1 {
		startIdx = idx
		marker = "```"
	} else {
		// No code block found, return empty string to indicate invalid content
		return ""
	}

	// Get content after the opening marker
	afterMarker := content[startIdx+len(marker):]

	// Find the closing ```
	endIdx := strings.Index(afterMarker, "```")
	if endIdx == -1 {
		// No closing marker found, return empty string
		return ""
	}

	// Return only the content between markers
	extracted := afterMarker[:endIdx]

	// Validate that extracted content looks like JSON (starts with [ or {)
	extracted = strings.TrimSpace(extracted)
	if len(extracted) == 0 || (extracted[0] != '[' && extracted[0] != '{') {
		return ""
	}

	return extracted
}
