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
	"github.com/brayschurman/chainrail/internal/stack"
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

	if err := g.Fetch(); err != nil {
		return err
	}

	prs, err := deps.gh.ListOpenPRs(ctx)
	if err != nil {
		return err
	}
	localBranches, err := g.ListLocalBranches()
	if err != nil {
		return err
	}
	localSet := make(map[string]bool, len(localBranches))
	for _, b := range localBranches {
		localSet[b] = true
	}

	chain, err := stack.Walk(originalBranch, trunk, prs, localSet)
	if err != nil {
		return err
	}
	if len(chain) == 0 {
		return &crerrors.ChainrailError{
			Code:       crerrors.CodeNotOnStack,
			Message:    "no stack PRs reachable from '" + originalBranch + "'",
			Suggestion: "run 'chainrail submit' first to open PRs for the stack",
		}
	}

	for _, layer := range chain {
		sha, err := g.RevParse("refs/heads/" + layer.Branch)
		if err != nil {
			return err
		}
		if err := g.UpdateRef(snapshotRefPrefix+layer.Branch, sha); err != nil {
			return err
		}
	}

	for i, layer := range chain {
		parent := trunk
		if layer.Parent != "" {
			parent = layer.Parent
		}
		if err := g.Checkout(layer.Branch); err != nil {
			return err
		}
		target := "origin/" + parent
		if i > 0 {
			target = parent
		}
		if err := g.Rebase(target); err != nil {
			return &crerrors.ChainrailError{
				Code:       crerrors.CodeRebaseConflict,
				Message:    "rebase failed on branch '" + layer.Branch + "': " + err.Error(),
				Suggestion: "resolve conflicts, then 'git add' and 'git rebase --continue', then re-run 'chainrail sync'",
				Cause:      err,
			}
		}
	}

	for _, layer := range chain {
		if err := g.PushWithLease(layer.Branch); err != nil {
			return err
		}
	}

	if err := g.Checkout(originalBranch); err != nil {
		return err
	}

	r.Success(out, fmt.Sprintf("stack synced: %d branch(es) rebased and pushed", len(chain)))
	return nil
}
