---
task: 009
status: in-progress
depends-on: [001, 005, 007, 008]
---

# 009 — init command

## Goal

Implement `chainrail init --base <trunk>` end-to-end. Verifies the environment, records config, idempotent.

## Files

- **Modify:** `cmd/init.go` — replace "not implemented" with the real impl
- **New:** `cmd/init_test.go` (overwrite the scaffold test from task 001 with real behavior tests)
- **Possibly new:** `cmd/common.go` if shared helpers emerge (renderer construction, etc.)

## Behavior

1. Check `git.IsInsideRepo()` — if false, return `*ChainrailError{Code: CodeNotGitRepo, Suggestion: "run from inside a git repository"}`.
2. Shell out to `gh auth status` via a one-off `exec.Command` (this is the only `gh` call init makes — full client is overkill). On non-zero exit, return `CodeNoGhAuth` with suggestion `run 'gh auth login'`.
3. Check the trunk branch exists locally OR on remote. If neither, return `CodeTrunkMissing`.
4. Check `git config --get chainrail.trunk`. If already set:
   - If same as `--base` value: print "chainrail already initialized for trunk: <name>" and exit 0.
   - If different: return `CodeAlreadyInit` with suggestion "run 'git config --unset chainrail.trunk' first".
5. Otherwise, `git config chainrail.trunk <name>`, print success, exit 0.

## Files in detail

`cmd/init.go`:

```go
var initCmd = &cobra.Command{
    Use:   "init",
    Short: "Initialize chainrail in the current repo",
    RunE: func(cmd *cobra.Command, args []string) error {
        base, _ := cmd.Flags().GetString("base")
        return runInit(cmd.OutOrStdout(), cmd.ErrOrStderr(), base)
    },
}

func init() {
    initCmd.Flags().String("base", "", "the trunk branch (required)")
    initCmd.MarkFlagRequired("base")
    rootCmd.AddCommand(initCmd)
}

func runInit(out, errOut io.Writer, base string) error {
    // implementation here
}
```

`runInit` is the testable entry point.

## Acceptance

- [ ] `chainrail init --base main` in a fresh git repo with `gh auth` works → prints success, sets `chainrail.trunk=main` in git config.
- [ ] Re-running with same `--base` is idempotent → prints "already initialized" and exits 0.
- [ ] Re-running with a *different* `--base` returns `CodeAlreadyInit`.
- [ ] Running outside a git repo returns `CodeNotGitRepo`.
- [ ] Running without `--base` exits non-zero with cobra's required-flag message.
- [ ] Tests use `t.TempDir()` + real `git init` (per task 008's pattern). Mock the `gh auth` call — see notes.
- [ ] `go test ./cmd/...` passes.

## Testing the gh auth call

The cleanest path: extract `checkGhAuth` to its own function that takes a `func() error` for the check. In tests, inject a stub.

## Out of scope

- Storing trunk anywhere other than `git config`.
- Multi-trunk support (one trunk per repo, period).
