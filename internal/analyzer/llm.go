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
	
	model := os.Getenv("OPENAI_MODEL")
	if model == "" {
		model = os.Getenv("MINIMAX_MODEL")
		if model == "" {
			model = "gpt-4"
		}
	}
	
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("MINIMAX_API_KEY")
	}
	
	baseURL := os.Getenv("LLM_BASE_URL")

	cfg := openai.DefaultConfig(apiKey)
	if baseURL != "" {
		cfg.BaseURL = baseURL
	} else if provider == "minimax" {
		cfg.BaseURL = "https://api.minimax.chat/v1"
		if model == "gpt-4" {
			model = "MiniMax-Text-01"
		}
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
	fmt.Printf("[DEBUG] Analyzing code with LLM (%s)\n", a.model)
	
	systemPrompt := `You are an expert code reviewer. Analyze the following GitHub pull request diff and identify potential issues, bugs, security vulnerabilities, code quality problems, or suggestions for improvement.

For each issue found, respond with a JSON array of objects with these fields:
- type: "bug", "security", "performance", "style", "suggestion"
- severity: "high", "medium", "low"
- file: filename (if applicable)
- line: line number (if applicable)
- description: brief description of the issue
- suggestion: how to fix or improve

Respond ONLY with a valid JSON array. If no issues found, return an empty array [].`

	userPrompt := buildUserPrompt(diff, prDetails)

	resp, err := a.client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
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
		},
	)
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
