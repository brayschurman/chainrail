---
task: 006
status: done
depends-on: []
---

# 006 — TTY detection helper

## Goal

A tiny helper that reports whether an `io.Writer` is connected to a terminal, so the renderer can switch to plain ASCII when piped.

## Files

- **New:** `internal/term/tty.go`
- **New:** `internal/term/tty_test.go`

## Implementation notes

Use stdlib only. The standard trick:

```go
package term

import (
    "io"
    "os"
)

func IsTTY(w io.Writer) bool {
    f, ok := w.(*os.File)
    if !ok {
        return false
    }
    info, err := f.Stat()
    if err != nil {
        return false
    }
    return (info.Mode() & os.ModeCharDevice) != 0
}
```

## Acceptance

- [ ] `IsTTY(os.Stdout)` returns true when the test is run from a real terminal — but skip that branch in CI/test mode (don't depend on TTY in tests).
- [ ] `IsTTY(&bytes.Buffer{})` returns false.
- [ ] `IsTTY(nil)` returns false (no panic).
- [ ] `go test ./internal/term/...` passes.

## Out of scope

- Any color or styling logic. This task only detects.
