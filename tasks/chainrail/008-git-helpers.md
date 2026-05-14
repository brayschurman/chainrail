---
task: 008
status: todo
depends-on: []
---

# 008 — git helpers

## Goal

Wrap the git CLI calls chainrail needs (branch ops, push, rebase) behind a small testable package.

## Files

- **New:** `internal/git/git.go`
- **New:** `internal/git/git_test.go`

## Implementation notes

Same injected-runner pattern as `internal/github/ghcli.go`. Functions to expose:

```go
package git

type Runner func(args ...string) (stdout, stderr []byte, err error)

type Git struct {
    run Runner
    cwd string
}

func New(cwd string) *Git
func NewWithRunner(cwd string, r Runner) *Git // for tests

func (g *Git) CurrentBranch() (string, error)
func (g *Git) IsInsideRepo() bool
func (g *Git) IsDirty() (bool, error)
func (g *Git) BranchExists(name string) (bool, error)
func (g *Git) RemoteExists(branch string) (bool, error) // checks origin/<branch>
func (g *Git) CreateBranch(name string) error           // git checkout -b <name>
func (g *Git) PushWithLease(branch string) error        // git push --force-with-lease origin <branch>
func (g *Git) Fetch() error                              // git fetch origin
func (g *Git) Rebase(target string) error                // git rebase <target>
func (g *Git) RebaseOnto(newBase, oldBase, branch string) error // git rebase --onto <newBase> <oldBase> <branch>
func (g *Git) RevParse(ref string) (string, error)       // returns full sha
func (g *Git) ConfigGet(key string) (string, error)
func (g *Git) ConfigSet(key, value string) error
```

For tests, integration-test the real implementation against `t.TempDir()` repos:

```go
func newTestRepo(t *testing.T) string {
    dir := t.TempDir()
    runMust(t, dir, "git", "init", "-b", "main")
    runMust(t, dir, "git", "config", "user.email", "test@test.com")
    runMust(t, dir, "git", "config", "user.name", "test")
    runMust(t, dir, "sh", "-c", "echo hi > a && git add . && git commit -m init")
    return dir
}
```

Then exercise `Git` against that. This is the *one* package where we use real `git` in tests because mocking git would be insane and `git` is essentially a build dependency.

## Acceptance

- [ ] Every function above exists and has a test.
- [ ] Integration tests use `t.TempDir()` and real `git`.
- [ ] `CurrentBranch` returns "main" on a freshly-initialized repo.
- [ ] `BranchExists` returns true after `CreateBranch`.
- [ ] `IsDirty` returns true after adding an untracked file.
- [ ] `Rebase` and `RebaseOnto` succeed against a 2-commit scenario.
- [ ] `go test ./internal/git/...` passes.

## Out of scope

- Working directory changes (`cd`). All ops use the `cwd` field.
- Conflict-resume UX (that's the sync command's job).
