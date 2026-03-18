package analyzer

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

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

请用中文回复。返回的JSON内容中的所有字段（type、severity、description、suggestion等）都应该是中文。

请为每个发现的问题返回一个 JSON 对象数组，包含以下字段：
- type: 问题类型，如 "bug"（缺陷）、"security"（安全）、"performance"（性能）、"style"（代码风格）、"suggestion"（建议）
- severity: 严重程度，如 "high"（高）、"medium"（中）、"low"（低）
- file: 文件名（如适用）
- line: 行号（如适用）
- description: 问题的简要描述（必须使用中文）
- suggestion: 如何修复或改进（必须使用中文）

请只返回一个有效的 JSON 数组。如果没有发现问题，返回空数组 []。`

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
	// Trim whitespace
	for len(jsonStr) > 0 && (jsonStr[0] == ' ' || jsonStr[0] == '\n' || jsonStr[0] == '\r' || jsonStr[0] == '\t') {
		jsonStr = jsonStr[1:]
	}
	for len(jsonStr) > 0 && (jsonStr[len(jsonStr)-1] == ' ' || jsonStr[len(jsonStr)-1] == '\n' || jsonStr[len(jsonStr)-1] == '\r' || jsonStr[len(jsonStr)-1] == '\t') {
		jsonStr = jsonStr[:len(jsonStr)-1]
	}
	
	// Try direct parse first
	var issues []types.Issue
	if err := json.Unmarshal([]byte(jsonStr), &issues); err == nil {
		return issues, nil
	}
	
	// Try to find JSON array in the content
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
