---
task: 001
status: todo
depends-on: []
---

# 001 — Cobra subcommand scaffold

## Goal

Wire up empty `init`, `add`, `submit`, `sync` subcommands under the cobra root so the CLI surface is complete and routable, even if each just prints "not implemented" for now.

## Files

- **New:** `cmd/init.go`, `cmd/add.go`, `cmd/submit.go`, `cmd/sync.go`
- **New:** `cmd/init_test.go`, `cmd/add_test.go`, `cmd/submit_test.go`, `cmd/sync_test.go`
- **Modify:** `cmd/root.go` (register subcommands in an `init()` block)

## Implementation notes

Each command file exports a `*cobra.Command` and registers itself with `rootCmd.AddCommand(...)` in its `init()`. Each `RunE` for now should return `errors.New("not implemented")` — a real `error`, not `nil`. That makes the cobra wiring testable.

`cmd/add.go` should accept exactly one positional arg (the slug). The others take no positional args.

## Acceptance

- [ ] `go build ./...` succeeds.
- [ ] `chainrail init`, `chainrail add foo`, `chainrail submit`, `chainrail sync` all exit non-zero with "not implemented" stderr.
- [ ] `chainrail add` (no arg) exits non-zero with cobra's usage message.
- [ ] One test per command verifies it's wired into the root (`rootCmd.Commands()` contains it) and exits non-zero on invocation.
- [ ] `go vet ./...` passes.
- [ ] `gofmt -l .` produces no output.

## Out of scope

- Any actual logic. This task only wires the cobra tree.
- Renderer or error-code integration. That's tasks 005/007.
