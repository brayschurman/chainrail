---
name: chainrail
description: Manage stacked GitHub PRs from the CLI. Use when the user asks for stacked PRs, splitting a big PR into reviewable layers, or cascading rebases — especially when a parent PR was squash-merged into trunk.
---

# chainrail

Use `chainrail` (alias `cn`) to organize work as a chain of small PRs instead of one big PR. Particularly valuable when squashing parent PRs would normally break the children — `cn sync` automatically rewrites the children's history onto the squash commit.

## When to use this skill

Trigger on any of:

- "make this a stack"
- "split this into stacked PRs"
- "I want small reviewable PRs"
- "stack of PRs"
- The user describes a feature that naturally decomposes into clear layers (schema → API → UI; refactor → feature → tests; preparation → main change → cleanup)
- The user is dealing with conflicts after squash-merging a parent PR (this is `cn sync`'s killer feature)

## Workflow

1. From the trunk branch (e.g. `main` or `staging`), run `cn add <slug>` for the first layer.
2. Implement the layer, commit normally.
3. `cn add <next-slug>` for the next layer — chainrail auto-stacks on top of the current branch.
4. Repeat for each layer.
5. `cn submit` opens all PRs with correct base chains and writes a stack-map into each PR body.
6. On review feedback: checkout the affected branch, fix, commit, then `cn sync` cascades the change up the stack.
7. After a parent PR is squash-merged into trunk, `cn sync` from any descendant detects the squash and rebases the rest of the stack cleanly — no manual recovery needed.

## Decomposition heuristics

When deciding how to layer a feature, think about what changes together:

- Database / schema changes → bottom of stack
- API / service layer → middle
- UI / route / form → top
- Tests live with the code they test (not their own layer)

Aim for 2–4 layers. 5-deep stacks are rare and usually mean the work should be two separate stacks landed in sequence.

## Footguns

- Never `git checkout` to a non-stack branch between `cn add` calls. chainrail uses the current branch as the implicit parent.
- Always commit before `cn add` — it refuses to run with uncommitted changes (returns `DIRTY_WORKTREE`).
- `cn init --base <trunk>` must run first per repo. The trunk is recorded in `.git/config` under `chainrail.trunk`.
- If a parent PR was squash-merged, just run `cn sync` from any descendant — it handles `git rebase --onto` automatically. **Do not** try to recover manually; running `cn sync` is the correct fix for what would otherwise be a 30+ false-conflict mess.
- On a rebase conflict, chainrail leaves the repo in mid-rebase state with snapshots at `refs/chainrail/snapshot/<branch>`. Resolve, `git add`, `git rebase --continue`, then re-run `cn sync`.

## v0.1 scope

Only `init`, `add`, `submit`, `sync` are implemented. Out of scope for v0.1:

- `cn checkout`, `cn modify`, `cn merge` — use `gh pr merge` for merging.
- Tree-shaped stacks (multiple children of one parent) — linear chains only.
- `--json` flag, `--dry-run` flag, MCP server — planned for v0.1.1.
- Pulling someone else's stack down by PR number.

## Output

v0.1 prints human-readable text. Exit codes:

- `0` — success
- `1` — error; stderr contains `Error [<CODE>]: <message>` and optional `Suggestion: ...`

Canonical error codes (key on these in scripts):

- `NOT_GIT_REPO`, `NO_GH_AUTH`, `TRUNK_MISSING`, `ALREADY_INITIALIZED`
- `DIRTY_WORKTREE`, `NOT_ON_STACK`, `SLUG_TAKEN`
- `GH_CALL_FAILED`, `GIT_CALL_FAILED`
- `SQUASH_DETECTED` (informational), `REBASE_CONFLICT`

`--json` output for fully machine-readable results lands in v0.1.1.
