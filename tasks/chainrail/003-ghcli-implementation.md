---
task: 003
status: todo
depends-on: [002]
---

# 003 — GhCli implementation of GitHubClient

## Goal

Implement `GitHubClient` by shelling out to the `gh` CLI and parsing its JSON output.

## Files

- **New:** `internal/github/ghcli.go` — `GhCli` struct implementing the interface
- **New:** `internal/github/ghcli_test.go` — tests using a fake `exec.Cmd` factory

## Implementation notes

`GhCli` should accept an injected command runner so tests don't actually shell out to `gh`. Pattern:

```go
type runner func(name string, args ...string) ([]byte, error)

type GhCli struct {
    run runner
}

func New() *GhCli {
    return &GhCli{run: defaultRun}
}

func defaultRun(name string, args ...string) ([]byte, error) {
    return exec.Command(name, args...).Output()
}
```

Each method calls `c.run("gh", "pr", "list", "--json", "...", ...)` and parses the JSON. In tests, inject a `runner` that returns canned JSON bytes for the expected args.

Map `gh` exit codes:
- Exit 0 with valid JSON → success
- Exit non-zero → `*errors.ChainrailError{Code: "GH_CALL_FAILED", Message: stderr, Suggestion: "check gh auth status"}`. **You can stub the error import as `fmt.Errorf` until task 007 lands** — leave a `// TODO(007): switch to ChainrailError` comment.

Use these gh subcommands:

| Interface method | gh invocation |
|---|---|
| `CurrentUser` | `gh api user --jq .login` |
| `ListOpenPRs` | `gh pr list --author @me --state open --json number,title,baseRefName,headRefName,state,body,mergeCommit` |
| `GetPR` | `gh pr view <num> --json number,title,baseRefName,headRefName,state,body,mergeCommit` |
| `CreatePR` | `gh pr create --base <base> --head <head> --title <title> --body <body>` (`--draft` if requested), then `gh pr view <new-num> --json ...` |
| `UpdatePRBody` | `gh pr edit <num> --body <body>` |
| `UpdatePRBase` | `gh pr edit <num> --base <newBase>` |

## Acceptance

- [ ] `GhCli` implements `GitHubClient` (compile-time check via `var _ GitHubClient = (*GhCli)(nil)`).
- [ ] Each method has at least one happy-path test and one error-path test using injected runner.
- [ ] `ListOpenPRs` correctly parses an empty array `[]`.
- [ ] `GetPR`'s `MergeCommitSHA` is empty when `mergeCommit` is null in the JSON.
- [ ] `go test ./internal/github/...` passes.
- [ ] `go vet ./...` passes.

## Out of scope

- Caching, retries, rate limiting.
- The `MockGhClient` (task 004).
- Wiring `GhCli` into commands (tasks 009–014).
