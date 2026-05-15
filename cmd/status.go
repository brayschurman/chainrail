package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"

	crerrors "github.com/brayschurman/chainrail/internal/errors"
	"github.com/brayschurman/chainrail/internal/git"
	"github.com/brayschurman/chainrail/internal/github"
	"github.com/brayschurman/chainrail/internal/output"
	"github.com/brayschurman/chainrail/internal/term"
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
	Short: "Show stack health — works from any branch",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		r := output.NewTextRenderer(term.IsTTY(cmd.OutOrStdout()))
		return runStatus(cmd.OutOrStdout(), r, statusJSONFlag, statusDeps{
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
	Layers []tui.Layer `json:"layers"`
}

func runStatus(out io.Writer, r output.Renderer, asJSON bool, deps statusDeps) error {
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
			Code:       crerrors.CodeTrunkMissing,
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

	localBranches, err := g.ListLocalBranches()
	if err != nil {
		return err
	}

	// Discover every stack this user has locally, grouped by base slug.
	allStacks := discoverAllStacks(user, localBranches)
	if len(allStacks) == 0 {
		if asJSON {
			return json.NewEncoder(out).Encode(statusOutput{Layers: []tui.Layer{}})
		}
		m := tui.Model{}
		tea.NewProgram(m).Run() //nolint — empty state renders the "no stacks" hint
		return nil
	}

	// Collect all candidate parent branches across all stacks for merged-PR lookup.
	var allParents []string
	for _, branches := range allStacks {
		if len(branches) > 1 {
			allParents = append(allParents, branches[:len(branches)-1]...)
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

	mergedPRs, err := deps.gh.ListMergedPRsByHead(ctx, allParents)
	if err != nil {
		return err
	}
	mergedByHead := make(map[string]github.PullRequest, len(mergedPRs))
	for _, pr := range mergedPRs {
		mergedByHead[pr.HeadRefName] = pr
	}

	// Build a flat, sorted layer list across all stacks.
	slugs := make([]string, 0, len(allStacks))
	for slug := range allStacks {
		slugs = append(slugs, slug)
	}
	sort.Strings(slugs)

	var layers []tui.Layer
	for _, slug := range slugs {
		branches := allStacks[slug]
		for i, branch := range branches {
			l := tui.Layer{
				Stack:     slug,
				Position:  i + 1,
				Branch:    branch,
				IsCurrent: branch == currentBranch,
			}
			if pr, ok := openByHead[branch]; ok {
				l.PRNumber = pr.Number
				l.PRState = "OPEN"
				_, parentMerged := resolveEffectiveParent(i, branches, mergedByHead, openByHead, trunk)
				l.NeedsSync = parentMerged
			} else if pr, ok := mergedByHead[branch]; ok {
				l.PRNumber = pr.Number
				l.PRState = "MERGED"
			}
			layers = append(layers, l)
		}
	}

	if asJSON {
		return json.NewEncoder(out).Encode(statusOutput{Layers: layers})
	}

	// Default cursor to current branch, or 0.
	cursor := 0
	for i, l := range layers {
		if l.IsCurrent {
			cursor = i
			break
		}
	}

	m := tui.Model{Layers: layers, Cursor: cursor}
	p := tea.NewProgram(m)
	final, err := p.Run()
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	result := final.(tui.Model).Result()
	return executeStatusAction(out, r, result, deps)
}

func executeStatusAction(out io.Writer, r output.Renderer, result tui.Result, deps statusDeps) error {
	ctx := context.Background()

	switch result.Action {
	case tui.ActionCheckout:
		g := git.New(deps.cwd)
		if err := g.Checkout(result.Branch); err != nil {
			return err
		}
		r.Success(out, "checked out "+result.Branch)

	case tui.ActionOpenPR:
		cmd := exec.Command("gh", "pr", "view", fmt.Sprintf("%d", result.PRNumber), "--web")
		cmd.Stdout = out
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("gh pr view --web: %w", err)
		}

	case tui.ActionSync:
		g := git.New(deps.cwd)
		if err := g.Checkout(result.Branch); err != nil {
			return err
		}
		syncR := output.NewTextRenderer(term.IsTTY(out))
		return runSync(out, syncR, syncDeps{cwd: deps.cwd, gh: deps.gh})

	case tui.ActionSubmit:
		g := git.New(deps.cwd)
		if err := g.Checkout(result.Branch); err != nil {
			return err
		}
		submitR := output.NewTextRenderer(term.IsTTY(out))
		return runSubmit(out, submitR, submitDeps{cwd: deps.cwd, gh: deps.gh})

	case tui.ActionNone:
		// user just quit

	default:
		_ = ctx // keep import
	}
	return nil
}

// discoverAllStacks finds every chainrail stack branch for this user in the
// local branch list. Returns a map of base-slug → ordered branch names.
func discoverAllStacks(user string, localBranches []string) map[string][]string {
	type entry struct {
		position int
		branch   string
	}
	grouped := map[string][]entry{}
	for _, b := range localBranches {
		p, ok := parseStackBranch(b, user)
		if !ok {
			continue
		}
		grouped[p.baseSlug] = append(grouped[p.baseSlug], entry{position: p.position, branch: b})
	}

	result := make(map[string][]string, len(grouped))
	for slug, entries := range grouped {
		sort.Slice(entries, func(i, j int) bool { return entries[i].position < entries[j].position })
		branches := make([]string, len(entries))
		for i, e := range entries {
			branches[i] = e.branch
		}
		result[slug] = branches
	}
	return result
}

// buildLayers is kept for tests.
func buildLayers(
	chainBranches []string,
	currentBranch, trunk, stack string,
	openByHead, mergedByHead map[string]github.PullRequest,
) []tui.Layer {
	layers := make([]tui.Layer, len(chainBranches))
	for i, branch := range chainBranches {
		l := tui.Layer{
			Stack:     stack,
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
