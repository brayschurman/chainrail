package github

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	crerrors "github.com/brayschurman/chainrail/internal/errors"
)

type runner func(name string, args ...string) ([]byte, error)

type GhCli struct {
	run runner
}

func New() *GhCli {
	return &GhCli{run: defaultRun}
}

func defaultRun(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).Output()
}

func (c *GhCli) CurrentUser(_ context.Context) (string, error) {
	out, err := c.run("gh", "api", "user", "--jq", ".login")
	if err != nil {
		return "", wrapGhErr(err, "gh api user")
	}
	return strings.TrimSpace(string(out)), nil
}

type ghPRRaw struct {
	Number      int    `json:"number"`
	Title       string `json:"title"`
	BaseRefName string `json:"baseRefName"`
	HeadRefName string `json:"headRefName"`
	State       string `json:"state"`
	Body        string `json:"body"`
	MergeCommit *struct {
		OID string `json:"oid"`
	} `json:"mergeCommit"`
	StatusCheckRollup []struct {
		Status     string `json:"status"`
		Conclusion string `json:"conclusion"`
		State      string `json:"state"`
	} `json:"statusCheckRollup"`
	ReviewDecision string `json:"reviewDecision"`
	UpdatedAt      string `json:"updatedAt"`
}

func (r ghPRRaw) toPR() PullRequest {
	pr := PullRequest{
		Number:         r.Number,
		Title:          r.Title,
		BaseRefName:    r.BaseRefName,
		HeadRefName:    r.HeadRefName,
		State:          r.State,
		Body:           r.Body,
		CIStatus:       rollupCIStatus(r.StatusCheckRollup),
		ReviewDecision: r.ReviewDecision,
		UpdatedAt:      r.UpdatedAt,
	}
	if r.MergeCommit != nil {
		pr.MergeCommitSHA = r.MergeCommit.OID
	}
	return pr
}

// rollupCIStatus collapses gh's per-check statusCheckRollup array into a
// single value: FAILURE if any check failed, PENDING if any is still running,
// SUCCESS if all completed successfully, "" if no checks are configured.
func rollupCIStatus(checks []struct {
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	State      string `json:"state"`
}) string {
	if len(checks) == 0 {
		return ""
	}
	anyPending := false
	anyFail := false
	for _, c := range checks {
		// Check runs use Status (COMPLETED/IN_PROGRESS/QUEUED) + Conclusion
		// (SUCCESS/FAILURE/...). Status contexts use State (SUCCESS/FAILURE/
		// PENDING/ERROR).
		status := strings.ToUpper(c.Status)
		concl := strings.ToUpper(c.Conclusion)
		state := strings.ToUpper(c.State)
		if status != "" && status != "COMPLETED" {
			anyPending = true
			continue
		}
		if state == "PENDING" {
			anyPending = true
			continue
		}
		if concl == "FAILURE" || concl == "TIMED_OUT" || concl == "CANCELLED" || concl == "ACTION_REQUIRED" {
			anyFail = true
		}
		if state == "FAILURE" || state == "ERROR" {
			anyFail = true
		}
	}
	if anyFail {
		return "FAILURE"
	}
	if anyPending {
		return "PENDING"
	}
	return "SUCCESS"
}

const prJSONFields = "number,title,baseRefName,headRefName,state,body,mergeCommit,statusCheckRollup,reviewDecision,updatedAt"

func (c *GhCli) ListOpenPRs(_ context.Context) ([]PullRequest, error) {
	out, err := c.run("gh", "pr", "list",
		"--author", "@me",
		"--state", "open",
		"--json", prJSONFields,
	)
	if err != nil {
		return nil, wrapGhErr(err, "gh pr list")
	}
	var raw []ghPRRaw
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("parse gh pr list: %w", err)
	}
	prs := make([]PullRequest, len(raw))
	for i, r := range raw {
		prs[i] = r.toPR()
	}
	return prs, nil
}

func (c *GhCli) ListAllOpenPRs(_ context.Context) ([]PullRequest, error) {
	out, err := c.run("gh", "pr", "list",
		"--state", "open",
		"--limit", "100",
		"--json", prJSONFields,
	)
	if err != nil {
		return nil, wrapGhErr(err, "gh pr list --state open")
	}
	var raw []ghPRRaw
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("parse gh pr list: %w", err)
	}
	prs := make([]PullRequest, len(raw))
	for i, r := range raw {
		prs[i] = r.toPR()
	}
	return prs, nil
}

func (c *GhCli) ListReviewRequestedPRs(_ context.Context) ([]PullRequest, error) {
	out, err := c.run("gh", "pr", "list",
		"--search", "is:open review-requested:@me",
		"--limit", "100",
		"--json", prJSONFields,
	)
	if err != nil {
		return nil, wrapGhErr(err, "gh pr list --search review-requested:@me")
	}
	var raw []ghPRRaw
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("parse gh pr list (review-requested): %w", err)
	}
	prs := make([]PullRequest, len(raw))
	for i, r := range raw {
		prs[i] = r.toPR()
	}
	return prs, nil
}

