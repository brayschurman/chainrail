package github

import (
	"context"
	"testing"
)

func TestPullRequestStructFields(t *testing.T) {
	pr := PullRequest{
		Number:         42,
		Title:          "fix the thing",
		BaseRefName:    "main",
		HeadRefName:    "feature/x",
		State:          "OPEN",
		Body:           "body",
		MergeCommitSHA: "",
	}
	if pr.Number != 42 {
		t.Fatalf("Number: got %d, want 42", pr.Number)
	}
	if pr.State != "OPEN" {
		t.Fatalf("State: got %q, want OPEN", pr.State)
	}
	if pr.MergeCommitSHA != "" {
		t.Fatalf("MergeCommitSHA: got %q, want empty", pr.MergeCommitSHA)
	}
}

func TestNewPRStructFields(t *testing.T) {
	p := NewPR{
		Title: "t",
		Body:  "b",
		Head:  "h",
		Base:  "ba",
		Draft: true,
	}
	if !p.Draft {
		t.Fatal("Draft should be true")
	}
	if p.Head != "h" || p.Base != "ba" {
		t.Fatalf("Head/Base mismatch: %+v", p)
	}
}

// Compile-time check that the interface shape is what we expect.
// Any future implementer will be checked the same way in its own package.
type interfaceShape interface {
	CurrentUser(ctx context.Context) (string, error)
	ListOpenPRs(ctx context.Context) ([]PullRequest, error)
	ListAllOpenPRs(ctx context.Context) ([]PullRequest, error)
	ListReviewRequestedPRs(ctx context.Context) ([]PullRequest, error)
	ChangesSinceReview(ctx context.Context) (map[int]int, error)
	ListMergedPRsByHead(ctx context.Context, heads []string) ([]PullRequest, error)
	GetPR(ctx context.Context, number int) (PullRequest, error)
	CreatePR(ctx context.Context, p NewPR) (PullRequest, error)
	UpdatePRBody(ctx context.Context, number int, body string) error
	UpdatePRBase(ctx context.Context, number int, newBase string) error
	UpdatePRTitle(ctx context.Context, number int, newTitle string) error
	PRDiff(ctx context.Context, number int) (string, error)
}

func TestGitHubClientInterfaceShape(t *testing.T) {
	var _ GitHubClient = (interfaceShape)(nil)
}
