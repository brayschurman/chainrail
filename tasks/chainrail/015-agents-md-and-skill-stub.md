---
task: 015
status: todo
depends-on: []
---

# 015 — AGENTS.md and Claude skill stub

## Goal

Ship the two agent-facing docs that make `chainrail` agent-friendly from day one.

## Files

- **New:** `AGENTS.md` at repo root
- **New:** `.claude/skills/chainrail/SKILL.md`

## AGENTS.md content

```markdown
# AGENTS.md

`chainrail` is a Go CLI for managing stacked GitHub PRs. Source of truth: `docs/features/chainrail/CONTEXT.md`.

## When working on this codebase

- Run `go test ./...` before committing.
- Run `go vet ./...` and `gofmt -w .` before committing.
- New commands go in `cmd/<name>.go` and register themselves via `init()`.
- All GitHub calls go through the `GitHubClient` interface in `internal/github/`. Never call `exec.Command("gh", ...)` from command code.
- All output goes through `internal/output.Renderer` against an `io.Writer`. Never call `fmt.Println` from command code.
- All errors returned from library code are `*errors.ChainrailError` with a code from `internal/errors/codes.go`.
- No new dependencies without explicit approval. Stdlib + cobra only.

## When using `chainrail` as a tool

See `.claude/skills/chainrail/SKILL.md` for trigger phrases, workflow, and footguns.

Every command supports a flagless mode (human output) — v0.1.1 will add `--json` for structured output. For now, parse stderr/exit code: `0` success, `1` transient, `2` permanent, `3` user input needed.
```

## .claude/skills/chainrail/SKILL.md content

```markdown
---
name: chainrail
description: Manage stacked GitHub PRs from the CLI. Use when the user asks for stacked PRs, splitting a big PR, or organizing work into reviewable layers.
---

# chainrail

Use `chainrail` (alias `cn`) to organize work as a chain of small PRs instead of one big PR.

## When to use this skill

Trigger on any of:
- "make this a stack"
- "split this into stacked PRs"
- "I want small reviewable PRs"
- The user is working on a feature that decomposes into clear layers (schema, API, UI) and is worried about review burden

## Workflow

1. From the trunk branch, `cn add <slug>` for the first layer.
2. Implement the layer, commit.
3. `cn add <next-slug>` for the next layer (auto-stacks on top of the current branch).
4. Repeat for each layer.
5. `cn submit` opens all PRs with correct bases and writes a stack-map into each PR body.
6. On review feedback: checkout the affected branch, fix, commit, then `cn sync` to cascade.

## Decomposition heuristics

When deciding stack layers for a feature:
- Database / schema changes → bottom of stack
- API / service layer → middle
- UI / route / form → top
- Tests live with the code they test (not their own layer)

Aim for 2–4 layers. Five-deep stacks are rare and usually mean the work should be two separate stacks.

## Footguns

- Never `git checkout` between `cn add` calls — chainrail uses the current branch as the implicit parent.
- Always commit before `cn add` — refuses to run with uncommitted changes.
- If a parent PR was squash-merged, just run `cn sync` from any child — it handles `git rebase --onto` automatically. No manual recovery needed.

## v0.1 scope

Only `init`, `add`, `submit`, `sync` are implemented. `checkout`, `modify`, `merge`, navigation commands, tree-shaped stacks: not yet.

## Output

v0.1 prints human-readable text. Parse exit codes:
- 0: success
- 1: transient (network blip — safe to retry)
- 2: permanent error (don't retry — read stderr)
- 3: user input needed (a conflict, or a required flag)

v0.1.1 will add `--json` for structured output.
```

## Acceptance

- [ ] Both files exist at the paths above.
- [ ] `AGENTS.md` documents the seams (GitHubClient, Renderer, ChainrailError).
- [ ] Skill file has frontmatter with `name` and `description`.
- [ ] No tests required (docs only).

## Out of scope

- Real skill content (decomposition examples, etc.) — fill in after dogfooding.
- MCP server documentation — that's v0.1.1.
