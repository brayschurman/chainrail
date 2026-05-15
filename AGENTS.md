# AGENTS.md

`chainrail` is a Go CLI for managing stacked GitHub PRs. Source of truth for the v0.1 design: `docs/features/chainrail/CONTEXT.md`.

## When working on this codebase

- Run `go test ./...` before committing.
- Run `go vet ./...` and `gofmt -w .` before committing.
- New commands go in `cmd/<name>.go` and register themselves with `rootCmd.AddCommand` in their `init()`.
- All GitHub calls go through the `GitHubClient` interface in `internal/github/`. Never call `exec.Command("gh", ...)` from command code — only `internal/github/ghcli.go` may shell out to `gh`.
- All output goes through `internal/output.Renderer` against an `io.Writer`. Never call `fmt.Println` from command or library code.
- All errors returned from library code are `*errors.ChainrailError` (alias `crerrors`) with a code from `internal/errors/codes.go`.
- Tests for code that touches git use the `t.TempDir()` + real-git pattern; see `internal/git/git_test.go` for the canonical setup helper.
- Tests for code that touches the `gh` CLI use the injected-runner pattern; see `internal/github/ghcli_test.go`.
- Tests for command-layer behavior inject a `*github.MockGhClient` for the `GitHubClient` dependency. Never shell out to real `gh` in tests.
- No new dependencies without explicit approval. Stdlib + `github.com/spf13/cobra` only.

## When using `chainrail` as a tool

See `.claude/skills/chainrail/SKILL.md` for trigger phrases, workflow, and footguns.

Every command currently prints human-readable text. Parse exit codes:

- `0` — success
- `1` — error; read stderr for `Error [<CODE>]: <message>` and optional `Suggestion: ...` lines

`--json` output and `--dry-run` flags are planned for v0.1.1 — the architectural seams (typed `Renderer` interface, typed `ChainrailError` codes, `io.Writer` everywhere) are already in place.

## Build & install

```
go install github.com/brayschurman/chainrail@latest
# binary is `chainrail`; alias to `cn` in your shell
alias cn=chainrail
```

Requires `gh` CLI installed and authenticated (`gh auth status`).
