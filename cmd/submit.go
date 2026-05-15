package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	crerrors "github.com/brayschurman/chainrail/internal/errors"
	"github.com/brayschurman/chainrail/internal/git"
	"github.com/brayschurman/chainrail/internal/github"
	"github.com/brayschurman/chainrail/internal/output"
	"github.com/brayschurman/chainrail/internal/term"
	"github.com/spf13/cobra"
)

type submitDeps struct {
	cwd string
	gh  github.GitHubClient
}

const (
	stackMapStartMarker = "<!-- chainrail:stack:start -->"
	stackMapEndMarker   = "<!-- chainrail:stack:end -->"
)

var submitCmd = &cobra.Command{
	Use:   "submit",
	Short: "Push the stack and open or update PRs",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		r := output.NewTextRenderer(term.IsTTY(cmd.OutOrStdout()))
		return runSubmit(cmd.OutOrStdout(), r, submitDeps{
			cwd: cwd,
			gh:  github.New(),
		})
	},
}

func init() {
	rootCmd.AddCommand(submitCmd)
}

func runSubmit(out io.Writer, r output.Renderer, deps submitDeps) error {
	ctx := context.Background()

	g := git.New(deps.cwd)
	if !g.IsInsideRepo() {
		return &crerrors.ChainrailError{
			Code:       crerrors.CodeNotGitRepo,
			Message:    "not inside a git repository",
			Suggestion: "run 'chainrail submit' from inside a git repository",
		}
	}

	trunk, err := g.ConfigGet(trunkConfigKey)
	if err != nil || trunk == "" {
		return &crerrors.ChainrailError{
			Code:       crerrors.CodeNotOnStack,
			Message:    "chainrail is not initialized in this repository",
			Suggestion: "run 'chainrail init --base <trunk>' first",
		}
	}

	dirty, err := g.IsDirty()
	if err != nil {
		return err
	}
	if dirty {
		return &crerrors.ChainrailError{
			Code:       crerrors.CodeDirtyWorktree,
			Message:    "working tree has uncommitted changes",
			Suggestion: "commit or stash before submitting the stack",
		}
	}

	user, err := deps.gh.CurrentUser(ctx)
	if err != nil {
		return err
	}

	currentBranch, err := g.CurrentBranch()
	if err != nil {
		return err
	}
	if currentBranch == trunk {
		return &crerrors.ChainrailError{
			Code:       crerrors.CodeNotOnStack,
			Message:    "currently on the trunk branch '" + trunk + "'",
			Suggestion: "check out a stack branch (e.g. one made with 'chainrail add') first",
		}
	}

	parsed, ok := parseStackBranch(currentBranch, user)
	if !ok {
		return &crerrors.ChainrailError{
			Code:       crerrors.CodeNotOnStack,
			Message:    "current branch '" + currentBranch + "' is not a chainrail stack branch for user '" + user + "'",
			Suggestion: "check out a branch created with 'chainrail add'",
		}
	}

	localBranches, err := g.ListLocalBranches()
	if err != nil {
		return err
	}
	chainBranches := discoverStackBranches(user, parsed.baseSlug, parsed.position, localBranches)
	if chainBranches == nil {
		return &crerrors.ChainrailError{
			Code:       crerrors.CodeNotOnStack,
			Message:    "stack chain for '" + parsed.baseSlug + "' has gaps locally",
			Suggestion: "ensure every position 1.." + fmt.Sprintf("%d", parsed.position) + " exists as a local branch",
		}
	}

	prs, err := deps.gh.ListOpenPRs(ctx)
	if err != nil {
		return err
	}
	byHead := make(map[string]github.PullRequest, len(prs))
	for _, pr := range prs {
		byHead[pr.HeadRefName] = pr
	}

	chain := make([]github.PullRequest, 0, len(chainBranches))
	for i, branch := range chainBranches {
		if err := g.PushWithLease(branch); err != nil {
			return err
		}
		if existing, ok := byHead[branch]; ok {
			chain = append(chain, existing)
			continue
		}

		base := trunk
		if i > 0 {
			base = chainBranches[i-1]
		}
		bp, _ := parseStackBranch(branch, user)
		title := parsed.baseSlug + ": " + bp.taskSlug
		newPR, err := deps.gh.CreatePR(ctx, github.NewPR{
			Title: title,
			Body:  "",
			Head:  branch,
			Base:  base,
		})
		if err != nil {
			return err
		}
		chain = append(chain, newPR)
	}

	stackMap := buildStackMap(chain, currentBranch)
	for _, pr := range chain {
		newBody := injectStackMap(pr.Body, stackMap)
		if newBody == pr.Body {
			continue
		}
		if err := deps.gh.UpdatePRBody(ctx, pr.Number, newBody); err != nil {
			return err
		}
	}

	r.Success(out, fmt.Sprintf("stack submitted: %d PR(s) on chain rooted at '%s'", len(chain), parsed.baseSlug))
	return nil
}

// discoverStackBranches returns local branches matching <user>/<baseSlug>-<N>-* for
// N from 1 to currentPos. Returns nil if the chain has gaps or duplicates.
func discoverStackBranches(user, baseSlug string, currentPos int, allBranches []string) []string {
	type entry struct {
		position int
		branch   string
	}
	var hits []entry
	for _, b := range allBranches {
		parsed, ok := parseStackBranch(b, user)
		if !ok {
			continue
		}
		if parsed.baseSlug != baseSlug {
			continue
		}
		if parsed.position < 1 || parsed.position > currentPos {
			continue
		}
		hits = append(hits, entry{position: parsed.position, branch: b})
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].position < hits[j].position })
	if len(hits) != currentPos {
		return nil
	}
	result := make([]string, 0, len(hits))
	for i, h := range hits {
		if h.position != i+1 {
			return nil
		}
		result = append(result, h.branch)
	}
	return result
}

func buildStackMap(chain []github.PullRequest, currentBranch string) string {
	var b strings.Builder
	b.WriteString(stackMapStartMarker)
	b.WriteString("\n## Stack\n\n")
	for i, pr := range chain {
		marker := ""
		if pr.HeadRefName == currentBranch {
			marker = " ← you are here"
		}
		fmt.Fprintf(&b, "- #%d (%d/%d) %s%s\n", pr.Number, i+1, len(chain), pr.HeadRefName, marker)
	}
	b.WriteString("\n<sub>Maintained by [chainrail](https://github.com/brayschurman/chainrail). Edit above and below freely.</sub>\n")
	b.WriteString(stackMapEndMarker)
	return b.String()
}

// injectStackMap replaces the chainrail stack-map block in body with newBlock,
// or prepends it (followed by a blank line) if no block exists yet.
func injectStackMap(body, newBlock string) string {
	startIdx := strings.Index(body, stackMapStartMarker)
	if startIdx >= 0 {
		endIdx := strings.Index(body, stackMapEndMarker)
		if endIdx >= 0 && endIdx > startIdx {
			tail := endIdx + len(stackMapEndMarker)
			return body[:startIdx] + newBlock + body[tail:]
		}
	}
	if body == "" {
		return newBlock + "\n"
	}
	return newBlock + "\n\n" + body
}
