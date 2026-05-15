# chainrail 🚂

Stacked PR management for GitHub. No squash-merge conflicts, no manual rebase dance.

A lean CLI that organises your work as a chain of small, reviewable pull requests — and keeps the whole chain consistent when you merge one. Built while GitHub's native `gh stack` is still in private preview.

---

## The problem

Your team uses squash-and-merge. You open a stacked PR. The parent gets merged — GitHub squashes it to a new SHA. Now every child branch shows the entire parent diff as "conflicts." You spend the next hour on `git rebase --onto` gymnastics.

`cn sync` does that rebase for you, automatically, for every child in the chain.

---

## Install

```bash
go install github.com/brayschurman/chainrail@latest
alias cn=chainrail
```

Requires **Go 1.21+** and the **`gh` CLI** installed and authenticated:

```bash
gh auth status
```

---

## Quickstart

```bash
# one-time setup per repo
cn init --base main

# build a stack
cn add dev-42-schema     # creates bray/dev-42-1-schema, checks it out
# ... implement, commit ...
cn add dev-42-api        # creates bray/dev-42-2-api on top
# ... implement, commit ...
cn add dev-42-ui         # creates bray/dev-42-3-ui on top

# open all three PRs with correct bases
cn submit

# someone reviews layer 2, you fix it, layer 3 needs to rebase
cn sync

# layer 1 gets squash-merged into main — cn sync detects the squash SHA
# and runs rebase --onto automatically. no conflicts.
cn sync
```

---

## Commands

**`cn init --base <trunk>`**
Registers the trunk branch in `.git/config`. Run once per repo. Safe to re-run (idempotent).

**`cn add <slug>`**
Creates the next branch in the stack: `<user>/<base-slug>-<N>-<slug>`. Refuses to run with uncommitted changes.

**`cn submit`**
Pushes every branch in the stack, opens pull requests with correct `--base` chains, and injects a stack-map into each PR body so reviewers can navigate. Idempotent — safe to re-run.

**`cn sync`**
Cascade-rebases the stack onto fresh trunk. Detects squash-merged parents and runs `git rebase --onto <squash_sha> <old_tip> <child>` automatically, then flips the PR's base on GitHub. Writes snapshot refs at `refs/chainrail/snapshot/<branch>` before touching anything.

---

## Stack-map in PR bodies

`cn submit` injects a navigation block into every PR description:

```
<!-- chainrail:stack:start -->
**Stack** (bottom → top)
1. bray/dev-42-1-schema ← you are here
2. bray/dev-42-2-api
3. bray/dev-42-3-ui
<!-- chainrail:stack:end -->
```

Re-running `cn submit` updates the block in place — no duplicates.

---

## Exit codes & errors

All errors print to stderr as `Error [CODE]: message` with an optional `Suggestion: ...` line.

| Code | Meaning |
|------|---------|
| `NOT_GIT_REPO` | not inside a git repo |
| `NO_GH_AUTH` | `gh` is not authenticated |
| `TRUNK_MISSING` | `cn init` hasn't been run |
| `DIRTY_WORKTREE` | uncommitted changes; commit or stash first |
| `NOT_ON_STACK` | current branch isn't a chainrail branch |
| `SLUG_TAKEN` | that slug already exists in this stack |
| `REBASE_CONFLICT` | conflict during rebase; resolve and re-run `cn sync` |
| `GH_CALL_FAILED` | the `gh` CLI returned an error |

Exit `0` on success, `1` on any error.

---

## Footguns

- Never `git checkout` to a non-stack branch between `cn add` calls — chainrail uses the current branch as the implicit parent.
- Always commit before `cn add`.
- `cn init --base <trunk>` must run once per clone — it's stored in `.git/config`, not committed.
- On a rebase conflict, chainrail leaves the repo in mid-rebase state. Resolve conflicts, `git add`, `git rebase --continue`, then re-run `cn sync`.
- Snapshot refs live at `refs/chainrail/snapshot/<branch>`. If a sync goes wrong you can recover with `git update-ref refs/heads/<branch> refs/chainrail/snapshot/<branch>`.

---

## v0.1 scope

What's in: `init`, `add`, `submit`, `sync`, squash-merge recovery.

What's not yet: `--json` output, `--dry-run`, `cn checkout`, `cn merge`, tree-shaped stacks (multiple children per parent), pulling someone else's stack by PR number. These are v0.1.1.

---

## Agent-friendly

See `AGENTS.md` for architectural seams and testing patterns. See `.claude/skills/chainrail/SKILL.md` for the Claude skill stub (trigger phrases, decomposition heuristics, footguns).

---

## License

MIT
