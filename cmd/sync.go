package cmd

import (
	"context"
	"fmt"
	"io"
	"os"

	crerrors "github.com/brayschurman/chainrail/internal/errors"
	"github.com/brayschurman/chainrail/internal/git"
	"github.com/brayschurman/chainrail/internal/github"
	"github.com/brayschurman/chainrail/internal/output"
	"github.com/brayschurman/chainrail/internal/term"
	"github.com/spf13/cobra"
)

type syncDeps struct {
	cwd string
	gh  github.GitHubClient
}

const snapshotRefPrefix = "refs/chainrail/snapshot/"

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Cascade-rebase the stack onto fresh trunk",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		r := output.NewTextRenderer(term.IsTTY(cmd.OutOrStdout()))
		return runSync(cmd.OutOrStdout(), r, syncDeps{
			cwd: cwd,
			gh:  github.New(),
		})
	},
}

func init() {
	rootCmd.AddCommand(syncCmd)
}

func runSync(out io.Writer, r output.Renderer, deps syncDeps) error {
	ctx := context.Background()

	g := git.New(deps.cwd)
	if !g.IsInsideRepo() {
		return &crerrors.ChainrailError{
			Code:       crerrors.CodeNotGitRepo,
			Message:    "not inside a git repository",
			Suggestion: "run 'chainrail sync' from inside a git repository",
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
			Suggestion: "commit or stash before syncing the stack",
		}
	}

	user, err := deps.gh.CurrentUser(ctx)
	if err != nil {
		return err
	}

	originalBranch, err := g.CurrentBranch()
	if err != nil {
		return err
	}
	if originalBranch == trunk {
		return &crerrors.ChainrailError{
			Code:       crerrors.CodeNotOnStack,
			Message:    "currently on the trunk branch '" + trunk + "'",
			Suggestion: "check out a stack branch first",
		}
	}

	parsed, ok := parseStackBranch(originalBranch, user)
	if !ok {
		return &crerrors.ChainrailError{
			Code:       crerrors.CodeNotOnStack,
			Message:    "current branch '" + originalBranch + "' is not a chainrail stack branch for user '" + user + "'",
			Suggestion: "check out a branch created with 'chainrail add'",
		}
	}

	if err := g.Fetch(); err != nil {
		return err
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
			Suggestion: fmt.Sprintf("ensure every position 1..%d exists as a local branch", parsed.position),
		}
	}

	openPRs, err := deps.gh.ListOpenPRs(ctx)
	if err != nil {
		return err
	}
	openByHead := make(map[string]github.PullRequest, len(openPRs))
	for _, pr := range openPRs {
		openByHead[pr.HeadRefName] = pr
	}

	// Candidate parents = every layer except the tip (they're potential ancestors
	// for layers above them and might have been squash-merged).
	var candidateParents []string
	if len(chainBranches) > 1 {
		candidateParents = append(candidateParents, chainBranches[:len(chainBranches)-1]...)
	}
	mergedPRs, err := deps.gh.ListMergedPRsByHead(ctx, candidateParents)
	if err != nil {
		return err
	}
	mergedByHead := make(map[string]github.PullRequest, len(mergedPRs))
	for _, pr := range mergedPRs {
		mergedByHead[pr.HeadRefName] = pr
	}

	for _, branch := range chainBranches {
		sha, err := g.RevParse("refs/heads/" + branch)
		if err != nil {
			return err
		}
		if err := g.UpdateRef(snapshotRefPrefix+branch, sha); err != nil {
			return err
		}
	}

	rebased := 0
	for i, branch := range chainBranches {
		openPR, isOpen := openByHead[branch]
		if !isOpen {
			continue
		}

		if err := g.Checkout(branch); err != nil {
			return err
		}

		parent, parentMerged := resolveEffectiveParent(i, chainBranches, mergedByHead, openByHead, trunk)

		if parentMerged {
			mergedPR := mergedByHead[parent]
			oldParentTip, err := g.RevParse(snapshotRefPrefix + parent)
			if err != nil {
				return err
			}
			if err := g.RebaseOnto(mergedPR.MergeCommitSHA, oldParentTip, branch); err != nil {
				return &crerrors.ChainrailError{
					Code:       crerrors.CodeRebaseConflict,
					Message:    "squash-recovery rebase failed on branch '" + branch + "': " + err.Error(),
					Suggestion: "resolve conflicts, then 'git add' and 'git rebase --continue', then re-run 'chainrail sync'",
					Cause:      err,
				}
			}
			if err := deps.gh.UpdatePRBase(ctx, openPR.Number, trunk); err != nil {
				return err
			}
		} else {
			target := "origin/" + parent
			if parent != trunk {
				target = parent
			}
			if err := g.Rebase(target); err != nil {
				return &crerrors.ChainrailError{
					Code:       crerrors.CodeRebaseConflict,
					Message:    "rebase failed on branch '" + branch + "': " + err.Error(),
					Suggestion: "resolve conflicts, then 'git add' and 'git rebase --continue', then re-run 'chainrail sync'",
					Cause:      err,
				}
			}
		}
		rebased++
	}

	for _, branch := range chainBranches {
		if _, isOpen := openByHead[branch]; !isOpen {
			continue
		}
		if err := g.PushWithLease(branch); err != nil {
			return err
		}
	}

	if err := g.Checkout(originalBranch); err != nil {
		return err
	}

	r.Success(out, fmt.Sprintf("stack synced: %d branch(es) rebased and pushed", rebased))
	return nil
}

// resolveEffectiveParent walks backward from chainBranches[i-1] looking for an
// "effective parent": the nearest preceding layer with a squash-merged PR (in
// which case parentMerged=true and the squash should be used) or with an open
// PR (in which case it's a normal rebase target). If neither is found, the
// trunk is the effective parent.
func resolveEffectiveParent(
	i int,
	chainBranches []string,
	mergedByHead, openByHead map[string]github.PullRequest,
	trunk string,
) (parent string, parentMerged bool) {
	for j := i - 1; j >= 0; j-- {
		prev := chainBranches[j]
		if pr, ok := mergedByHead[prev]; ok && pr.MergeCommitSHA != "" {
			return prev, true
		}
		if _, ok := openByHead[prev]; ok {
			return prev, false
		}
	}
	return trunk, false
}
