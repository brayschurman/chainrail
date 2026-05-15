---
task: 014
status: done
depends-on: [013]
---

# 014 — sync: squash-merged-parent recovery

## Goal

The marquee feature. When a parent PR has been squash-merged into trunk, sync detects it and runs `git rebase --onto <squash_sha> <old_parent_tip> <child>` to drop the now-redundant commits, instead of attempting a naive rebase that would conflict.

## Files

- **Modify:** `cmd/sync.go`
- **Modify:** `cmd/sync_test.go`

## Detection algorithm

For each layer in the stack, *before* doing the normal rebase from task 013:

1. Get the layer's parent PR (the PR whose `HeadRefName` is this layer's parent branch).
2. If that PR's `State == "MERGED"` and `MergeCommitSHA != ""`:
   - The parent has been merged. We need to handle it specially.
   - Get the parent branch's old tip: `oldTip := <parent_branch>` (resolve via `git rev-parse refs/heads/<parent>` — the branch still exists locally even if not on origin).
   - Get the squash commit: `squashSHA := parent PR's MergeCommitSHA`.
   - Run `git.RebaseOnto(squashSHA, oldTip, layer.Branch)`.
   - Also: update the PR's base on GitHub from the merged parent branch → trunk, via `GitHubClient.UpdatePRBase(layer.PR.Number, trunk)`. (GitHub may auto-do this when you merge; do it explicitly to be safe.)
   - Then: this layer's *effective parent* for the rest of sync is now trunk, not the old merged parent. Update the in-memory chain accordingly.
3. If parent PR is `OPEN`: proceed with normal task-013 rebase logic.

## After all layers processed

Continue with the rest of task 013's logic: write snapshots, force-push.

## The crucial test

This is the test that validates the PR #683 fix exists:

```go
func TestSync_SquashMergedParent(t *testing.T) {
    // setup:
    //   trunk: main with commits [A]
    //   branch1: main + [B, C], PR #100, MERGED, MergeCommitSHA = S
    //   branch2: branch1 + [D], PR #101, OPEN, base = branch1
    //   main now has commits [A, S]   (S is the squash of B+C)
    //   branch1 still locally points at the [A, B, C] tip
    //
    // run: sync from branch2
    //
    // expect:
    //   branch2 is now [A, S, D]    (D replayed onto S; B, C dropped)
    //   PR #101's base is now "main" (was "branch1")
    //   No git conflicts during the rebase
}
```

## Acceptance

- [ ] Squash-merged-parent test above passes.
- [ ] Normal happy-path test from task 013 still passes.
- [ ] PR base update happens on GitHub (assertable via MockGhClient `Calls`).
- [ ] If grandparent is also squashed (rare; trunk has *two* squash commits since the chain started): handle correctly. Test this.
- [ ] `go test ./cmd/...` passes.
- [ ] `go test ./...` passes overall.
- [ ] `go vet ./...` passes.

## Out of scope

- Detecting *non-squash* merge methods (merge commit, rebase merge) — for v0.1 just handle squash, since that's the failure mode we care about. Other merge methods don't produce orphaned commits the same way.
- Multiple stacked squash-merges happening between runs (e.g. PRs 1 *and* 2 both got squash-merged) — should still work via the same algorithm walking up the chain, but call it out as worth testing if time allows.
