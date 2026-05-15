# chainrail v0.1 — Tasks

**Read first, every iteration:**

- `docs/features/chainrail/CONTEXT.md`
- `docs/features/chainrail/chainrail-rules.md`

## How this folder works

Each `NNN-*.md` file is a single task. Tasks are designed to be picked up sequentially by the ralph loop.

Frontmatter:

```yaml
---
task: 001
status: todo # → in-progress → done | blocked
depends-on: [] # other task numbers that must be `done` first
---
```

Lifecycle:

1. Loop picks lowest-numbered `todo` whose `depends-on` are all `done`.
2. Marks `in-progress`, commits.
3. Implements with TDD, runs tests.
4. Re-reads the task's `## Acceptance` section, confirms every checkbox.
5. Marks `done`, commits, exits.

Blocker path: if stuck after honest effort, mark `status: blocked`, add a `## Blocker` section explaining what failed, commit, exit. Next iteration skips it.

Done condition: when every task is `done`, run `go test ./...` and `go vet ./...`. If green, output `<promise>CHAINRAIL_COMPLETE</promise>` and exit.

## Task dependency graph

```
001 (cobra scaffold) ──┬── 005 (Renderer) ──── 007 (Errors) ──┬─── 009 (init)
                       │                                       ├─── 010 (add)
002 (GhClient iface) ──┼── 003 (GhCli)                         │
                       ├── 004 (MockGhClient)                  │
                       └── 011 (stack-walk) ──────────────────┤
                                                               ├─── 012 (submit)
006 (TTY)                                                      │
008 (git helpers) ────────────────────────────────────────────┤
                                                               └─── 013 (sync) ─── 014 (squash recovery)

015 (AGENTS.md + skill) — independent
```
