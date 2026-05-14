---
task: 004
status: todo
depends-on: [002]
---

# 004 — MockGhClient for tests

## Goal

In-memory fake `GitHubClient` for use by all command tests. Authorized fake — every other test that needs a `GitHubClient` uses this one.

## Files

- **New:** `internal/github/mock.go` — `MockGhClient` struct
- **New:** `internal/github/mock_test.go` — tests verifying the mock itself behaves

## Implementation notes

```go
package github

type MockGhClient struct {
    User    string
    PRs     map[int]PullRequest
    NextNum int // auto-increment for CreatePR
    // optionally: a Calls log for assertion in tests
}

func NewMock() *MockGhClient {
    return &MockGhClient{User: "test-user", PRs: map[int]PullRequest{}, NextNum: 100}
}
```

Implement each interface method against the in-memory map. `CreatePR` allocates the next PR number, stores the PR with `State: "OPEN"`, returns it. `UpdatePRBody` and `UpdatePRBase` mutate the stored PR.

The mock should expose a `Calls` slice or similar so tests can assert on what was called and in what order. Keep this simple — a `[]string` of `"CreatePR(head=X,base=Y)"` is enough.

## Acceptance

- [ ] `MockGhClient` implements `GitHubClient` (compile-time check).
- [ ] Round-trip test: `CreatePR` → `GetPR(num)` returns the same PR.
- [ ] Round-trip test: `CreatePR` → `UpdatePRBody` → `GetPR` returns the new body.
- [ ] `ListOpenPRs` returns only PRs with `State == "OPEN"`.
- [ ] Test that `Calls` (or equivalent) records the operations in order.
- [ ] `go test ./internal/github/...` passes.

## Out of scope

- Network simulation, latency, error injection beyond the basics.
- Persistence — pure in-memory.