// ChangesSinceReview fetches the open PRs the current user has reviewed and
// counts commits that landed after each review. Returns a map keyed by PR
// number. PRs the user hasn't reviewed are absent from the map.
//
// Implementation: a single search to enumerate reviewed PRs, then per-PR
// fetches of reviews + commits. Capped at 50 PRs to bound the gh fanout.
func (c *GhCli) ChangesSinceReview(_ context.Context) (map[int]int, error) {
	user, err := c.CurrentUser(context.Background())
	if err != nil {
		return nil, err
	}

	listOut, err := c.run("gh", "pr", "list",
		"--search", "reviewed-by:@me is:open",
		"--limit", "50",
		"--json", "number",
	)
	if err != nil {
		return nil, wrapGhErr(err, "gh pr list --search reviewed-by:@me")
	}
	var numbers []struct {
		Number int `json:"number"`
	}
	if err := json.Unmarshal(listOut, &numbers); err != nil {
		return nil, fmt.Errorf("parse gh pr list (reviewed-by): %w", err)
	}

	out := make(map[int]int, len(numbers))
	for _, n := range numbers {
		count, ok := c.commitsAfterUserReview(n.Number, user)
		if ok && count > 0 {
			out[n.Number] = count
		}
	}
	return out, nil
}

// commitsAfterUserReview returns the number of commits on PR `num` whose
// committedDate is strictly after the most recent review by `user`. The bool
// is false on parse/fetch error — callers treat that as "skip this PR."
func (c *GhCli) commitsAfterUserReview(num int, user string) (int, bool) {
	out, err := c.run("gh", "pr", "view", strconv.Itoa(num),
		"--json", "reviews,commits",
	)
	if err != nil {
		return 0, false
	}
	var raw struct {
		Reviews []struct {
			Author      struct{ Login string } `json:"author"`
			SubmittedAt string                 `json:"submittedAt"`
		} `json:"reviews"`
		Commits []struct {
			CommittedDate string `json:"committedDate"`
		} `json:"commits"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return 0, false
	}

	var latest time.Time
	for _, r := range raw.Reviews {
		if r.Author.Login != user {
			continue
		}
		t, err := time.Parse(time.RFC3339, r.SubmittedAt)
		if err != nil {
			continue
		}
		if t.After(latest) {
			latest = t
		}
	}
	if latest.IsZero() {
		return 0, false
	}

	count := 0
	for _, c := range raw.Commits {
		t, err := time.Parse(time.RFC3339, c.CommittedDate)
		if err != nil {
			continue
		}
		if t.After(latest) {
			count++
		}
	}
	return count, true
}

func (c *GhCli) ListMergedPRsByHead(_ context.Context, heads []string) ([]PullRequest, error) {
	out := make([]PullRequest, 0, len(heads))
	for _, head := range heads {
		raw, err := c.run("gh", "pr", "list",
			"--head", head,
			"--state", "merged",
			"--limit", "1",
			"--json", prJSONFields,
		)
		if err != nil {
			return nil, wrapGhErr(err, "gh pr list --head "+head+" --state merged")
		}
		var prs []ghPRRaw
		if err := json.Unmarshal(raw, &prs); err != nil {
			return nil, fmt.Errorf("parse gh pr list --head: %w", err)
		}
		if len(prs) == 0 {
			continue
		}
		out = append(out, prs[0].toPR())
	}
	return out, nil
}

func (c *GhCli) GetPR(_ context.Context, number int) (PullRequest, error) {
	out, err := c.run("gh", "pr", "view", strconv.Itoa(number),
		"--json", prJSONFields,
	)
	if err != nil {
		return PullRequest{}, wrapGhErr(err, fmt.Sprintf("gh pr view %d", number))
	}
	var raw ghPRRaw
	if err := json.Unmarshal(out, &raw); err != nil {
		return PullRequest{}, fmt.Errorf("parse gh pr view: %w", err)
	}
	return raw.toPR(), nil
}

var prURLNumber = regexp.MustCompile(`/pull/(\d+)`)

func (c *GhCli) CreatePR(ctx context.Context, p NewPR) (PullRequest, error) {
	args := []string{"pr", "create",
		"--base", p.Base,
		"--head", p.Head,
		"--title", p.Title,
		"--body", p.Body,
	}
	if p.Draft {
		args = append(args, "--draft")
	}
	out, err := c.run("gh", args...)
	if err != nil {
		return PullRequest{}, wrapGhErr(err, "gh pr create")
	}
	m := prURLNumber.FindStringSubmatch(string(out))
	if len(m) < 2 {
		return PullRequest{}, fmt.Errorf("could not parse PR number from gh pr create output: %q", string(out))
	}
	num, err := strconv.Atoi(m[1])
	if err != nil {
		return PullRequest{}, fmt.Errorf("parse PR number: %w", err)
	}
	return c.GetPR(ctx, num)
}

func (c *GhCli) UpdatePRBody(_ context.Context, number int, body string) error {
	_, err := c.run("gh", "pr", "edit", strconv.Itoa(number), "--body", body)
	if err != nil {
		return wrapGhErr(err, fmt.Sprintf("gh pr edit %d --body", number))
	}
	return nil
}

func (c *GhCli) UpdatePRTitle(_ context.Context, number int, newTitle string) error {
	_, err := c.run("gh", "pr", "edit", strconv.Itoa(number), "--title", newTitle)
	if err != nil {
		return wrapGhErr(err, fmt.Sprintf("gh pr edit %d --title", number))
	}
	return nil
}

func (c *GhCli) UpdatePRBase(_ context.Context, number int, newBase string) error {
	_, err := c.run("gh", "pr", "edit", strconv.Itoa(number), "--base", newBase)
	if err != nil {
		return wrapGhErr(err, fmt.Sprintf("gh pr edit %d --base", number))
	}
	return nil
}

func wrapGhErr(err error, op string) error {
	return &crerrors.ChainrailError{
		Code:       crerrors.CodeGhCallFailed,
		Message:    fmt.Sprintf("%s failed: %s", op, err.Error()),
		Suggestion: "check 'gh auth status' and ensure the gh CLI is installed",
		Cause:      err,
	}
}
