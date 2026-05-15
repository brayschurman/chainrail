# chainrail v0.1 — Context

Shared-language doc for the v0.1 build. Read first, every iteration.

## Existing system (what's already there)

This is a brand-new repo. At kickoff it contains:

- `main.go` — entry point that delegates to `cmd.Execute()`
- `cmd/root.go` — empty cobra root command
- `go.mod` — module `github.com/brayschurman/chainrail`, depends on `github.com/spf13/cobra`
- `README.md`, `LICENSE` (MIT), `.gitignore`

Nothing else. Each task in `tasks/chainrail/` adds one well-defined piece.

## What chainrail does

A CLI that wraps the `gh` CLI to manage chains of dependent GitHub PRs ("stacked PRs"). Four commands in v0.1:

| Command | Job |
|---|---|
| `chainrail init --base <trunk>` | Verify git repo + `gh auth` + that the trunk branch exists. Record the trunk in `.git/config`. Idempotent. |
| `chainrail add <slug>` | Create a new branch off the current one with a name like `bray/dev-200-N-slug`. Auto-numbers based on the current branch's position in a stack. |
| `chainrail submit` | Push every branch in the current stack and open/update one PR per branch with the correct `--base`. Inject a stack-map block into each PR body. |
| `chainrail sync` | Fetch trunk; cascade-rebase each branch in the stack onto its parent; force-push with lease. **Detect squash-merged parents** and run `git rebase --onto <squash_sha> <old_parent_tip> <child>` to drop the now-redundant commits. |

## New vocabulary (introduced by this feature)

- **Stack** — an ordered chain of branches, each whose parent is either the trunk or the previous branch in the chain. v0.1 only supports linear chains, not trees.
- **Trunk** — the base branch the stack lives on top of (`staging`, `main`, etc.). Configurable per-repo.
- **Stack root** — the bottom-most branch in the stack, whose base is the trunk.
- **Stack tip** — the top-most branch in the stack, where new work currently goes.
- **Squash-merged parent** — a parent branch whose PR was squashed into trunk. The original commits are now orphaned on the child branch and must be rebased away.

## Architecture (the seams that matter)

These seams exist so v0.1.1's agent-friendly features (`--json`, `--dry-run`, MCP server) drop in cleanly. **Respect them when adding code.**

1. **`internal/github.GitHubClient` interface.** Every GitHub call goes through this. `GhCli` implementation shells out to `gh`; `MockGhClient` implementation is for tests. Never call `exec.Command("gh", ...)` directly from command code.
2. **`internal/output.Renderer` interface.** Commands return typed results; the renderer formats them. v0.1 only has `TextRenderer`. v0.1.1 will add `JSONRenderer`. Never call `fmt.Println` from command code.
3. **`io.Writer` everywhere.** Each command function accepts `out, err io.Writer`. Pass `os.Stdout` / `os.Stderr` in `main`. Lets tests capture output and lets future MCP wrapper pipe to a buffer.
4. **Typed error codes.** Errors are `*ChainrailError` with `Code`, `Message`, `Suggestion`. The renderer formats them; tests assert on `Code`, not on the rendered text.
5. **Idempotent commands.** `init` on an already-initialized repo prints "already initialized" and exits 0. `submit` updates PRs in place if they exist. `add` errors cleanly if the slug is already used.
6. **No interactive prompts.** Ever. If something would normally prompt, exit with a structured error and a `suggestion`.

## Key files this feature will touch

| Concern | Path |
| --- | --- |
| Entry point | `main.go` (rarely touched after task 001) |
| Cobra command tree | `cmd/root.go`, `cmd/init.go`, `cmd/add.go`, `cmd/submit.go`, `cmd/sync.go` |
| GitHub client interface | `internal/github/client.go` |
| `gh` CLI implementation | `internal/github/ghcli.go` |
| Test mock | `internal/github/mock.go` |
| Stack-walking logic | `internal/stack/walk.go` |
| Output renderer | `internal/output/renderer.go` |
| TTY detection | `internal/term/tty.go` |
| Typed errors | `internal/errors/codes.go` |
| Git helpers | `internal/git/git.go` |
| Agent docs | `AGENTS.md`, `.claude/skills/chainrail/SKILL.md` |

## External dependencies

- `github.com/spf13/cobra` — CLI framework. Already in `go.mod`.
- Go stdlib only beyond that. No additional packages unless a task explicitly authorizes one. If you think you need a package, mark the task `blocked` instead.

## Out of scope (v0.1)

Hard line on what NOT to touch:

- `cn checkout`, `cn modify`, `cn merge`, navigation commands (`up`/`down`/`top`/`bottom`)
- Tree-shaped stacks (multiple children of one parent) — linear chains only
- Direct GitHub API client (use `gh` CLI; the interface stays swappable)
- `goreleaser` / release distribution setup
- `--json` flag, `--dry-run` flag, MCP server — architectural seams only; the actual surface waits for v0.1.1
- `cn status` — the `status` command itself is v0.1.1 (the stack-walking *logic* it would use is built in v0.1 for `submit`/`sync` to share)
