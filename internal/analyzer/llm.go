package analyzer

import (
	"context"
	"encoding/json"
	"fmt"
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
	fmt.Printf("[DEBUG] Analyzing code with LLM (%s, provider: %s)\n", a.model, a.provider)

	systemPrompt := `你是一位资深的代码审查专家。请分析以下 GitHub Pull Request 的代码差异，识别潜在的问题、bug、安全漏洞、代码质量问题或改进建议。

请用中文回复。description（问题描述）和 suggestion（修复建议）必须使用中文。

重要：JSON 键名必须保持英文，否则后端无法解析。返回格式如下：
- type: 问题类型，如 "bug"、"security"、"performance"、"style"、"suggestion"（必须英文）
- severity: 严重程度，如 "high"、"medium"、"low"（必须英文）
- file: 文件名（如适用）
- line: 行号（如适用）
- description: 问题的简要描述（必须使用中文）
- suggestion: 如何修复或改进（必须使用中文）

请只返回一个有效的 JSON 数组。如果没有发现问题，返回空数组 []。

响应示例（不要包含 markdown 代码块标记，直接返回 JSON）：
[
  {
    "type": "bug",
    "severity": "high",
    "file": "internal/handler/webhook.go",
    "line": 42,
    "description": "空指针解引用风险：user 变量可能为 nil",
    "suggestion": "在使用前检查 user 是否为 nil"
  },
  {
    "type": "style",
    "severity": "low",
    "description": "代码格式不规范，建议使用 gofmt 格式化",
    "suggestion": "运行 gofmt -w . 格式化代码"
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

	// Add response format for Bailian
	if a.provider == "bailian" || a.provider == "aliyun" || a.provider == "qwen" {
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
		fmt.Printf("[DEBUG] Failed to parse LLM response: %v\n", err)
		return []types.Issue{}, nil
	}

	fmt.Printf("[DEBUG] LLM found %d issues\n", len(issues))
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
	// Try to extract JSON from markdown code blocks
	start := 0
	end := len(content)

	// Check for markdown code block
	if idx := findAnyIndex(content, "```json", "```"); idx != -1 {
		start = idx + len("```json")
		if endIdx := findAnyIndex(content[start:], "```"); endIdx != -1 {
			end = start + endIdx
		}
	}

	jsonStr := content[start:end]
	// Use standard library to trim whitespace
	jsonStr = strings.TrimSpace(jsonStr)

	// Try direct parse first
	var issues []types.Issue
	if err := json.Unmarshal([]byte(jsonStr), &issues); err == nil {
		return issues, nil
	}

	// Fallback: search for JSON array in the original content (for unrecognized markdown formats)
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
	fmt.Printf("[WARN] Failed to parse LLM response as JSON, content: %s\n", content)
	return []types.Issue{}, nil
}

func findAnyIndex(s string, substrs ...string) int {
	for _, substr := range substrs {
		if idx := findIndex(s, substr); idx != -1 {
			return idx
		}
	}
	return -1
}

func findIndex(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
