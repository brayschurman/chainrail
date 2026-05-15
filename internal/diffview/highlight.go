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
