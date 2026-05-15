package github

import "context"

type GitHubClient interface {
	CurrentUser(ctx context.Context) (string, error)
	ListOpenPRs(ctx context.Context) ([]PullRequest, error)
	// ListAllOpenPRs returns all open PRs in the repo regardless of author.
	// Used by cn status --all to build the full PR dependency graph.
	ListAllOpenPRs(ctx context.Context) ([]PullRequest, error)
	// ListReviewRequestedPRs returns open PRs where the current user is a
	// requested reviewer. Used by the Review tab.
	ListReviewRequestedPRs(ctx context.Context) ([]PullRequest, error)
	// ChangesSinceReview returns a map keyed by PR number of how many commits
	// have landed on each PR after the current user's most recent review.
	// Only includes PRs the user has actually reviewed; PRs absent from the
	// map should be treated as "no prior review."
	ChangesSinceReview(ctx context.Context) (map[int]int, error)
	// ListMergedPRsByHead returns at most one merged PR per head (the most recent
	// by number when there are duplicates). Heads that have no merged PR are
	// simply absent from the result. Used by sync's squash-merged-parent detection.
	ListMergedPRsByHead(ctx context.Context, heads []string) ([]PullRequest, error)
	GetPR(ctx context.Context, number int) (PullRequest, error)
	CreatePR(ctx context.Context, p NewPR) (PullRequest, error)
	UpdatePRBody(ctx context.Context, number int, body string) error
	UpdatePRBase(ctx context.Context, number int, newBase string) error
	UpdatePRTitle(ctx context.Context, number int, newTitle string) error
	// PRDiff returns the unified diff for a PR as a single string,
	// equivalent to `gh pr diff <number> --patch`.
	PRDiff(ctx context.Context, number int) (string, error)
	// PRFiles returns each file in a PR along with its blob SHA after the
	// change. Used for content-hash review state.
	PRFiles(ctx context.Context, number int) ([]PRFile, error)
	// RepoInfo returns the owner/name of the current repo, derived from the
	// `gh` CLI's repo resolution (i.e. respects $GH_REPO or the local
	// remote).
	RepoInfo(ctx context.Context) (owner, name string, err error)
}

// PRFile is one file's metadata from a PR — enough to drive per-file
// review state and basic diff summaries.
type PRFile struct {
	Path      string
	BlobSHA   string
	Additions int
	Deletions int
}

type PullRequest struct {
	Number         int
	Title          string
	BaseRefName    string
	HeadRefName    string
	State          string
	Body           string
	MergeCommitSHA string
	// CIStatus is the aggregate status check rollup: SUCCESS, FAILURE,
	// PENDING, or "" (no checks configured).
	CIStatus string
	// ReviewDecision is APPROVED, CHANGES_REQUESTED, REVIEW_REQUIRED, or "".
	ReviewDecision string
	// UpdatedAt is the RFC3339 string from gh's updatedAt field.
	UpdatedAt string
}

type NewPR struct {
	Title string
	Body  string
	Head  string
	Base  string
	Draft bool
}
