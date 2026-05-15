package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/brayschurman/chainrail/internal/diffview"
	"github.com/brayschurman/chainrail/internal/github"
	"github.com/brayschurman/chainrail/internal/output"
	"github.com/brayschurman/chainrail/internal/reviewstate"
	"github.com/brayschurman/chainrail/internal/term"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

type viewDeps struct {
	gh github.GitHubClient
}

var viewCmd = &cobra.Command{
	Use:   "view <PR>",
	Short: "Open a PR's diff in the terminal",
	Long: `Open a PR's diff inside chainrail's terminal viewer — file sidebar
on the left, unified diff on the right. Keyboard-driven: tab cycles files,
↑↓ scrolls, q quits.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		num, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("PR must be a number, got %q", args[0])
		}
		r := output.NewTextRenderer(term.IsTTY(cmd.OutOrStdout()))
		return runView(cmd.OutOrStdout(), r, num, viewDeps{gh: github.New()})
	},
}

func init() {
	rootCmd.AddCommand(viewCmd)
}


func runView(out io.Writer, r output.Renderer, number int, deps viewDeps) error {
	ctx := context.Background()
	pr, err := deps.gh.GetPR(ctx, number)
	if err != nil {
		return err
	}
	diff, err := deps.gh.PRDiff(ctx, number)
	if err != nil {
		return err
	}
	files := diffview.Parse(diff)
	title := fmt.Sprintf("#%d %s", pr.Number, pr.Title)
	m := diffview.New(title, files)
	m.PRBody = pr.Body
	m.PRNumber = pr.Number
	m.PlanSignal = diffview.DetectPlan(pr.Body)
	m.PlanNudger = func(num int, body string) error {
		return deps.gh.CommentOnPR(context.Background(), num, body)
	}

	// Best-effort review-state wiring. If any of these calls fail we skip
	// the review UI entirely rather than refusing to open the viewer.
	if owner, name, err := deps.gh.RepoInfo(ctx); err == nil {
		if store, err := reviewstate.NewStore(); err == nil {
			if st, err := store.Load(owner, name, number); err == nil {
				m.RepoOwner = owner
				m.RepoName = name
				m.ReviewStore = store
				m.ReviewState = st
				if user, err := deps.gh.CurrentUser(ctx); err == nil && st.Reviewer == "" {
					st.Reviewer = user
				}
			}
			if prFiles, err := deps.gh.PRFiles(ctx, number); err == nil {
				blobs := make(map[string]string, len(prFiles))
				for _, f := range prFiles {
					blobs[f.Path] = f.BlobSHA
				}
				m.BlobByPath = blobs
			}
		}
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		_ = os.Stderr
		_ = r
		return fmt.Errorf("diff viewer error: %w", err)
	}
	return nil
}
