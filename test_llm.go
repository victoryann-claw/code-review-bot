package main

import (
    "context"
    "fmt"
    "os"
    
    "github.com/victoryann-claw/code-review-bot/internal/analyzer"
    "github.com/victoryann-claw/code-review-bot/internal/types"
)

func main() {
    os.Setenv("LLM_PROVIDER", "bailian")
    os.Setenv("DASHSCOPE_API_KEY", "sk-02535bf5da7040bf8cdc7965c19825c8")
    os.Setenv("DASHSCOPE_MODEL", "qwen3.5-plus")
    
    llm := analyzer.NewLLMAnalyzer()
    
    diff := `--- a/src/index.js
+++ b/src/index.js
@@ -1,3 +1,5 @@
+import something from 'evil';
+eval(userInput);
 const hello = 'world';
+var x = 1;`
    
    prDetails := &types.PRDetails{
        Number:  1,
        Title:   "Test PR",
        Author:  "testuser",
        Head:    "feature",
        Base:    "main",
    }
    
    issues, err := llm.AnalyzeCode(context.Background(), diff, prDetails)
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }
    
    fmt.Printf("Found %d issues:\n", len(issues))
    for _, issue := range issues {
        fmt.Printf("- [%s] %s: %s\n", issue.Severity, issue.Type, issue.Description)
        if issue.Suggestion != "" {
            fmt.Printf("  Suggestion: %s\n", issue.Suggestion)
        }
    }
}
