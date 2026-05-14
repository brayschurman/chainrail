---
active: true
iteration: 10
session_id: 719c9cdf-6119-4855-8812-c9eacd97b0f8
max_iterations: 20
completion_promise: "CHAINRAIL_COMPLETE"
started_at: "2026-05-14T23:34:52Z"
---

# Ralph loop prompt — chainrail v0.1

You are working on chainrail v0.1, a Go CLI for stacked GitHub PRs. Your job each iteration is to complete **one** task from `tasks/chainrail/` and then exit.

## Read first (every iteration)

1. `docs/features/chainrail/CONTEXT.md` — domain language, existing patterns, file map, architectural seams
2. `docs/features/chainrail/chainrail-rules.md` — Go conventions, testing rules, invariants, visual-check acceptance
3. `tasks/chainrail/README.md` — how the task folder works and the dependency graph

## Per-iteration procedure

1. **Pick the task.** Open `tasks/chainrail/` and list the `NNN-*.md` files. Find the **lowest-numbered** task whose frontmatter has `status: todo`. Verify all of its `depends-on` tasks are `status: done`. If the lowest-numbered todo has unmet deps, that's a bug — note it and exit. If everything is `done`, jump to step 7.

2. **Claim it.** Update the task's frontmatter to `status: in-progress`. Commit with message `chore(chainrail): claim task NNN`.

3. **Implement it (TDD).**
   - Read the task file fully.
   - Find existing analogous code (the task will usually point at it; otherwise search the repo).
   - Write the failing test(s) first. Run them, confirm they fail for the right reason.
   - Implement the smallest change that makes them pass.
   - Run `go vet ./...` and `go test ./...`.
   - If tests still fail, fix them. If you've tried the same fix 3 times and it's still broken, go to step 6 (blocker).

4. **Verify it.** Re-read the task's "Acceptance" section. Confirm every checkbox. If anything is missing, fix it.

5. **Mark done + commit.** Update the task's frontmatter to `status: done`. Stage all changes. Commit with message `feat(chainrail): NNN — <task subject>`. Then **exit**.

6. **Blocker path.** If you can't complete the task after honest effort:
   - Update the task's frontmatter to `status: blocked`.
   - Add a `## Blocker` section to the task file describing what failed, what you tried, and what would unblock it.
   - Commit with message `chore(chainrail): block task NNN — <one-line reason>`.
   - Exit. The next iteration will pick the next-lowest todo.

7. **Done condition.** If every task in `tasks/chainrail/` is `status: done`:
   - Re-read the rules doc's "Acceptance" section.
   - Run `go vet ./...` and `go test ./...` from repo root. If anything fails, treat as a new task: open the failing task, switch it back to `in-progress`, fix, re-mark `done`, commit, exit.
   - Run `gofmt -l .` — must produce no output. If it does, run `gofmt -w .`, commit as `chore(chainrail): gofmt`, exit.
   - If all tests are green and gofmt is clean, output exactly: `<promise>CHAINRAIL_COMPLETE</promise>` and exit.

## Hard rules

- **One task per iteration.** Do not work on task NNN+1 in the same iteration as task NNN.
- **Always commit before exit.** Never leave uncommitted work — the next iteration starts with a fresh context and won't know what was in progress.
- **Stay on the branch you're given.** Don't merge, push to GitHub, or switch branches. (Local branch switching for testing is fine; just return to the starting branch before commit.)
- **Respect the out-of-scope list** in CONTEXT.md.
- **No new dependencies.** stdlib + cobra only. If you think you need a package, mark the task `blocked` instead.
- **No `fmt.Println` in command or library code.** Use the renderer. No `os.Exit` outside `main.go`.
- **No comments explaining what code does** — names should be enough. Only comment non-obvious *why*.
- **Every new file with logic gets a `_test.go` sibling.** No exceptions for `cmd/`, `internal/`.
- **All GitHub calls go through `internal/github.GitHubClient`.** Never `exec.Command("gh", ...)` from command code (only from `internal/github/ghcli.go`).

## When you're stuck

- Read the existing analog the task or CONTEXT.md points at.
- Check `internal/github/ghcli_test.go` for the injected-runner test pattern (any task involving an external CLI uses this pattern).
- Check `internal/git/git_test.go` for the tempdir-real-git test pattern.

## Output style

- Concise commit messages.
- No long prose summaries — your work is in the diff and the task frontmatter.
