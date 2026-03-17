package handler

import (
	"bytes"
	"context"
	"io"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/victoryann-claw/code-review-bot/internal/analyzer"
	"github.com/victoryann-claw/code-review-bot/internal/config"
	"github.com/victoryann-claw/code-review-bot/internal/formatter"
	"github.com/victoryann-claw/code-review-bot/internal/github"
	"github.com/victoryann-claw/code-review-bot/internal/types"
)

// GitHubEvent represents a GitHub webhook event
type GitHubEvent struct {
	Action      string `json:"action"`
	PullRequest struct {
		Number int `json:"number"`
		Title  string `json:"title"`
		Body   string `json:"body"`
		Head   struct {
			Ref string `json:"ref"`
		} `json:"head"`
		Base struct {
			Ref string `json:"ref"`
		} `json:"base"`
		User struct {
			Login string `json:"login"`
		} `json:"user"`
		HTMLURL string `json:"html_url"`
	} `json:"pull_request"`
	Repository struct {
		Owner struct {
			Login string `json:"login"`
		} `json:"owner"`
		Name string `json:"name"`
	} `json:"repository"`
}

// HandleWebhook handles GitHub webhook requests
func HandleWebhook(c *gin.Context) {
	// Get raw body for signature validation
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Printf("Error reading request body: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	// Restore the body for later use
	c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	rawBody := string(bodyBytes)

	cfg := config.GetConfig()
	secret := ""
	if cfg != nil {
		secret = cfg.GitHub.WebhookSecret
	}
	if secret == "" {
		secret = GetEnvOrDefault("GITHUB_WEBHOOK_SECRET", "")
	}

	// Validate signature if secret is configured
	if secret != "" {
		signature := GetSignature(map[string]string{
			"x-hub-signature-256": c.GetHeader("X-Hub-Signature-256"),
			"x-hub-signature":     c.GetHeader("X-Hub-Signature"),
		})
		
		if !ValidateSignature(signature, []byte(rawBody), secret) {
			log.Println("Invalid webhook signature")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid signature"})
			return
		}
	}

	event := c.GetHeader("X-GitHub-Event")
	DebugLog("Received event: %s", event)

	// Only process pull request events
	if event != "pull_request" {
		c.JSON(http.StatusOK, gin.H{"message": "Event ignored"})
		return
	}

	// Parse webhook payload
	var eventPayload GitHubEvent
	if err := c.ShouldBindJSON(&eventPayload); err != nil {
		log.Printf("Error parsing webhook payload: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload"})
		return
	}

	action := eventPayload.Action

	// Only process when PR is opened or synchronize (new commits)
	if action != "opened" && action != "synchronize" {
		DebugLog("Action '%s' ignored", action)
		c.JSON(http.StatusOK, gin.H{"message": "Action ignored"})
		return
	}

	pr := eventPayload.PullRequest
	repo := eventPayload.Repository
	owner := repo.Owner.Login
	repoName := repo.Name
	prNumber := pr.Number

	DebugLog("Processing PR #%d from %s/%s", prNumber, owner, repoName)

	// Get GitHub token
	token := ""
	if cfg != nil {
		token = cfg.GitHub.Token
	}
	if token == "" {
		token = GetEnvOrDefault("GITHUB_TOKEN", "")
	}

	if token == "" {
		log.Println("GitHub token not configured")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "GitHub token not configured"})
		return
	}

	// Create GitHub client
	ghClient := github.NewGitHubClient(token)
	ctx := context.Background()

	// Get PR details and diff
	prDetails, err := ghClient.GetPullRequestDetails(ctx, owner, repoName, prNumber)
	if err != nil {
		log.Printf("Error getting PR details: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get PR details"})
		return
	}

	diff, err := ghClient.GetPullRequestDiff(ctx, owner, repoName, prNumber)
	if err != nil {
		log.Printf("Error getting PR diff: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get PR diff"})
		return
	}

	if diff == "" {
		DebugLog("No diff found, skipping review")
		c.JSON(http.StatusOK, gin.H{"message": "No changes to review"})
		return
	}

	// Analyze code with LLM
	DebugLog("Analyzing code with LLM...")
	llmAnalyzer := analyzer.NewLLMAnalyzer()
	issues, err := llmAnalyzer.AnalyzeCode(ctx, diff, &types.PRDetails{
		Number: prDetails.Number,
		Title:  prDetails.Title,
		Body:   prDetails.Body,
		Head:   prDetails.Head,
		Base:   prDetails.Base,
		Author: prDetails.Author,
		URL:    prDetails.URL,
	})
	if err != nil {
		log.Printf("Error analyzing code: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to analyze code"})
		return
	}

	if len(issues) == 0 {
		DebugLog("No issues found by AI")
		c.JSON(http.StatusOK, gin.H{"message": "No issues found"})
		return
	}

	// Format and post review comment
	DebugLog("Posting %d review comments...", len(issues))
	
	// Convert types.Issue to formatter.Issue
	var formIssues []formatter.Issue
	for _, issue := range issues {
		formIssues = append(formIssues, formatter.Issue{
			Type:        issue.Type,
			Severity:    issue.Severity,
			File:        issue.File,
			Line:        issue.Line,
			Description: issue.Description,
			Suggestion:  issue.Suggestion,
		})
	}
	
	comment := formatter.FormatReviewComment(formIssues)

	_, err = ghClient.CreateReviewComment(ctx, owner, repoName, prNumber, comment)
	if err != nil {
		log.Printf("Error creating review comment: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create comment"})
		return
	}

	DebugLog("Review completed successfully")
	c.JSON(http.StatusOK, gin.H{"success": true, "comments": len(issues)})
}
