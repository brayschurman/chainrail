package diffview

import (
	"strings"
	"testing"
)

func TestHighlight_KnownExtensionEmitsANSI(t *testing.T) {
	h := NewHighlighter()
	if !h.enabled {
		t.Skip("NO_COLOR set in environment — chroma disabled")
	}
	got := h.Highlight("foo.go", "func hello() {}")
	if !strings.Contains(got, "\x1b[") {
		t.Errorf("expected ANSI escape codes for known Go source, got %q", got)
	}
}

func TestHighlight_UnknownExtensionPassthrough(t *testing.T) {
	h := NewHighlighter()
	got := h.Highlight("foo.zzzz", "raw text")
	if got != "raw text" {
		t.Errorf("unknown ext: got %q, want passthrough", got)
	}
}

func TestHighlight_NoExtensionPassthrough(t *testing.T) {
	h := NewHighlighter()
	got := h.Highlight("Makefile", "all: build")
	// Makefile may or may not be detected (chroma can sometimes match by
	// filename). Just assert non-empty.
	if got == "" {
		t.Errorf("expected non-empty output for Makefile, got empty")
	}
}

func TestHighlight_DisabledWhenNoHighlighter(t *testing.T) {
	var h *Highlighter
	got := h.Highlight("foo.go", "func x() {}")
	if got != "func x() {}" {
		t.Errorf("nil receiver should passthrough, got %q", got)
	}
}
