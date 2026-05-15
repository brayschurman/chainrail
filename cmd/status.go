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

var (
	statusJSONFlag bool
	statusAllFlag  bool
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show stack health — works from any branch",
	Long: `Show your stack health in an interactive TUI.

Use --all to see every open PR in the repo as a dependency chain,
without needing chainrail to be initialized first.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		r := output.NewTextRenderer(term.IsTTY(cmd.OutOrStdout()))
		return runStatus(cmd.OutOrStdout(), r, statusJSONFlag, statusAllFlag, statusDeps{
			cwd: cwd,
			gh:  github.New(),
		})
	},
}

func init() {
	statusCmd.Flags().BoolVar(&statusJSONFlag, "json", false, "output as JSON instead of TUI")
	statusCmd.Flags().BoolVarP(&statusAllFlag, "all", "a", false, "show all open PRs as a dependency chain (no cn init required)")
	rootCmd.AddCommand(statusCmd)
}

type statusOutput struct {
	Layers []tui.Layer `json:"layers"`
}

func runStatus(out io.Writer, r output.Renderer, asJSON, allPRs bool, deps statusDeps) error {
	ctx := context.Background()

	if allPRs {
		return runStatusAll(ctx, out, r, asJSON, deps)
	}
	return runStatusStack(ctx, out, r, asJSON, deps)
}

// runStatusAll fetches every open PR and visualises the full dependency graph.
// Works without cn init.
func runStatusAll(ctx context.Context, out io.Writer, r output.Renderer, asJSON bool, deps statusDeps) error {
	g := git.New(deps.cwd)
	currentBranch := ""
	if g.IsInsideRepo() {
		currentBranch, _ = g.CurrentBranch()
	}

	prs, err := deps.gh.ListAllOpenPRs(ctx)
	if err != nil {
		return err
	}

	layers := buildPRTree(prs, currentBranch)

	if asJSON {
		return json.NewEncoder(out).Encode(statusOutput{Layers: layers})
	}

	return launchTUI(out, r, layers, deps)
}

// runStatusStack shows chainrail-managed stacks for the current repo.
func runStatusStack(ctx context.Context, out io.Writer, r output.Renderer, asJSON bool, deps statusDeps) error {
	g := git.New(deps.cwd)

	if !g.IsInsideRepo() {
		return &crerrors.ChainrailError{
			Code:    crerrors.CodeNotGitRepo,
			Message: "not inside a git repository",
		}
	}

	trunk, err := g.ConfigGet(trunkConfigKey)
	if err != nil || trunk == "" {
		// Friendly nudge instead of a hard error.
		fmt.Fprintln(out, "chainrail isn't set up in this repo yet.")
		fmt.Fprintln(out)
		fmt.Fprintln(out, "  Run 'cn init --base <trunk>' to start managing stacked PRs.")
		fmt.Fprintln(out, "  Or run 'cn status --all' to see all open PRs in this repo.")
		return nil
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

	allStacks := discoverAllStacks(user, localBranches)

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
				l.Title = pr.Title
				l.CIStatus = pr.CIStatus
				l.ReviewDecision = pr.ReviewDecision
				l.UpdatedAt = pr.UpdatedAt
				_, parentMerged := resolveEffectiveParent(i, branches, mergedByHead, openByHead, trunk)
				l.NeedsSync = parentMerged
			} else if pr, ok := mergedByHead[branch]; ok {
				l.PRNumber = pr.Number
				l.PRState = "MERGED"
				l.Title = pr.Title
				l.UpdatedAt = pr.UpdatedAt
			}
			layers = append(layers, l)
		}
	}

	if len(layers) == 0 {
		// No local stack branches — fall through to empty TUI hint.
	}

	if asJSON {
		return json.NewEncoder(out).Encode(statusOutput{Layers: layers})
	}

	// Build the Review and All tabs alongside Mine. Best-effort: failures here
	// leave the tab empty rather than blocking the TUI.
	reviewLayers := buildReviewLayers(ctx, deps.gh, currentBranch)
	allLayers := buildAllLayers(ctx, deps.gh, currentBranch)

	tabs := []tui.Tab{
		{Label: "Mine", Layers: layers},
		{Label: "Review", Layers: reviewLayers},
		{Label: "All", Layers: allLayers},
	}

	return launchTUIWithTabs(out, r, tabs, 0, deps)
}

// buildReviewLayers returns one Layer per PR where the current user is a
// requested reviewer. Returns nil on error.
func buildReviewLayers(ctx context.Context, gh github.GitHubClient, currentBranch string) []tui.Layer {
	prs, err := gh.ListReviewRequestedPRs(ctx)
	if err != nil || len(prs) == 0 {
		return nil
	}
	sort.Slice(prs, func(i, j int) bool { return prs[i].Number < prs[j].Number })
	out := make([]tui.Layer, len(prs))
	for i, pr := range prs {
		out[i] = tui.Layer{
			Stack:          pr.BaseRefName,
			Branch:         pr.HeadRefName,
			Title:          pr.Title,
			PRNumber:       pr.Number,
			PRState:        pr.State,
			CIStatus:       pr.CIStatus,
			ReviewDecision: pr.ReviewDecision,
			UpdatedAt:      pr.UpdatedAt,
			IsCurrent:      pr.HeadRefName == currentBranch,
		}
	}
	return out
}

func buildAllLayers(ctx context.Context, gh github.GitHubClient, currentBranch string) []tui.Layer {
	prs, err := gh.ListAllOpenPRs(ctx)
	if err != nil {
		return nil
	}
	return buildPRTree(prs, currentBranch)
}

func launchTUI(out io.Writer, r output.Renderer, layers []tui.Layer, deps statusDeps) error {
	return launchTUIWithTabs(out, r, []tui.Tab{{Label: "All", Layers: layers}}, 0, deps)
}

func launchTUIWithTabs(out io.Writer, r output.Renderer, tabs []tui.Tab, active int, deps statusDeps) error {
	// Seed each tab's cursor at its current branch row, if any.
	for ti, tab := range tabs {
		for i, l := range tab.Layers {
			if l.IsCurrent {
				tabs[ti].Cursor = i
				break
			}
		}
	}

	// If only one tab is in play, drop the tab bar — keeps the simple-case UX.
	var modelTabs []tui.Tab
	if len(tabs) > 1 {
		modelTabs = tabs
	}

	m := tui.Model{
		Layers:    tabs[active].Layers,
		Cursor:    tabs[active].Cursor,
		Tabs:      modelTabs,
		ActiveTab: active,
		Updater:   deps.gh.UpdatePRTitle,
	}
	p := tea.NewProgram(m)
	final, err := p.Run()
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	return executeStatusAction(out, r, final.(tui.Model).Result(), deps)
}

func executeStatusAction(out io.Writer, r output.Renderer, result tui.Result, deps statusDeps) error {
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
	}
	return nil
}

// buildPRTree takes all open PRs and arranges them as a depth-first tree
// rooted at branches that aren't the head of another open PR.
func buildPRTree(prs []github.PullRequest, currentBranch string) []tui.Layer {
	if len(prs) == 0 {
		return nil
	}

	// Index: headRefName → PR
	headSet := make(map[string]bool, len(prs))
	for _, pr := range prs {
		headSet[pr.HeadRefName] = true
	}

	// Children: baseRef → []PR (sorted by number)
	children := make(map[string][]github.PullRequest)
	for _, pr := range prs {
		children[pr.BaseRefName] = append(children[pr.BaseRefName], pr)
	}
	for k := range children {
		sort.Slice(children[k], func(i, j int) bool {
			return children[k][i].Number < children[k][j].Number
		})
	}

	// Roots: PRs whose base isn't another open PR's head.
	var roots []github.PullRequest
	for _, pr := range prs {
		if !headSet[pr.BaseRefName] {
			roots = append(roots, pr)
		}
	}
	sort.Slice(roots, func(i, j int) bool { return roots[i].Number < roots[j].Number })

	// Group roots by their base branch (trunk) for the Stack header.
	var layers []tui.Layer
	var walk func(pr github.PullRequest, depth int, stack string)
	walk = func(pr github.PullRequest, depth int, stack string) {
		layers = append(layers, tui.Layer{
			Stack:          stack,
			Branch:         pr.HeadRefName,
			Title:          pr.Title,
			PRNumber:       pr.Number,
			PRState:        pr.State,
			CIStatus:       pr.CIStatus,
			ReviewDecision: pr.ReviewDecision,
			UpdatedAt:      pr.UpdatedAt,
			IsCurrent:      pr.HeadRefName == currentBranch,
			Depth:          depth,
		})
		for _, child := range children[pr.HeadRefName] {
			walk(child, depth+1, stack)
		}
	}

	for _, root := range roots {
		walk(root, 0, root.BaseRefName)
	}
	return layers
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
			l.CIStatus = pr.CIStatus
			l.ReviewDecision = pr.ReviewDecision
			l.UpdatedAt = pr.UpdatedAt
			_, parentMerged := resolveEffectiveParent(i, chainBranches, mergedByHead, openByHead, trunk)
			l.NeedsSync = parentMerged
		} else if pr, ok := mergedByHead[branch]; ok {
			l.PRNumber = pr.Number
			l.PRState = "MERGED"
			l.UpdatedAt = pr.UpdatedAt
		}
		layers[i] = l
	}
	return layers
}
