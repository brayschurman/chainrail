---
task: 002
status: in-progress
depends-on: []
---

# 002 — GitHubClient interface + types

## Goal

Define the `GitHubClient` interface in `internal/github/` along with the data types it returns (`PullRequest`, `MergeCommit`, etc.). No implementation yet — just the contract.

## Files

- **New:** `internal/github/client.go` — interface + types
- **New:** `internal/github/types_test.go` — basic round-trip tests on any type that has serialization-like behavior (e.g., parsing a stack-map block out of a PR body)

## Interface contract

```go
package github

type GitHubClient interface {
    // CurrentUser returns the authenticated user's login.
    CurrentUser(ctx context.Context) (string, error)

    // ListOpenPRs returns all open PRs authored by the current user in the repo
    // identified by the current git remote. Each PR includes BaseRefName, HeadRefName,
    // Number, Title, State, and MergeCommitSHA (empty when not merged).
    ListOpenPRs(ctx context.Context) ([]PullRequest, error)

    // GetPR fetches a single PR by number, including its body, base, head, and merge commit.
    GetPR(ctx context.Context, number int) (PullRequest, error)

    // CreatePR opens a new PR.
    CreatePR(ctx context.Context, p NewPR) (PullRequest, error)

    // UpdatePRBody replaces the body of an existing PR.
    UpdatePRBody(ctx context.Context, number int, body string) error

    // UpdatePRBase changes the base ref of an existing PR.
    UpdatePRBase(ctx context.Context, number int, newBase string) error
}

type PullRequest struct {
    Number          int
    Title           string
    BaseRefName     string
    HeadRefName     string
    State           string // "OPEN" | "CLOSED" | "MERGED"
    Body            string
    MergeCommitSHA  string // empty unless merged
}

type NewPR struct {
    Title string
    Body  string
    Head  string
    Base  string
    Draft bool
}
```

Use `context.Context` even if v0.1 doesn't propagate cancellation — costs nothing and is the right shape.

## Acceptance

- [ ] `internal/github/client.go` defines the interface and types as above.
- [ ] Trivial round-trip test exists (e.g., a struct literal compiles and field access works).
- [ ] `go build ./...` succeeds.
- [ ] `go vet ./...` passes.
- [ ] No implementation of `GitHubClient` in this task — only the interface.

## Out of scope

- Implementation. Tasks 003 (GhCli) and 004 (Mock) do that.
- Stack-map parsing helpers — that's task 011 (stack-walk).
