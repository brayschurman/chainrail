package cmd

import (
	"context"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"

	crerrors "github.com/brayschurman/chainrail/internal/errors"
	"github.com/brayschurman/chainrail/internal/git"
	"github.com/brayschurman/chainrail/internal/github"
	"github.com/brayschurman/chainrail/internal/output"
	"github.com/brayschurman/chainrail/internal/term"
	"github.com/spf13/cobra"
)

type addDeps struct {
	cwd     string
	getUser func() (string, error)
}

var addCmd = &cobra.Command{
	Use:   "add <slug>",
	Short: "Create the next branch in the stack",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		r := output.NewTextRenderer(term.IsTTY(cmd.OutOrStdout()))
		return runAdd(cmd.OutOrStdout(), r, args[0], addDeps{
			cwd:     cwd,
			getUser: defaultGetUser,
		})
	},
}

func init() {
	rootCmd.AddCommand(addCmd)
}

var slugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

func runAdd(out io.Writer, r output.Renderer, slug string, deps addDeps) error {
	if !slugPattern.MatchString(slug) {
		return &crerrors.ChainrailError{
			Code:       crerrors.CodeNotOnStack,
			Message:    "invalid slug '" + slug + "'",
			Suggestion: "use lowercase letters, digits, and hyphens (e.g. 'schema' or 'fix-bug-42')",
		}
	}

	g := git.New(deps.cwd)
	if !g.IsInsideRepo() {
		return &crerrors.ChainrailError{
			Code:       crerrors.CodeNotGitRepo,
			Message:    "not inside a git repository",
			Suggestion: "run 'chainrail add' from inside a git repository",
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
			Suggestion: "commit or stash before adding to the stack",
		}
	}

	currentBranch, err := g.CurrentBranch()
	if err != nil {
		return err
	}

	user, err := deps.getUser()
	if err != nil {
		return err
	}

	var baseSlug string
	var position int

	if currentBranch == trunk {
		baseSlug = slug
		position = 1
	} else {
		parsed, ok := parseStackBranch(currentBranch, user)
		if !ok {
			return &crerrors.ChainrailError{
				Code:       crerrors.CodeNotOnStack,
				Message:    "current branch '" + currentBranch + "' is not a chainrail stack branch for user '" + user + "'",
				Suggestion: "git checkout " + trunk + ", or check out an existing stack branch, then re-run 'chainrail add'",
			}
		}
		baseSlug = parsed.baseSlug
		position = parsed.position + 1
	}

	newBranch := user + "/" + baseSlug + "-" + strconv.Itoa(position) + "-" + slug

	exists, err := g.BranchExists(newBranch)
	if err != nil {
		return err
	}
	if exists {
		return &crerrors.ChainrailError{
			Code:       crerrors.CodeSlugTaken,
			Message:    "branch '" + newBranch + "' already exists",
			Suggestion: "choose a different slug or delete the existing branch",
		}
	}

	if err := g.CreateBranch(newBranch); err != nil {
		return err
	}
	r.Success(out, "created stack branch: "+newBranch)
	return nil
}

type parsedStackBranch struct {
	baseSlug string
	position int
	taskSlug string
}

// parseStackBranch decodes a branch name of the form `<user>/<base-slug>-<N>-<task-slug>`.
// Returns ok=false if the branch doesn't match this user's stack-branch pattern.
//
// Scans right-to-left through dash-separated segments so that hyphens inside
// base-slug or task-slug are handled correctly (e.g. ignore-logs-1-ignore-logs).
func parseStackBranch(branch, user string) (parsedStackBranch, bool) {
	prefix := user + "/"
	if !strings.HasPrefix(branch, prefix) {
		return parsedStackBranch{}, false
	}
	parts := strings.Split(strings.TrimPrefix(branch, prefix), "-")
	// Need at least base + N + task = 3 segments.
	if len(parts) < 3 {
		return parsedStackBranch{}, false
	}
	// Walk right-to-left; skip the rightmost segment (it belongs to task-slug),
	// then find the first integer — that's the position number.
	for i := len(parts) - 2; i >= 1; i-- {
		pos, err := strconv.Atoi(parts[i])
		if err != nil || pos < 1 {
			continue
		}
		baseSlug := strings.Join(parts[:i], "-")
		taskSlug := strings.Join(parts[i+1:], "-")
		if baseSlug == "" || taskSlug == "" {
			continue
		}
		return parsedStackBranch{baseSlug: baseSlug, position: pos, taskSlug: taskSlug}, true
	}
	return parsedStackBranch{}, false
}

func defaultGetUser() (string, error) {
	return github.New().CurrentUser(context.Background())
}
