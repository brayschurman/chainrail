package cmd

import (
	"io"
	"os"
	"os/exec"

	crerrors "github.com/brayschurman/chainrail/internal/errors"
	"github.com/brayschurman/chainrail/internal/git"
	"github.com/brayschurman/chainrail/internal/output"
	"github.com/brayschurman/chainrail/internal/term"
	"github.com/spf13/cobra"
)

type initDeps struct {
	cwd       string
	checkAuth func() error
}

const trunkConfigKey = "chainrail.trunk"

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize chainrail in the current repo",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		base, _ := cmd.Flags().GetString("base")
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		r := output.NewTextRenderer(term.IsTTY(cmd.OutOrStdout()))
		return runInit(cmd.OutOrStdout(), r, base, initDeps{
			cwd:       cwd,
			checkAuth: defaultGhAuthCheck,
		})
	},
}

func init() {
	initCmd.Flags().String("base", "", "the trunk branch (required)")
	_ = initCmd.MarkFlagRequired("base")
	rootCmd.AddCommand(initCmd)
}

func runInit(out io.Writer, r output.Renderer, base string, deps initDeps) error {
	g := git.New(deps.cwd)
	if !g.IsInsideRepo() {
		return &crerrors.ChainrailError{
			Code:       crerrors.CodeNotGitRepo,
			Message:    "not inside a git repository",
			Suggestion: "run 'chainrail init' from inside a git repository",
		}
	}
	if err := deps.checkAuth(); err != nil {
		return &crerrors.ChainrailError{
			Code:       crerrors.CodeNoGhAuth,
			Message:    "gh CLI is not authenticated",
			Suggestion: "run 'gh auth login' and try again",
			Cause:      err,
		}
	}

	localExists, _ := g.BranchExists(base)
	remoteExists, _ := g.RemoteExists(base)
	if !localExists && !remoteExists {
		return &crerrors.ChainrailError{
			Code:       crerrors.CodeTrunkMissing,
			Message:    "trunk branch '" + base + "' not found locally or on origin",
			Suggestion: "create the branch locally, or push it to origin, then re-run 'chainrail init'",
		}
	}

	existing, _ := g.ConfigGet(trunkConfigKey)
	if existing != "" {
		if existing == base {
			r.Success(out, "chainrail already initialized for trunk: "+base)
			return nil
		}
		return &crerrors.ChainrailError{
			Code:       crerrors.CodeAlreadyInit,
			Message:    "chainrail is already initialized for trunk '" + existing + "'",
			Suggestion: "run 'git config --unset " + trunkConfigKey + "' first if you really want to change it",
		}
	}

	if err := g.ConfigSet(trunkConfigKey, base); err != nil {
		return err
	}
	r.Success(out, "chainrail initialized with trunk: "+base)
	return nil
}

func defaultGhAuthCheck() error {
	cmd := exec.Command("gh", "auth", "status")
	return cmd.Run()
}
