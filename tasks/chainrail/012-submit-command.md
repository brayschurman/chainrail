---
task: 012
status: in-progress
depends-on: [001, 002, 003, 004, 005, 007, 008, 011]
---

# 012 — submit command

## Goal

Implement `chainrail submit` end-to-end. Pushes every branch in the stack and opens/updates one PR per branch with the correct `--base` and a stack-map block in the body.

## Files

- **Modify:** `cmd/submit.go`
- **New/overwrite:** `cmd/submit_test.go`

## Behavior

1. Sanity checks: in git repo, init has been run (trunk known), `gh auth` valid (call `GitHubClient.CurrentUser`), worktree clean.
2. Compute the stack:
   - Determine current branch.
   - Fetch open PRs via `GitHubClient.ListOpenPRs`.
   - Call `stack.Walk` to get the ordered chain.
   - If `len(chain) == 0`: return `CodeNotOnStack`.
3. For each layer in the chain, bottom-up:
   - `git.PushWithLease(layer.Branch)`.
   - If `layer.PR == nil` (no PR yet): `gh.CreatePR(NewPR{Head: layer.Branch, Base: layer.Parent or trunk, Title: derive-from-branch-name, Body: ""})`.
   - If `layer.PR != nil`: do nothing for create.
4. After all PRs exist, build the stack-map markdown block:

   ```markdown
   <!-- chainrail:stack:start -->
   ## Stack

   - #<num1> (1/N) <branch1>
   - #<num2> (2/N) <branch2> ← you are here (if applicable)
   - …

   <sub>Maintained by [chainrail](https://github.com/brayschurman/chainrail). Edit above and below freely.</sub>
   <!-- chainrail:stack:end -->
   ```

5. For each PR: read existing body, replace the block between markers (or prepend if not present), `gh.UpdatePRBody`.

## Idempotency

Re-running `submit` with no local changes must result in 0 created PRs and ≤ N body updates (only updates a body if the stack-map content differs). Test this explicitly.

## Acceptance

- [ ] First `submit` on a 3-branch stack: pushes 3 branches, creates 3 PRs with correct `--base` chain, body contains stack-map block.
- [ ] Re-running `submit` with no changes: creates 0 PRs. Body updates only if content changed.
- [ ] Stack-map block in body has start/end markers and lists all branches with correct PR numbers.
- [ ] `submit` from trunk (not on a stack branch) returns `CodeNotOnStack`.
- [ ] All tests use `MockGhClient` — no real `gh` calls.
- [ ] `go test ./cmd/...` passes.

## Out of scope

- Drafting PRs (`--draft` flag) — defer to v0.1.1.
- Updating PR titles after creation.
- Handling the case where a teammate already opened a PR for one of your branches with a different number (unusual; defer to v0.1.1).
