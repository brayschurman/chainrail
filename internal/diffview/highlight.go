package diffview

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/charmbracelet/lipgloss"
)

// Highlighter colorizes diff content lines using chroma based on file
// extension. Lexers and the formatter are looked up once per extension and
// reused. When NO_COLOR is set or the lexer is unknown, Highlight returns the
// input unchanged.
type Highlighter struct {
	mu      sync.Mutex
	byExt   map[string]chroma.Lexer
	style   *chroma.Style
	fmt     chroma.Formatter
	enabled bool
}

func NewHighlighter() *Highlighter {
	enabled := os.Getenv("NO_COLOR") == ""
	style := styles.Get("github-dark")
	if style == nil {
		style = styles.Fallback
	}
	return &Highlighter{
		byExt:   map[string]chroma.Lexer{},
		style:   style,
		fmt:     formatters.Get("terminal256"),
		enabled: enabled,
	}
}

// Highlight returns the colorized form of src for the lexer matching path's
// extension. Falls back to src on any error or when disabled.
func (h *Highlighter) Highlight(path, src string) string {
	if h == nil || !h.enabled || src == "" {
		return src
	}
	lex := h.lexerFor(path)
	if lex == nil {
		return src
	}
	it, err := lex.Tokenise(nil, src)
	if err != nil {
		return src
	}
	var buf bytes.Buffer
	if err := h.fmt.Format(&buf, h.style, it); err != nil {
		return src
	}
	// chroma's terminal256 formatter sometimes appends a trailing newline;
	// strip it so we don't corrupt the row.
	return strings.TrimRight(buf.String(), "\n")
}

// HighlightWithBg tokenises src with the lexer for path and renders each
// token with both its syntax foreground AND the given diff background. This
// gives the hunk/diffs.com look where chroma's colors stay vivid on top of a
// soft +/- tint, without relying on fragile mid-stream ANSI rewriting.
//
// Returns src wrapped in a single bg style if no lexer matches or
// highlighting is disabled. The caller is responsible for any extra padding
// needed to reach a target pane width.
func (h *Highlighter) HighlightWithBg(path, src string, bg lipgloss.Color) string {
	if h == nil || !h.enabled || src == "" {
		return lipgloss.NewStyle().Background(bg).Render(src)
	}
	lex := h.lexerFor(path)
	if lex == nil {
		return lipgloss.NewStyle().Background(bg).Render(src)
	}
	it, err := lex.Tokenise(nil, src)
	if err != nil {
		return lipgloss.NewStyle().Background(bg).Render(src)
	}

	var b strings.Builder
	for tok := it(); tok != chroma.EOF; tok = it() {
		entry := h.style.Get(tok.Type)
		st := lipgloss.NewStyle().Background(bg)
		if entry.Colour.IsSet() {
			st = st.Foreground(lipgloss.Color(entry.Colour.String()))
		}
		if entry.Bold == chroma.Yes {
			st = st.Bold(true)
		}
		if entry.Italic == chroma.Yes {
			st = st.Italic(true)
		}
		b.WriteString(st.Render(tok.Value))
	}
	return b.String()
}

func (h *Highlighter) lexerFor(path string) chroma.Lexer {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == "" {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if l, ok := h.byExt[ext]; ok {
		return l
	}
	l := lexers.Match(path)
	if l != nil {
		l = chroma.Coalesce(l)
	}
	h.byExt[ext] = l
	return l
}
