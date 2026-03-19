package analyzer

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/sashabaranov/go-openai"
	"github.com/victoryann-claw/code-review-bot/internal/types"
)

// Issue deduplication state - persists across reviews for the same PR
var (
	seenIssuesMu sync.Mutex
	seenIssues   = make(map[string]map[string]bool)
)

// ClearSeenIssues clears the deduplication cache for a PR when action=opened or merged
func ClearSeenIssues(owner, repo string, prNumber int) {
	key := fmt.Sprintf("%s/%s/pr-%d", owner, repo, prNumber)
	seenIssuesMu.Lock()
	defer seenIssuesMu.Unlock()
	delete(seenIssues, key)
	log.Printf("[DEBUG] Cleared seen issues cache for %s", key)
}

// ClearAllSeenIssues clears all deduplication cache (for testing)
func ClearAllSeenIssues() {
	seenIssuesMu.Lock()
	defer seenIssuesMu.Unlock()
	seenIssues = make(map[string]map[string]bool)
	log.Printf("[DEBUG] Cleared all seen issues cache")
}

// hashIssue creates a stable hash key for issue deduplication
func hashIssue(file string, line int, description string) string {
	// Normalize description: lowercase, remove extra spaces
	desc := strings.ToLower(description)
	desc = strings.Join(strings.Fields(desc), " ")
	h := sha256.New()
	h.Write([]byte(fmt.Sprintf("%s:%d:%s", file, line, desc)))
	return fmt.Sprintf("%x", h.Sum(nil))[:16] // Use first 16 chars
}

// deduplicateIssues removes duplicate issues based on file+line+description hash
func deduplicateIssues(issues []types.Issue, owner, repo string, prNumber int) []types.Issue {
	if len(issues) == 0 {
		return issues
	}
	
	key := fmt.Sprintf("%s/%s/pr-%d", owner, repo, prNumber)
	
	seenIssuesMu.Lock()
	defer seenIssuesMu.Unlock()
	
	if seenIssues[key] == nil {
		seenIssues[key] = make(map[string]bool)
	}
	
	var unique []types.Issue
	for _, issue := range issues {
		h := hashIssue(issue.File, issue.Line, issue.Description)
		if !seenIssues[key][h] {
			seenIssues[key][h] = true
			unique = append(unique, issue)
		}
	}
	
	if len(unique) < len(issues) {
		log.Printf("[DEBUG] Deduplicated %d issues down to %d", len(issues), len(unique))
	}
	return unique
}

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
		temperature: 0.0,
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

**🚫 严格禁止的审查行为：**
- 不要报告"可能"存在的问题，只能报告确定的问题
- 不要因为无法验证其他部分就假设有问题
- diff 中没有明确显示的错误，不要报告
- 如果代码能正常编译运行，且没有明显逻辑错误，应该判定为"无问题"
- **特别是**：如果你在 diff 中看到了函数的完整实现（包括 strings.Index 等函数调用），不要报告"空指针"、"未验证"等推测性问题

**🔴 Git Diff 语境强制规则：**

1. **识别 Diff 语境**：你处理的是 Git Diff 片段，不是完整文件。看到代码截断是正常现象，不代表代码有问题。

2. **禁止推断缺失**：严禁报告由于 Diff 截断导致的"缺少括号"、"缺少 Return"、"变量未定义"等问题。Diff 显示的是变更部分，不是完整文件。

3. **只看新增行**：仅针对带有 `+` 标记的新增代码行进行逻辑审查。不要对 `-` 删除行或上下文缺失的部分进行推测。

4. **证据先行**：在指出语法错误前，确认该错误是否真的出现在新增行内。如果是由于上下文缺失导致的疑似错误，**直接忽略**。

5. **代码完整性判断**：如果一个函数在 Diff 中有开始但没有明显的结束标记，不要报告"代码不完整"。Diff 截断不代表函数不完整。

**📋 硬性规则 - 绝对禁止违反：**

1. **不讨论代码设计选择**：对于"返回 nil vs 返回空数组"、"用 log 还是 fmt"这种设计选择问题，视为已解决，不要再报告
2. **不重复报告同一问题**：同一个问题之前报告过且代码未改变，不要再次报告
3. **不接受"可能"的判断**：只能说"确定有问题"，不能说"可能有问题"
4. **API 兼容性由调用方决定**：如 ResponseFormat 支持问题，不是你该管的
5. **diff 截断由调用方处理**：如 10MB 限制问题，不是你该报告的

**只有以下问题值得报告：**
- 语法错误（代码明显无法编译）
- 逻辑错误（if/else 分支明显错误）
- 安全漏洞（硬编码密码、SQL 注入等）
- 明显的空指针/越界访问（diff 中明确显示）
- 功能性 bug（功能实现明显错误）

**🔍 存在性检查（Grounding）- 强制执行：**
当你报告"缺少函数"、"函数未定义"、"调用未定义的函数"等问题时：
1. 先列出你在 diff 中搜索该关键词的结果
2. 如果搜索结果不为空（即函数确实在 diff 中存在），则**禁止**报告此问题
3. 示例：
   - ❌ 错误："removeMarkdownCodeBlocks 函数未定义"
   - ✅ 正确：先搜索"removeMarkdownCodeBlocks"，如果找到定义，则不报告

**🔄 自我验证（Self-Correction）- 强制执行：**
在完成审查输出之前，增加一步验证：
1. 重新检查你发现的每一个问题
2. 确认"这个问题在 diff 中确实存在"，而不是"我猜测可能有问题"
3. 如果发现任何不确定的问题，**直接删除**，不要报告
4. 宁可漏报，不要误报

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
		log.Printf("[WARN] Failed to parse LLM response: %v", err)
		// Return empty issues instead of nil to avoid nil pointer dereference
		return []types.Issue{}, nil
	}

	log.Printf("[DEBUG] LLM found %d issues", len(issues))
	
	// Apply deduplication - issues are deduplicated per PR (using owner+repo+prNumber as key)
	// Skip deduplication if prDetails is nil or missing owner/repo info
	var uniqueIssues []types.Issue
	if prDetails != nil && prDetails.Owner != "" && prDetails.Repo != "" {
		uniqueIssues = deduplicateIssues(issues, prDetails.Owner, prDetails.Repo, prDetails.Number)
	} else {
		uniqueIssues = issues
		log.Printf("[WARN] Skipping deduplication due to missing prDetails info")
	}
	
	return uniqueIssues, nil
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
	return []types.Issue{}, nil
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
		// No code block found - return original content for direct JSON parsing
		return content
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
