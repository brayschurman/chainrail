package output

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	crerrors "github.com/brayschurman/chainrail/internal/errors"
)

func TestTextRenderer_Success_TTY(t *testing.T) {
	r := NewTextRenderer(true)
	var out bytes.Buffer
	r.Success(&out, "all good")
	got := out.String()
	if !strings.Contains(got, "all good") {
		t.Fatalf("output missing message: %q", got)
	}
	if !strings.HasSuffix(got, "\n") {
		t.Fatalf("output should end with newline: %q", got)
	}
}

func TestTextRenderer_Success_NoTTY_NoGlyphs(t *testing.T) {
	r := NewTextRenderer(false)
	var out bytes.Buffer
	r.Success(&out, "done")
	got := out.String()
	for _, b := range []byte(got) {
		if b > 127 {
			t.Fatalf("non-TTY output contains non-ASCII byte 0x%x in %q", b, got)
		}
	}
}

func TestTextRenderer_Detail(t *testing.T) {
	r := NewTextRenderer(false)
	var out bytes.Buffer
	r.Detail(&out, "trunk", "main")
	got := out.String()
	if !strings.Contains(got, "trunk") || !strings.Contains(got, "main") {
		t.Fatalf("detail missing label/value: %q", got)
	}
}

func TestTextRenderer_List(t *testing.T) {
	r := NewTextRenderer(false)
	var out bytes.Buffer
	r.List(&out, []ListItem{
		{Marker: "*", Text: "first"},
		{Marker: "*", Text: "second"},
	})
	got := out.String()
	if !strings.Contains(got, "first") || !strings.Contains(got, "second") {
		t.Fatalf("list output incomplete: %q", got)
	}
	if strings.Count(got, "\n") < 2 {
		t.Fatalf("expected one line per item: %q", got)
	}
}

func TestTextRenderer_Step_AllStatuses(t *testing.T) {
	r := NewTextRenderer(false)
	for _, status := range []StepStatus{StepOK, StepFail, StepPending} {
		var out bytes.Buffer
		r.Step(&out, status, "working")
		got := out.String()
		if !strings.Contains(got, "working") {
			t.Fatalf("step status %d missing message: %q", status, got)
		}
	}
}

func TestTextRenderer_Error_WritesToErrOut(t *testing.T) {
	r := NewTextRenderer(false)
	var out, errOut bytes.Buffer
	r.Error(&out, &errOut, errors.New("something broke"))
	if out.Len() != 0 {
		t.Fatalf("stdout should be empty, got %q", out.String())
	}
	if !strings.Contains(errOut.String(), "something broke") {
		t.Fatalf("errOut missing message: %q", errOut.String())
	}
}

func TestRendererInterface_TextRendererImplements(t *testing.T) {
	var _ Renderer = (*TextRenderer)(nil)
}

func TestTextRenderer_Error_ChainrailError_WithSuggestion(t *testing.T) {
	r := NewTextRenderer(false)
	var out, errOut bytes.Buffer
	ce := &crerrors.ChainrailError{
		Code:       crerrors.CodeDirtyWorktree,
		Message:    "working tree dirty",
		Suggestion: "commit or stash",
	}
	r.Error(&out, &errOut, ce)
	got := errOut.String()
	if !strings.Contains(got, crerrors.CodeDirtyWorktree) {
		t.Fatalf("expected code in output, got %q", got)
	}
	if !strings.Contains(got, "working tree dirty") {
		t.Fatalf("expected message in output, got %q", got)
	}
	if !strings.Contains(got, "commit or stash") {
		t.Fatalf("expected suggestion in output, got %q", got)
	}
}

func TestTextRenderer_Error_ChainrailError_NoSuggestion(t *testing.T) {
	r := NewTextRenderer(false)
	var out, errOut bytes.Buffer
	ce := &crerrors.ChainrailError{
		Code:    crerrors.CodeNotGitRepo,
		Message: "not a git repo",
	}
	r.Error(&out, &errOut, ce)
	got := errOut.String()
	if !strings.Contains(got, "NOT_GIT_REPO") {
		t.Fatalf("expected code, got %q", got)
	}
	if strings.Contains(got, "Suggestion:") {
		t.Fatalf("should not include Suggestion line when empty: %q", got)
	}
}
