# chainrail 🚂

Ship smaller PRs. Review faster. Never fight a squash-merge conflict again.

[![Latest release](https://img.shields.io/github/v/release/brayschurman/chainrail?style=for-the-badge)](https://github.com/brayschurman/chainrail/releases)
[![MIT License](https://img.shields.io/badge/License-MIT-blue.svg?style=for-the-badge)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?style=for-the-badge&logo=go)](https://go.dev)

Chainrail organises your work into a chain of small, reviewable pull requests and keeps the whole chain healthy when things change — including when a parent PR gets squash-merged and GitHub would normally explode with false conflicts.

---

## The problem 😤

Your team uses squash-and-merge. You build a stacked PR. The parent gets merged — GitHub squashes it to a new commit. Now every child branch shows the entire parent diff as "conflicts." You spend the next hour doing git surgery.

`cn sync` does that surgery for you, automatically, every time.

---

## Install

```bash
go install github.com/brayschurman/chainrail@latest
alias cn=chainrail
```

Requires **Go 1.21+** and the **`gh` CLI** installed and authenticated.

---

## See your stack at a glance ✨

Run `cn status` on any branch in your stack:

```
╭──────────────────────────────────────────────────────────────╮
│                                                              │
│   chainrail · dev-42                                         │
│   ──────────────────────────────────────────────────         │
│                                                              │
│     1  bray/dev-42-1-schema    #12  ✓ merged                 │
│   ▶ 2  bray/dev-42-2-api       #13  ● open                   │
│     3  bray/dev-42-3-ui        #14  ● open  ⚠ needs sync     │
│                                                              │
│     q quit                                                   │
│                                                              │
╰──────────────────────────────────────────────────────────────╯
```

Color-coded PR states, sync warnings, and your current position — all live from GitHub. No more keeping the stack in your head.

---

## Quickstart 🚀

```bash
# one-time setup per repo
cn init --base main

# build a stack, one layer at a time
cn add feature-schema   # creates your-name/feature-1-schema and checks it out
# ... write code, commit ...
cn add feature-api      # stacks on top
# ... write code, commit ...
cn add feature-ui       # one more layer
# ... write code, commit ...

# open all PRs at once, with correct bases
cn submit

# parent PR got squash-merged? just run:
cn sync                 # chainrail figures out the rest
```

---

## Commands

**`cn init --base <trunk>`** — set up chainrail in your repo (once per clone)

**`cn add <slug>`** — create the next branch in your stack

**`cn submit`** — push all layers and open PRs with correct base chains

**`cn sync`** — rebase everything onto fresh trunk; handles squash-merged parents automatically

**`cn status`** — open the TUI to see your full stack health at a glance

---

## Stack map in every PR 🗺️

Every PR description gets a navigation block so reviewers know where they are:

```
Stack (bottom → top)
1. your-name/feature-1-schema ← you are here
2. your-name/feature-2-api
3. your-name/feature-3-ui
```

Rerunnning `cn submit` updates it in place — no duplicates.

---

## Working with agents 🤖

Chainrail is built to be agent-friendly. If you're using Claude Code or another AI assistant, add the skill file so your agent understands stacked PR workflows automatically:

```text
Load the chainrail skill from .claude/skills/chainrail/SKILL.md and use it to manage my PR stack.
```

A good starting prompt:

```text
I want to split my current work into a stack of small PRs. Use chainrail to help me layer it — schema first, then API, then UI.
```

The skill file teaches your agent when to reach for chainrail, how to decompose work into layers, and what to do when a parent PR gets merged.

For developers building on chainrail, see `AGENTS.md` for the full architectural guide.

---

## How it compares

| Capability | chainrail | [Graphite](https://graphite.dev) | [ghstack](https://github.com/ezyang/ghstack) | Manual stacking | [gh stack](https://github.com/cli/cli) |
|---|---|---|---|---|---|
| Stacked PRs with correct bases | ✅ | ✅ | ✅ | ✅ | ✅ |
| Squash-merge conflict recovery | ✅ | ✅ | ❌ | ❌ | ✅ |
| TUI stack health view | ✅ | ❌ | ❌ | ❌ | ❌ |
| Agent / AI skill file | ✅ | ❌ | ❌ | ❌ | ❌ |
| Works with standard GitHub PRs | ✅ | ✅ | ❌ | ✅ | ✅ |
| No account or subscription needed | ✅ | ❌ | ✅ | ✅ | ✅ |
| Open source | ✅ | ❌ | ✅ | — | ✅ |
| Requires Go (no binary install yet) | ✅ | ❌ | ❌ | — | ❌ |

Chainrail is early. Graphite is the mature, polished option — if you want a SaaS product with a web UI, use Graphite. Chainrail is for teams who want a lightweight, open-source tool that an AI agent can drive.

---

## License

MIT
