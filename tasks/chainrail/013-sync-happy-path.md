---
task: 013
status: in-progress
depends-on: [001, 002, 003, 004, 005, 007, 008, 011]
---

# 013 — sync (happy path)

## Goal

Implement `chainrail sync` for the case where parents haven't been merged. Cascade-rebases each branch onto the latest tip of its parent, force-pushes with lease.

## Files

- **Modify:** `cmd/sync.go`
- **New/overwrite:** `cmd/sync_test.go`

## Behavior

1. Sanity checks (same as submit).
2. `git.Fetch()` to update remote-tracking refs.
3. Compute the stack via `stack.Walk`.
4. For each layer bottom-up *except* the bottom (which doesn't need rebasing — its base is trunk and we leave that to the user):
   - Wait — actually rethink. The bottom of the stack rebases onto trunk too if trunk has moved. Pseudocode:

     ```
     for i, layer in enumerate(chain):
         parent = trunk if i == 0 else chain[i-1].Branch
         git.Checkout(layer.Branch)
         git.Rebase("origin/" + parent)
         if rebase failed with conflict: return CodeRebaseConflict with paused state
     ```
5. After all rebases succeed:
   - For each layer: write a snapshot ref `refs/chainrail/snapshot/<branch>` pointing at the pre-rebase tip (collected at start of step 4).
   - `git.PushWithLease` each branch.
6. Restore the original current branch.

## Conflict handling for v0.1

When a rebase fails:
- Leave the rebase state in place (don't auto-abort).
- Return `CodeRebaseConflict` with `Suggestion: "resolve conflicts, then 'git add' the files and 'git rebase --continue', then re-run cn sync"`.
- **Defer** `cn continue`/`cn abort` commands to v0.1.1. v0.1 expects the user to use git directly to resolve and then re-run sync.

Snapshot refs let the user do `git update-ref refs/heads/<branch> refs/chainrail/snapshot/<branch>` to recover, but we don't ship a `cn abort` wrapper in v0.1.

## Acceptance

- [ ] Sync on a 3-branch stack where trunk has advanced: rebases all three, force-pushes all three, no errors.
- [ ] Sync with no remote changes: still runs rebase (it's a no-op fast-forward in this case), still force-pushes (also a no-op since the SHA didn't change), exits cleanly.
- [ ] On rebase conflict: returns `CodeRebaseConflict`, leaves repo in mid-rebase state, snapshots are written.
- [ ] Snapshot refs exist after a successful sync.
- [ ] All tests use real git in tempdir + MockGhClient.
- [ ] `go test ./cmd/...` passes.

## Out of scope

- Squash-merged-parent detection — that's task 014.
- `cn continue`/`cn abort` commands.
- Interactive conflict resolution.
