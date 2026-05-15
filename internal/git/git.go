package git

import (
	"fmt"
	"os/exec"
	"strings"

	crerrors "github.com/brayschurman/chainrail/internal/errors"
)

type Runner func(args ...string) (stdout, stderr []byte, err error)

type Git struct {
	run Runner
	cwd string
}

func New(cwd string) *Git {
	g := &Git{cwd: cwd}
	g.run = g.defaultRun
	return g
}

func NewWithRunner(cwd string, r Runner) *Git {
	return &Git{cwd: cwd, run: r}
}

func (g *Git) defaultRun(args ...string) ([]byte, []byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = g.cwd
	var stderrBuf strings.Builder
	cmd.Stderr = &stderrBuf
	out, err := cmd.Output()
	return out, []byte(stderrBuf.String()), err
}

func (g *Git) wrap(err error, op string, stderr []byte) error {
	if err == nil {
		return nil
	}
	msg := strings.TrimSpace(string(stderr))
	if msg == "" {
		msg = err.Error()
	}
	return &crerrors.ChainrailError{
		Code:    crerrors.CodeGitCallFailed,
		Message: fmt.Sprintf("git %s failed: %s", op, msg),
		Cause:   err,
	}
}

func (g *Git) IsInsideRepo() bool {
	_, _, err := g.run("rev-parse", "--is-inside-work-tree")
	return err == nil
}

func (g *Git) CurrentBranch() (string, error) {
	out, stderr, err := g.run("rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", g.wrap(err, "rev-parse --abbrev-ref HEAD", stderr)
	}
	return strings.TrimSpace(string(out)), nil
}

func (g *Git) IsDirty() (bool, error) {
	out, stderr, err := g.run("status", "--porcelain")
	if err != nil {
		return false, g.wrap(err, "status --porcelain", stderr)
	}
	return strings.TrimSpace(string(out)) != "", nil
}

func (g *Git) BranchExists(name string) (bool, error) {
	_, _, err := g.run("show-ref", "--verify", "--quiet", "refs/heads/"+name)
	if err == nil {
		return true, nil
	}
	if _, ok := err.(*exec.ExitError); ok {
		return false, nil
	}
	return false, g.wrap(err, "show-ref", nil)
}

func (g *Git) RemoteExists(branch string) (bool, error) {
	_, _, err := g.run("show-ref", "--verify", "--quiet", "refs/remotes/origin/"+branch)
	if err == nil {
		return true, nil
	}
	if _, ok := err.(*exec.ExitError); ok {
		return false, nil
	}
	return false, g.wrap(err, "show-ref", nil)
}

func (g *Git) CreateBranch(name string) error {
	_, stderr, err := g.run("checkout", "-b", name)
	if err != nil {
		return g.wrap(err, "checkout -b "+name, stderr)
	}
	return nil
}

func (g *Git) Checkout(name string) error {
	_, stderr, err := g.run("checkout", name)
	if err != nil {
		return g.wrap(err, "checkout "+name, stderr)
	}
	return nil
}

func (g *Git) PushWithLease(branch string) error {
	_, stderr, err := g.run("push", "--force-with-lease", "origin", branch)
	if err != nil {
		return g.wrap(err, "push --force-with-lease "+branch, stderr)
	}
	return nil
}

func (g *Git) Fetch() error {
	_, stderr, err := g.run("fetch", "origin")
	if err != nil {
		return g.wrap(err, "fetch origin", stderr)
	}
	return nil
}

func (g *Git) Rebase(target string) error {
	_, stderr, err := g.run("rebase", target)
	if err != nil {
		return g.wrap(err, "rebase "+target, stderr)
	}
	return nil
}

func (g *Git) RebaseOnto(newBase, oldBase, branch string) error {
	_, stderr, err := g.run("rebase", "--onto", newBase, oldBase, branch)
	if err != nil {
		return g.wrap(err, fmt.Sprintf("rebase --onto %s %s %s", newBase, oldBase, branch), stderr)
	}
	return nil
}

func (g *Git) RevParse(ref string) (string, error) {
	out, stderr, err := g.run("rev-parse", ref)
	if err != nil {
		return "", g.wrap(err, "rev-parse "+ref, stderr)
	}
	return strings.TrimSpace(string(out)), nil
}

func (g *Git) ConfigGet(key string) (string, error) {
	out, stderr, err := g.run("config", "--get", key)
	if err != nil {
		return "", g.wrap(err, "config --get "+key, stderr)
	}
	return strings.TrimSpace(string(out)), nil
}

func (g *Git) ConfigSet(key, value string) error {
	_, stderr, err := g.run("config", key, value)
	if err != nil {
		return g.wrap(err, "config "+key, stderr)
	}
	return nil
}

func (g *Git) UpdateRef(refname, sha string) error {
	_, stderr, err := g.run("update-ref", refname, sha)
	if err != nil {
		return g.wrap(err, "update-ref "+refname, stderr)
	}
	return nil
}

func (g *Git) ListLocalBranches() ([]string, error) {
	out, stderr, err := g.run("for-each-ref", "--format=%(refname:short)", "refs/heads")
	if err != nil {
		return nil, g.wrap(err, "for-each-ref refs/heads", stderr)
	}
	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return nil, nil
	}
	return strings.Split(trimmed, "\n"), nil
}
