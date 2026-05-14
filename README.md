# chainrail

A stacked-PR CLI for GitHub. Manage chains of dependent pull requests without the manual git rebase dance.

Stopgap until GitHub's native `gh stack` ships GA. Built in Go, wraps the `gh` CLI, agent-friendly by design.

## Status

v0.1 in development. Not yet released.

## Install

```bash
go install github.com/brayschurman/chainrail@latest
# binary lands as `chainrail`; alias to `cn` in your shell:
alias cn=chainrail
```

Requires `gh` CLI installed and authenticated (`gh auth status`).

## Commands

- `cn init --base <trunk>` — set up chainrail in a repo
- `cn add <slug>` — create the next branch in the stack
- `cn submit` — push branches and open/update PRs with correct bases
- `cn sync` — cascade-rebase the stack onto fresh trunk; handles squash-merged parents

## Why

When a parent PR is squash-merged, GitHub creates a new SHA on trunk. Children of that parent then show all the parent's changes again as "new" — producing dozens of false conflicts. `cn sync` detects this and does the right `git rebase --onto <squash> <old_tip> <child>` automatically.

## License

MIT
