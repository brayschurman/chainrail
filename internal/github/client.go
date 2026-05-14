package github

import "context"

type GitHubClient interface {
	CurrentUser(ctx context.Context) (string, error)
	ListOpenPRs(ctx context.Context) ([]PullRequest, error)
	GetPR(ctx context.Context, number int) (PullRequest, error)
	CreatePR(ctx context.Context, p NewPR) (PullRequest, error)
	UpdatePRBody(ctx context.Context, number int, body string) error
	UpdatePRBase(ctx context.Context, number int, newBase string) error
}

type PullRequest struct {
	Number         int
	Title          string
	BaseRefName    string
	HeadRefName    string
	State          string
	Body           string
	MergeCommitSHA string
}

type NewPR struct {
	Title string
	Body  string
	Head  string
	Base  string
	Draft bool
}
