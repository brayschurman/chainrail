package output

import (
	"errors"
	"fmt"
	"io"

	crerrors "github.com/brayschurman/chainrail/internal/errors"
)

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
	Marker string
	Text   string
}

type TextRenderer struct {
	isTTY bool
}

func NewTextRenderer(isTTY bool) *TextRenderer {
	return &TextRenderer{isTTY: isTTY}
}

func (r *TextRenderer) Success(out io.Writer, message string) {
	prefix := "OK"
	if r.isTTY {
		prefix = "✓"
	}
	fmt.Fprintf(out, "%s %s\n", prefix, message)
}

func (r *TextRenderer) Detail(out io.Writer, label, value string) {
	fmt.Fprintf(out, "  %s: %s\n", label, value)
}

func (r *TextRenderer) List(out io.Writer, items []ListItem) {
	for _, item := range items {
		marker := item.Marker
		if marker == "" {
			marker = "-"
		}
		fmt.Fprintf(out, "  %s %s\n", marker, item.Text)
	}
}

func (r *TextRenderer) Step(out io.Writer, status StepStatus, message string) {
	prefix := r.stepPrefix(status)
	fmt.Fprintf(out, "%s %s\n", prefix, message)
}

func (r *TextRenderer) stepPrefix(status StepStatus) string {
	if !r.isTTY {
		switch status {
		case StepOK:
			return "OK"
		case StepFail:
			return "FAIL"
		case StepPending:
			return "--"
		}
		return "--"
	}
	switch status {
	case StepOK:
		return "✓"
	case StepFail:
		return "✗"
	case StepPending:
		return "…"
	}
	return "·"
}

func (r *TextRenderer) Error(_ io.Writer, errOut io.Writer, err error) {
	if err == nil {
		return
	}
	var ce *crerrors.ChainrailError
	if errors.As(err, &ce) {
		fmt.Fprintf(errOut, "Error [%s]: %s\n", ce.Code, ce.Message)
		if ce.Suggestion != "" {
			fmt.Fprintf(errOut, "  Suggestion: %s\n", ce.Suggestion)
		}
		return
	}
	fmt.Fprintf(errOut, "Error: %s\n", err.Error())
}
