package github

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/google/go-github/v63/github"
	"github.com/victoryann-claw/code-review-bot/internal/types"
)

// GitHubClient wraps the GitHub API client
type GitHubClient struct {
	client *github.Client
	token  string
}

// NewGitHubClient creates a new GitHub client
func NewGitHubClient(token string) *GitHubClient {
	client := github.NewClient(nil)
	if token != "" {
		client = client.WithAuthToken(token)
	}
	return &GitHubClient{
		client: client,
		token:  token,
	}
}

// GetPullRequestDetails gets pull request details
func (g *GitHubClient) GetPullRequestDetails(ctx context.Context, owner, repo string, prNumber int) (*types.PRDetails, error) {
	log.Printf("[DEBUG] Fetching PR details for %s/%s #%d", owner, repo, prNumber)
	
	pr, _, err := g.client.PullRequests.Get(ctx, owner, repo, prNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get pull request: %w", err)
	}

	return &types.PRDetails{
		Number: pr.GetNumber(),
		Title:  pr.GetTitle(),
		Body:   pr.GetBody(),
		Head:   pr.GetHead().GetRef(),
		Base:   pr.GetBase().GetRef(),
		Author: pr.GetUser().GetLogin(),
		URL:    pr.GetHTMLURL(),
	}, nil
}

// GetPullRequestDiff gets pull request diff
func (g *GitHubClient) GetPullRequestDiff(ctx context.Context, owner, repo string, prNumber int) (string, error) {
	log.Printf("[DEBUG] Fetching PR diff for %s/%s #%d", owner, repo, prNumber)
	
	// Use GitHub's raw diff endpoint
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls/%d.diff", owner, repo, prNumber)
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	
	if g.token != "" {
		req.Header.Set("Authorization", "Bearer "+g.token)
	}
	req.Header.Set("Accept", "application/vnd.github.v3.diff")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch diff: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch diff: status %d", resp.StatusCode)
	}
	
	buf := make([]byte, resp.ContentLength+1024)
	n, err := resp.Body.Read(buf)
	if err != nil && err.Error() != "EOF" {
		return "", fmt.Errorf("failed to read diff: %w", err)
	}
	
	return string(buf[:n]), nil
}

// CreateReviewComment creates a comment on the pull request
func (g *GitHubClient) CreateReviewComment(ctx context.Context, owner, repo string, prNumber int, body string) (*github.IssueComment, error) {
	log.Printf("[DEBUG] Creating review comment for %s/%s #%d", owner, repo, prNumber)
	
	comment, _, err := g.client.Issues.CreateComment(ctx, owner, repo, prNumber, &github.IssueComment{
		Body: github.String(body),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create comment: %w", err)
	}

	return comment, nil
}

// CreatePullRequestReview creates a review with inline comments
func (g *GitHubClient) CreatePullRequestReview(ctx context.Context, owner, repo string, prNumber int, body string, comments []*github.DraftReviewComment) (*github.PullRequestReview, error) {
	log.Printf("[DEBUG] Creating PR review for %s/%s #%d", owner, repo, prNumber)
	
	review := &github.PullRequestReviewRequest{
		Body:     github.String(body),
		Event:   github.String("COMMENT"),
		Comments: comments,
	}
	
	result, _, err := g.client.PullRequests.CreateReview(ctx, owner, repo, prNumber, review)
	if err != nil {
		return nil, fmt.Errorf("failed to create review: %w", err)
	}
	
	return result, nil
}
