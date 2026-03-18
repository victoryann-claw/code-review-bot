package formatter

import "fmt"

type Issue struct {
	Type        string `json:"type"`
	Severity    string `json:"severity"`
	File        string `json:"file,omitempty"`
	Line        int    `json:"line,omitempty"`
	Description string `json:"description"`
	Suggestion  string `json:"suggestion,omitempty"`
}

func FormatReviewComment(issues []Issue) string {
	if len(issues) == 0 {
		return "## 🤖 AI 代码审查

未发现问题，代码质量良好！🎉"
	}

	var high, medium, low []Issue
	for _, issue := range issues {
		switch issue.Severity {
		case "high":
			high = append(high, issue)
		case "medium":
			medium = append(medium, issue)
		case "low":
			low = append(low, issue)
		default:
			low = append(low, issue)
		}
	}

	reviewResult := "可以合并 ✅"
	if len(high) > 0 || len(medium) > 0 {
		reviewResult = "需要修改"
	}

	comment := "## 🤖 AI 代码审查

"
	comment += fmt.Sprintf("发现 %d 个问题：%d 个高危，%d 个中危，%d 个低危

", 
		len(issues), len(high), len(medium), len(low))

	if len(high) > 0 {
		comment += "### 🔴 高危问题

"
		for i, issue := range high {
			comment += formatIssue(issue, i+1)
		}
		comment += "
"
	}

	if len(medium) > 0 {
		comment += "### 🟡 中危问题

"
		startIdx := len(high) + 1
		for i, issue := range medium {
			comment += formatIssue(issue, startIdx+i)
		}
		comment += "
"
	}

	if len(low) > 0 {
		comment += "### 🟢 低危问题

"
		startIdx := len(high) + len(medium) + 1
		for i, issue := range low {
			comment += formatIssue(issue, startIdx+i)
		}
	}

	comment += fmt.Sprintf("
---
**审查结果**: %s

*此审查由AI生成，请在应用前验证建议。*", reviewResult)

	return comment
}

func formatIssue(issue Issue, index int) string {
	formatted := fmt.Sprintf("%d. [%s] ", index, issue.Type)

	if issue.File != "" {
		formatted += fmt.Sprintf("文件: `%s`", issue.File)
		if issue.Line > 0 {
			formatted += fmt.Sprintf(":%d", issue.Line)
		}
		formatted += "
"
	} else {
		formatted += "
"
	}

	formatted += fmt.Sprintf("   问题描述: %s
", issue.Description)

	if issue.Suggestion != "" {
		formatted += fmt.Sprintf("   建议修复方式: %s
", issue.Suggestion)
	}

	return formatted + "
"
}

func getIcon(issueType string) string {
	icons := map[string]string{
		"bug":         "🐛",
		"security":    "🔒",
		"performance": "⚡",
		"style":       "🎨",
		"suggestion":  "💡",
	}
	if icon, ok := icons[issueType]; ok {
		return icon
	}
	return "📝"
}
