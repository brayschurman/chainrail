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
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		_ = os.Stderr
		_ = r
		return fmt.Errorf("diff viewer error: %w", err)
	}
	return nil
}
