# chainrail v0.1 — Rules / Acceptance

Per-case rules and the human-verifiable demo at the end.

## Go conventions

- **Stdlib + cobra only.** No new dependencies unless the task explicitly authorizes one. If a task needs a package, mark it `blocked` instead of adding it.
- **Format on save.** Every change must pass `gofmt -l .` (no output) and `go vet ./...`.
- **Errors are typed.** Library code returns `*errors.ChainrailError` from `internal/errors`. Never return bare `fmt.Errorf` from a command function.
- **No `panic()`** in library or command code. Tests can use `t.Fatal` freely.
- **No `os.Exit`** outside `main.go`. Commands return errors; the renderer in main decides the exit code.
- **`io.Writer` everywhere.** Commands accept `out, errOut io.Writer`. No `fmt.Println` direct.
- **No comments narrating code.** Names speak for themselves. Comment only for non-obvious *why* (a workaround, a subtle invariant, an intentional departure). Never comment "what" the code does.
- **Match existing patterns.** If `cmd/init.go` exists, mirror its shape when adding `cmd/add.go`.

## Testing rules

- **Every new file with logic gets a `_test.go` sibling.** No exceptions for `cmd/`, `internal/github/`, `internal/stack/`, `internal/git/`, `internal/errors/`, `internal/output/`.
- **Use stdlib `testing`.** No testify, no ginkgo. Table-driven tests preferred.
- **Integration tests use `t.TempDir()` and a real `git init`.** Spin up a fresh repo per test. Don't share state.
- **Mock the `GitHubClient` interface in tests** — never shell out to real `gh` in tests. The `MockGhClient` (task 004) is the only authorized fake.
- **Test the seams.** Every command's test should exercise it through the cobra `Execute` path with a captured `out` buffer — not by calling internal functions directly. This is what catches the "renderer plumbing got disconnected" class of bug.

## Acceptance — visual check (user-verifiable, run after the loop completes)

A short script you can run when Ralph signals `<promise>CHAINRAIL_COMPLETE</promise>`. Do this in a throwaway test repo, *not* in `virtual-architect`.

```bash
# Build and install
cd ~/maket/chainrail
go build -o ~/.local/bin/chainrail .
export PATH=~/.local/bin:$PATH
alias cn=chainrail

# Create a throwaway test repo
mkdir -p /tmp/chainrail-smoke && cd /tmp/chainrail-smoke
git init -b main
echo "# smoke" > README.md
git add . && git commit -m "init"
gh repo create chainrail-smoke --private --source=. --push
```

Then walk through:

1. **init.** `cn init --base main` → prints success.
2. **idempotent init.** `cn init --base main` again → prints "already initialized, all good," exits 0.
3. **add.** `cn add foo` → creates `bray/foo-1-foo` branch (or similar — see task 010 for the exact pattern). `git branch --show-current` confirms.
4. **commit + add.** Add a file, commit, then `cn add bar` → creates next branch on top.
5. **submit.** `cn submit` → opens 2 PRs on GitHub, second one's base is the first branch.
6. **review feedback cascade.** Switch to the bottom branch, add a commit, push. Then `cn sync` → rebases the top branch onto the new bottom, force-pushes.
7. **squash-merge recovery.** Squash-merge the bottom PR via GitHub UI. Then `cn sync` from the top branch → detects the squash, replays only the top's unique commits onto trunk, force-pushes cleanly.

If all 7 steps work, v0.1 ships.

## Invariants the implementation must enforce

These become assertions in some task's tests:

1. **`cn init`** records `chainrail.trunk` in `.git/config` (idempotent — running it again with the same `--base` is a no-op).
2. **`cn add <slug>`** refuses to run if the working tree is dirty. Returns `ChainrailError{Code: "DIRTY_WORKTREE"}`.
3. **`cn add <slug>`** refuses to run if not on a stack branch or the trunk. Returns `ChainrailError{Code: "NOT_ON_STACK"}`.
4. **`cn submit`** is idempotent — running it twice with no local changes opens 0 new PRs and updates 0 PR bodies.
5. **`cn sync`** never force-pushes the trunk. Trunk is read-only.
6. **`cn sync`** writes a snapshot ref `refs/chainrail/snapshot/<branch>` for every branch it rebases, so the rebase is recoverable via `git update-ref` if something goes wrong.
7. **Stack map block** in PR bodies is bounded by HTML comment markers `<!-- chainrail:stack:start -->` and `<!-- chainrail:stack:end -->`. The tool only edits between those markers.
