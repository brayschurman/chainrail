package github

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

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
}

func (r ghPRRaw) toPR() PullRequest {
	pr := PullRequest{
		Number:      r.Number,
		Title:       r.Title,
		BaseRefName: r.BaseRefName,
		HeadRefName: r.HeadRefName,
		State:       r.State,
		Body:        r.Body,
	}
	if r.MergeCommit != nil {
		pr.MergeCommitSHA = r.MergeCommit.OID
	}
	return pr
}

const prJSONFields = "number,title,baseRefName,headRefName,state,body,mergeCommit"

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
