---
task: 005
status: in-progress
depends-on: [001]
---

# 005 — Renderer abstraction (output seam)

## Goal

Introduce the `Renderer` interface so commands return typed results that are formatted by a renderer rather than printed directly. v0.1 ships only `TextRenderer`; the seam exists so v0.1.1 can add `JSONRenderer` in one file.

## Files

- **New:** `internal/output/renderer.go` — `Renderer` interface and `TextRenderer` impl
- **New:** `internal/output/renderer_test.go`

## Implementation notes

```go
package output

import "io"

type Renderer interface {
    Success(out io.Writer, message string)
    Detail(out io.Writer, label, value string)
    List(out io.Writer, items []ListItem)
    Step(out io.Writer, status StepStatus, message string)
    Error(out io.Writer, errOut io.Writer, err error)
}

type StepStatus int

const (
    StepOK StepStatus = iota
    StepFail
    StepPending
)

type ListItem struct {
    Marker string // "→" or "✓"
    Text   string
}
```

`TextRenderer`:
- `Success`: write `✓ <message>\n` to `out`.
- `Detail`: write `  <label>: <value>\n`.
- `List`: each item indented.
- `Step`: prefix with `✓`, `✗`, or `…` based on `StepStatus`.
- `Error`: if `err` is `*errors.ChainrailError` (forward-reference to task 007 — for now accept `error` and write `err.Error()`), write to `errOut`. Leave a `// TODO(007): format ChainrailError fields` comment if needed.

**Glyphs:** use ASCII fallbacks (`OK`, `FAIL`, `--`) when `IsTTY(out)` returns false. Task 006 ships `IsTTY`. For this task, write `TextRenderer` to take an `isTTY bool` field set at construction time; the consumer decides. If task 006 isn't done yet, default to glyphs and add a TODO.

## Acceptance

- [ ] `Renderer` interface defined.
- [ ] `TextRenderer` implements all methods.
- [ ] Test: `Success` writes the expected string to a `bytes.Buffer`.
- [ ] Test: `Error` writes to `errOut`, not `out`.
- [ ] Test: with `isTTY=false`, no UTF-8 glyphs appear in the output.
- [ ] `go test ./internal/output/...` passes.

## Out of scope

- `JSONRenderer` — that's a v0.1.1 follow-up.
- Wiring renderers into specific commands — tasks 009–014 do that.
