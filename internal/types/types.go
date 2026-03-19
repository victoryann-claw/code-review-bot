package types

// Issue represents a code review issue
type Issue struct {
	Type        string `json:"type"`
	Severity    string `json:"severity"`
	File        string `json:"file,omitempty"`
	Line        int    `json:"line,omitempty"`
	Description string `json:"description"`
	Suggestion  string `json:"suggestion,omitempty"`
}

// PRDetails holds pull request details
type PRDetails struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	Body   string `json:"body"`
	Head   string `json:"head"`
	Base   string `json:"base"`
	Author string `json:"author"`
	URL    string `json:"url"`
	Owner  string `json:"owner"`
	Repo   string `json:"repo"`
}
