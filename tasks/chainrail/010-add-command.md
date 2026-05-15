---
task: 010
status: done
depends-on: [001, 005, 007, 008]
---

# 010 — add command

## Goal

Implement `chainrail add <slug>` end-to-end. Creates the next branch in the stack, named for the user, with auto-incremented position.

## Files

- **Modify:** `cmd/add.go`
- **New/overwrite:** `cmd/add_test.go`

## Branch naming convention

Branches follow `<user>/<base-slug>-<position>-<task-slug>`:

- `<user>` — from `gh api user --jq .login`. Cache at first call.
- `<base-slug>` — for v0.1, derive from the first slug passed to `cn add` on the current stack. Store in `git config chainrail.stack.<user>.basebranch.basename = <base-slug>` so subsequent `cn add` calls know what to use. For simplicity, on first `cn add` from trunk, treat the slug as the base-slug.
- `<position>` — incremented integer (1, 2, 3, …).
- `<task-slug>` — the argument passed to `cn add`.

Example progression:

```
$ git checkout main              # main
$ cn add schema                  # creates bray/schema-1-schema
$ cn add api                     # creates bray/schema-2-api (parent: bray/schema-1-schema)
$ cn add ui                      # creates bray/schema-3-ui (parent: bray/schema-2-api)
```

For v0.1 keep this simple: the **first** `cn add` from trunk establishes `<base-slug>` as the slug given. Subsequent `cn add` calls re-use it.

## Behavior

1. `init` must have been run (`chainrail.trunk` exists in git config). Otherwise `CodeNotOnStack`.
2. `git.IsDirty()` — if true, return `CodeDirtyWorktree, Suggestion: "commit or stash before adding to the stack"`.
3. Determine the current branch.
4. If current branch is trunk: this is the first `add`. Set base-slug = the given slug, position = 1.
5. If current branch matches `<user>/<base-slug>-N-*`: parse N, increment to N+1.
6. Otherwise: return `CodeNotOnStack` with suggestion `git checkout <trunk> first, or onto an existing stack branch`.
7. New branch name = `<user>/<base-slug>-<N+1>-<slug>`.
8. If branch already exists locally: return `CodeSlugTaken`.
9. `git.CreateBranch(name)`.
10. Print success.

## Acceptance

- [ ] First `cn add foo` from trunk creates `<user>/foo-1-foo`.
- [ ] Second `cn add bar` from `<user>/foo-1-foo` creates `<user>/foo-2-bar`.
- [ ] `cn add baz` from a non-stack, non-trunk branch returns `CodeNotOnStack`.
- [ ] `cn add` with dirty worktree returns `CodeDirtyWorktree`.
- [ ] `cn add foo` when `<user>/foo-1-foo` already exists returns `CodeSlugTaken`.
- [ ] Tests cover all paths using tempdir repos.
- [ ] `go test ./cmd/...` passes.

## Out of scope

- Pushing the branch. That's `submit`'s job.
- Anything to do with PRs. The branch is local-only after `cn add`.
