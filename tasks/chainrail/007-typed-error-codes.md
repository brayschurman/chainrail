---
task: 007
status: todo
depends-on: [005]
---

# 007 — Typed error codes

## Goal

Replace ad-hoc errors with a structured `*ChainrailError` carrying `Code`, `Message`, `Suggestion`. The renderer formats it for humans; v0.1.1's `JSONRenderer` will format it for agents.

## Files

- **New:** `internal/errors/codes.go`
- **New:** `internal/errors/codes_test.go`
- **Modify:** `internal/output/renderer.go` — teach `TextRenderer.Error` to format `*ChainrailError` nicely. Clean up the `TODO(007)` left in task 005.
- **Modify:** `internal/github/ghcli.go` — replace the `fmt.Errorf("GH_CALL_FAILED...")` from task 003 with `*ChainrailError`. Clean up the `TODO(007)` left in task 003.

## Implementation notes

```go
package errors

type ChainrailError struct {
    Code       string
    Message    string
    Suggestion string
    Cause      error
}

func (e *ChainrailError) Error() string {
    if e.Suggestion != "" {
        return e.Message + " (suggestion: " + e.Suggestion + ")"
    }
    return e.Message
}

func (e *ChainrailError) Unwrap() error { return e.Cause }

// Canonical codes — keep this list short and stable, agents will key on these
const (
    CodeNotGitRepo       = "NOT_GIT_REPO"
    CodeNoGhAuth         = "NO_GH_AUTH"
    CodeTrunkMissing     = "TRUNK_MISSING"
    CodeAlreadyInit      = "ALREADY_INITIALIZED"
    CodeDirtyWorktree    = "DIRTY_WORKTREE"
    CodeNotOnStack       = "NOT_ON_STACK"
    CodeSlugTaken        = "SLUG_TAKEN"
    CodeGhCallFailed     = "GH_CALL_FAILED"
    CodeGitCallFailed    = "GIT_CALL_FAILED"
    CodeSquashDetected   = "SQUASH_DETECTED" // informational; not always an error
    CodeRebaseConflict   = "REBASE_CONFLICT"
)
```

`TextRenderer.Error`:
- If err is `*ChainrailError` with `Suggestion`: render as `Error [CODE]: <Message>\n  Suggestion: <Suggestion>\n`.
- Otherwise: render as `Error: <err.Error()>`.

## Acceptance

- [ ] `*ChainrailError` implements `error` and `Unwrap`.
- [ ] All canonical codes defined as exported constants.
- [ ] `TextRenderer.Error` formats `*ChainrailError` with code and suggestion when present.
- [ ] All `TODO(007)` comments from earlier tasks removed.
- [ ] `go test ./...` passes.

## Out of scope

- Adding more codes than the canonical list. New codes get added by the tasks that need them.
