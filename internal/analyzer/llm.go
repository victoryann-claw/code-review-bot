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

请用中文回复。description（问题描述）和 suggestion（修复建议）必须使用中文。

**重要：JSON 键名必须保持英文**，否则后端无法解析。返回格式如下：
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
		log.Printf("[DEBUG] Failed to parse LLM response: %v", err)
		return []types.Issue{}, nil
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
	// Remove markdown code blocks, keep content
	jsonStr := removeMarkdownCodeBlocks(content)
	
	// Trim whitespace
	jsonStr = strings.TrimSpace(jsonStr)
	
	// Try direct parse first
	var issues []types.Issue
	if err := json.Unmarshal([]byte(jsonStr), &issues); err == nil {
		return issues, nil
	}
	
	// Try to find JSON array in the original content
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
	log.Printf("[WARN] Failed to parse LLM response (length: %d), trying alternative extraction", len(content))
	return []types.Issue{}, nil
}

// removeMarkdownCodeBlocks removes only the code block markers (```json and ```)
// and returns the content between them
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
		// No code block found, return original content
		return content
	}
	
	// Get content after the opening marker
	afterMarker := content[startIdx+len(marker):]
	
	// Find the closing ```
	endIdx := strings.Index(afterMarker, "```")
	if endIdx == -1 {
		// No closing marker found, return content after opening marker
		return afterMarker
	}
	
	// Return only the content between markers
	return afterMarker[:endIdx]
}
