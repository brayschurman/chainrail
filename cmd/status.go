package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	crerrors "github.com/brayschurman/chainrail/internal/errors"
	"github.com/brayschurman/chainrail/internal/git"
	"github.com/brayschurman/chainrail/internal/github"
	"github.com/brayschurman/chainrail/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

type statusDeps struct {
	cwd string
	gh  github.GitHubClient
}

var statusJSONFlag bool

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the current stack state",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		return runStatus(cmd.OutOrStdout(), statusJSONFlag, statusDeps{
			cwd: cwd,
			gh:  github.New(),
		})
	},
}

func init() {
	statusCmd.Flags().BoolVar(&statusJSONFlag, "json", false, "output as JSON instead of TUI")
	rootCmd.AddCommand(statusCmd)
}

type statusOutput struct {
	Stack  string      `json:"stack"`
	Trunk  string      `json:"trunk"`
	Layers []tui.Layer `json:"layers"`
}

func runStatus(out io.Writer, asJSON bool, deps statusDeps) error {
	ctx := context.Background()
	g := git.New(deps.cwd)

	if !g.IsInsideRepo() {
		return &crerrors.ChainrailError{
			Code:    crerrors.CodeNotGitRepo,
			Message: "not inside a git repository",
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

	user, err := deps.gh.CurrentUser(ctx)
	if err != nil {
		return err
	}

	currentBranch, err := g.CurrentBranch()
	if err != nil {
		return err
	}

	parsed, ok := parseStackBranch(currentBranch, user)
	if !ok {
		return &crerrors.ChainrailError{
			Code:       crerrors.CodeNotOnStack,
			Message:    "current branch '" + currentBranch + "' is not a chainrail stack branch",
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
			Code:    crerrors.CodeNotOnStack,
			Message: "stack chain for '" + parsed.baseSlug + "' has gaps locally",
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

	layers := buildLayers(chainBranches, currentBranch, trunk, openByHead, mergedByHead)

	if asJSON {
		return json.NewEncoder(out).Encode(statusOutput{
			Stack:  parsed.baseSlug,
			Trunk:  trunk,
			Layers: layers,
		})
	}

	m := tui.Model{
		StackName: parsed.baseSlug,
		Trunk:     trunk,
		Layers:    layers,
	}
	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}
	return nil
}

func buildLayers(
	chainBranches []string,
	currentBranch, trunk string,
	openByHead, mergedByHead map[string]github.PullRequest,
) []tui.Layer {
	layers := make([]tui.Layer, len(chainBranches))
	for i, branch := range chainBranches {
		l := tui.Layer{
			Position:  i + 1,
			Branch:    branch,
			IsCurrent: branch == currentBranch,
		}
		if pr, ok := openByHead[branch]; ok {
			l.PRNumber = pr.Number
			l.PRState = "OPEN"
			_, parentMerged := resolveEffectiveParent(i, chainBranches, mergedByHead, openByHead, trunk)
			l.NeedsSync = parentMerged
		} else if pr, ok := mergedByHead[branch]; ok {
			l.PRNumber = pr.Number
			l.PRState = "MERGED"
		}
		layers[i] = l
	}
	return layers
}
