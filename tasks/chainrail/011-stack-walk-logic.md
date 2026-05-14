---
task: 011
status: in-progress
depends-on: [002]
---

# 011 — Stack-walking logic

## Goal

Pure function that, given the current branch and the list of open PRs from GitHub, returns the ordered chain of branches from trunk → current. This is the shared engine for `submit` and `sync`.

## Files

- **New:** `internal/stack/walk.go`
- **New:** `internal/stack/walk_test.go`

## Function signature

```go
package stack

import "github.com/brayschurman/chainrail/internal/github"

type Layer struct {
    Branch string
    PR     *github.PullRequest // nil if no PR opened yet
    Parent string // parent branch name; "" if parent is trunk
}

// Walk returns the chain of branches from the bottom of the stack (closest to trunk)
// up to and including currentBranch. The first element's Parent is the trunk.
//
// localBranches is the set of branches that exist locally — used to identify which
// PRs belong to the user's stack vs. unrelated open PRs.
func Walk(currentBranch, trunk string, prs []github.PullRequest, localBranches map[string]bool) ([]Layer, error)
```

## Algorithm

1. Build a map of `BaseRefName → []PullRequest` from `prs`.
2. Build a reverse map: `HeadRefName → PullRequest` so you can find the PR for any local branch.
3. Start at `currentBranch`. Walk *down* to the trunk:
   - For each branch B: its parent is the `BaseRefName` of the PR whose `HeadRefName == B`. If there's no PR for B, use the local branch's upstream (fall back to git logic outside this function — this function operates on the data passed in).
   - Stop when the parent is the trunk.
4. Reverse the collected branches so the result is bottom-up.

Edge cases the test must cover:
- Current branch is the trunk → return empty slice (not an error).
- Current branch has no PR but its parent (via naming convention `<user>/<base>-N-*` decremented to N-1) does → still construct the chain. *For v0.1 simplicity, you may require all branches to have PRs to be considered part of the stack.* Document the constraint.
- Two unrelated open PRs in the repo → only the chain reachable from `currentBranch` is returned.

## Acceptance

- [ ] Walk returns the correct ordered chain for a 3-PR stack.
- [ ] Walk returns an empty slice when current branch is the trunk.
- [ ] Walk handles a 1-PR stack.
- [ ] Walk ignores unrelated open PRs.
- [ ] Walk returns an error or empty slice when the current branch has no PR and is not the trunk — document which (recommend: return empty slice with no error; callers handle the "you're not in a stack" case).
- [ ] `go test ./internal/stack/...` passes.

## Out of scope

- Reading anything from disk or the network. Pure function.
- Detecting squash-merged parents (that's part of task 014's `sync` logic, using the data this function returns).
