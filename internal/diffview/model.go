package diffview

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Model is the bubbletea model for the PR diff viewer.
type Model struct {
	Title       string // e.g. "#687 wall drawing UX"
	Files       []File
	width       int
	height      int
	highlighter *Highlighter

	cursor   int // file index in sidebar
	scrollY  int // line index at top of diff pane
	quitting bool

	// Render cache: scrolling and tabbing are hot paths, so we pre-render the
	// expensive (chroma + lipgloss) work per file/width and reuse it on every
	// frame. Invalidated when cursor or width changes.
	rc renderCache
}

// renderCache stores fully pre-formatted strings — ready to write straight to
// the screen — for the current file at the current geometry. A frame at
// steady state is then just slicing into rc.lines.
type renderCache struct {
	fileIdx  int
	width    int
	sideW    int
	diffW    int
	bodyH    int
	lines    []string // pre-rendered content lines for the current file
	emptyRow string   // pre-rendered empty diff-pane row at current width
	emptySide string  // pre-rendered empty sidebar row at current width
	fileHdr  string   // pre-rendered "FILES" sidebar header
	sideRows []string // pre-rendered sidebar rows (one per file)
	// File-header bar text for the current file.
	pathHdr string
}

// Color palette — dark theme, hunk-inspired.
var (
	// Foregrounds
	colorPink    = lipgloss.Color("205") // accents / title
	colorOrange  = lipgloss.Color("214") // file header / warn
	colorCyan    = lipgloss.Color("75")  // hunk header
	colorGreenFg = lipgloss.Color("10")  // + marker
	colorRedFg   = lipgloss.Color("9")   // - marker
	colorFaint   = lipgloss.Color("245") // faint context
	colorBright  = lipgloss.Color("255") // selected sidebar row

	// Backgrounds — hex truecolor so we can pick muted forest-green and
	// wine-red tints that read as "diff state" rather than "warning sign".
	// Tuned against Claude Code's diff palette; the goal is calm contrast,
	// not stoplight saturation.
	bgChrome   = lipgloss.Color("#2a2a2a") // top header / keybinding bar
	bgPane     = lipgloss.Color("#1c1c1c") // diff pane / sidebar background
	bgFileHdr  = lipgloss.Color("#2e2e2e") // per-file path header
	bgSel      = lipgloss.Color("#3a3a3a") // sidebar selected row
	bgHunk     = lipgloss.Color("#262638") // hunk header background (cool blue-grey)
	bgAdd      = lipgloss.Color("#143020") // muted forest green for + lines
	bgDel      = lipgloss.Color("#3a1a1a") // muted wine red for - lines
	bgGutter   = lipgloss.Color("#161616") // line-number gutter background
	bgAddGut   = lipgloss.Color("#0e2418") // gutter on + lines (slightly darker than bgAdd)
	bgDelGut   = lipgloss.Color("#2e1414") // gutter on - lines (slightly darker than bgDel)
)

// Reusable styles. Width is applied at render time so panes can be sized
// against the current terminal.
var (
	styleTitle    = lipgloss.NewStyle().Bold(true).Foreground(colorBright)
	styleHeading  = lipgloss.NewStyle().Bold(true).Foreground(colorPink)
	styleChrome   = lipgloss.NewStyle().Background(bgChrome).Foreground(colorBright)
	stylePaneBg   = lipgloss.NewStyle().Background(bgPane).Foreground(colorBright)
	styleFaintBg  = lipgloss.NewStyle().Background(bgPane).Foreground(colorFaint)
	styleFileHdr  = lipgloss.NewStyle().Background(bgFileHdr).Foreground(colorOrange).Bold(true)
	styleHunkLine = lipgloss.NewStyle().Background(bgHunk).Foreground(colorCyan)
	styleAddLine  = lipgloss.NewStyle().Background(bgAdd).Foreground(colorBright)
	styleDelLine  = lipgloss.NewStyle().Background(bgDel).Foreground(colorBright)
	styleAddMark  = lipgloss.NewStyle().Background(bgAdd).Foreground(colorGreenFg).Bold(true)
	styleDelMark  = lipgloss.NewStyle().Background(bgDel).Foreground(colorRedFg).Bold(true)
	styleSidebarSel = lipgloss.NewStyle().Background(bgSel).Foreground(colorBright).Bold(true)
	styleKey      = lipgloss.NewStyle().Background(bgChrome).Foreground(colorPink).Bold(true)
	styleKeyDim   = lipgloss.NewStyle().Background(bgChrome).Foreground(colorFaint)
	styleGutter   = lipgloss.NewStyle().Background(bgGutter).Foreground(colorFaint)
	styleAddGut   = lipgloss.NewStyle().Background(bgAddGut).Foreground(colorFaint)
	styleDelGut   = lipgloss.NewStyle().Background(bgDelGut).Foreground(colorFaint)
)

const (
	defaultWidth  = 160
	defaultHeight = 48
)

func New(title string, files []File) Model {
	return Model{
		Title:       title,
		Files:       files,
		width:       defaultWidth,
		height:      defaultHeight,
		highlighter: NewHighlighter(),
	}
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "tab", "n":
			if len(m.Files) > 0 {
				m.cursor = (m.cursor + 1) % len(m.Files)
				m.scrollY = 0
			}
		case "shift+tab", "p":
			if len(m.Files) > 0 {
				m.cursor = (m.cursor - 1 + len(m.Files)) % len(m.Files)
				m.scrollY = 0
			}
		case "down", "j":
			m.scrollY++
		case "up", "k":
			if m.scrollY > 0 {
				m.scrollY--
			}
		case "pgdown", " ":
			m.scrollY += m.diffContentHeight()
		case "pgup":
			m.scrollY -= m.diffContentHeight()
			if m.scrollY < 0 {
				m.scrollY = 0
			}
		case "g", "home":
			m.scrollY = 0
		case "G", "end":
			m.scrollY = m.maxScroll()
		}
	}
	if m.scrollY > m.maxScroll() {
		m.scrollY = m.maxScroll()
	}
	return m, nil
}

func (m *Model) ensureCache() {
	want := renderCache{
		fileIdx: m.cursor,
		width:   m.width,
		bodyH:   m.bodyHeight(),
		sideW:   m.sidebarWidth(),
	}
	want.diffW = m.width - want.sideW
	if m.rc.fileIdx == want.fileIdx &&
		m.rc.width == want.width &&
		m.rc.sideW == want.sideW &&
		m.rc.diffW == want.diffW &&
		m.rc.bodyH == want.bodyH &&
		len(m.rc.lines) > 0 {
		return
	}

	want.emptyRow = stylePaneBg.Width(want.diffW).Render("")
	want.emptySide = styleFaintBg.Width(want.sideW).Render("")
	want.fileHdr = styleFileHdr.Width(want.sideW).Render(" FILES")

	// Pre-render the diff content lines for the current file.
	if m.cursor < len(m.Files) {
		f := m.Files[m.cursor]
		want.pathHdr = renderFileHeader(f, want.diffW)
		want.lines = m.renderFileLines(f, want.diffW)
	}

	// Pre-render sidebar rows.
	want.sideRows = m.buildSidebarRows(want.sideW)

	m.rc = want
}

func (m Model) View() string {
	if m.quitting {
		return ""
	}
	if len(m.Files) == 0 {
		return styleChrome.Width(m.width).Render(" no changes in this PR ")
	}

	// View is a value receiver in bubbletea; we touch the cache via a
	// pointer-local copy to avoid copying the renderCache slice each frame.
	mp := &m
	mp.ensureCache()

	header := mp.renderTopBar()
	sidebar := mp.renderSidebarCached()
	diff := mp.renderDiffCached()
	body := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, diff)
	keys := mp.renderKeybindings(m.width)

	return header + "\n" + body + "\n" + keys
}

// ---------------------------------------------------------------------------
// Top bar
// ---------------------------------------------------------------------------

func (m Model) renderTopBar() string {
	left := " " + styleTitle.Render("chainrail") + "  " + styleHeading.Render(m.Title)
	// styleChrome.Width pads with bg color to full width.
	return styleChrome.Width(m.width).Render(left)
}

// ---------------------------------------------------------------------------
// Sidebar
// ---------------------------------------------------------------------------

// buildSidebarRows pre-renders one styled string per file plus a header. The
// rows are reused frame-to-frame and only rebuilt when width or cursor moves.
func (m Model) buildSidebarRows(w int) []string {
	if w <= 0 {
		return nil
	}
	rows := make([]string, 0, len(m.Files))
	const countsW = 11
	nameW := w - 3 - 1 - countsW
	if nameW < 6 {
		nameW = 6
	}
	for i, f := range m.Files {
		name := truncatePath(f.Path, nameW)
		counts := fmt.Sprintf("+%-3d -%-3d", f.Adds, f.Dels)
		row := fmt.Sprintf(" %s %-*s %s",
			selectionMarker(i == m.cursor),
			nameW, name,
			counts,
		)
		if i == m.cursor {
			rows = append(rows, styleSidebarSel.Width(w).Render(row))
		} else {
			rows = append(rows, styleFaintBg.Width(w).Render(row))
		}
	}
	return rows
}

func (m *Model) renderSidebarCached() string {
	w := m.rc.sideW
	h := m.rc.bodyH
	if w <= 0 || h <= 0 {
		return ""
	}
	out := make([]string, 0, h)
	out = append(out, m.rc.fileHdr)
	for _, r := range m.rc.sideRows {
		if len(out) >= h-1 {
			break
		}
		out = append(out, r)
	}
	for len(out) < h-1 {
		out = append(out, m.rc.emptySide)
	}
	footer := fmt.Sprintf(" %d file%s", len(m.Files), pluralS(len(m.Files)))
	out = append(out, styleFileHdr.Width(w).Render(footer))
	if len(out) > h {
		out = out[:h]
	}
	return strings.Join(out, "\n")
}

func selectionMarker(selected bool) string {
	if selected {
		return "▸"
	}
	return " "
}

// ---------------------------------------------------------------------------
// Diff pane
// ---------------------------------------------------------------------------

// renderDiffCached slices into the pre-rendered file lines and assembles the
// diff pane. Hot path — no chroma or lipgloss work here.
func (m *Model) renderDiffCached() string {
	w := m.rc.diffW
	h := m.rc.bodyH
	if w <= 0 || h <= 0 || m.cursor >= len(m.Files) {
		return ""
	}
	f := m.Files[m.cursor]
	contentH := h - 2
	if contentH < 1 {
		contentH = 1
	}
	end := m.scrollY + contentH
	if end > len(m.rc.lines) {
		end = len(m.rc.lines)
	}

	lines := make([]string, 0, h)
	lines = append(lines, m.rc.pathHdr)
	for i := m.scrollY; i < end; i++ {
		lines = append(lines, m.rc.lines[i])
	}
	for len(lines) < h-1 {
		lines = append(lines, m.rc.emptyRow)
	}
	lines = append(lines, m.renderDiffFooter(f, w))
	if len(lines) > h {
		lines = lines[:h]
	}
	return strings.Join(lines, "\n")
}

func renderFileHeader(f File, w int) string {
	left := " " + f.Path
	right := fmt.Sprintf("+%d -%d ", f.Adds, f.Dels)
	gap := w - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
		// Truncate path if needed.
		need := lipgloss.Width(left) + gap + lipgloss.Width(right) - w
		if need > 0 && len(left) > need+1 {
			left = left[:len(left)-need-1] + "…"
		}
	}
	return styleFileHdr.Width(w).Render(left + strings.Repeat(" ", gap) + right)
}

func (m Model) renderDiffFooter(f File, w int) string {
	total := len(f.Lines)
	visEnd := m.scrollY + m.diffContentHeight()
	if visEnd > total {
		visEnd = total
	}
	text := fmt.Sprintf(" %s · %d–%d of %d ", fileCountStr(m.cursor+1, len(m.Files)), m.scrollY+1, visEnd, total)
	return styleFileHdr.Width(w).Render(text)
}

func fileCountStr(cur, total int) string {
	return fmt.Sprintf("file %d/%d", cur, total)
}

// renderFileLines walks a file's diff once, tracking old/new line numbers as
// it goes, and produces one fully-styled string per displayable line. Hunk
// headers reset the counters; file headers are dropped.
func (m Model) renderFileLines(f File, w int) []string {
	out := make([]string, 0, len(f.Lines))
	var oldNo, newNo int
	for _, l := range f.Lines {
		switch l.Kind {
		case LineFile:
			// Drop — we already have the file-path header bar above.
			continue
		case LineHunk:
			oldNo, newNo = parseHunkStart(l.Text)
			out = append(out, styleHunkLine.Width(w).Render(" "+l.Text))
		case LineAdd:
			out = append(out, m.styledRow(l, f.Path, 0, newNo, w))
			newNo++
		case LineDel:
			out = append(out, m.styledRow(l, f.Path, oldNo, 0, w))
			oldNo++
		case LineContext:
			out = append(out, m.styledRow(l, f.Path, oldNo, newNo, w))
			oldNo++
			newNo++
		case LineNoNewLine:
			out = append(out, styleFaintBg.Width(w).Render("        "+l.Text))
		default:
			out = append(out, stylePaneBg.Width(w).Render(l.Text))
		}
	}
	return out
}

// styledRow renders one content row of the diff: line-number gutter on the
// left, then the line itself with a subtle background tint by kind. Old / new
// number columns are 4 chars each; pass 0 to render that column as blanks.
func (m Model) styledRow(l Line, path string, oldNo, newNo, w int) string {
	body := m.highlightForBg(path, stripMarker(l.Text))

	gutterText := fmt.Sprintf("%s %s ", numCol(oldNo), numCol(newNo))

	var gutterStyle, lineStyle, markerStyle lipgloss.Style
	var marker string

	switch l.Kind {
	case LineAdd:
		gutterStyle = styleAddGut
		lineStyle = styleAddLine
		markerStyle = styleAddMark
		marker = "+"
	case LineDel:
		gutterStyle = styleDelGut
		lineStyle = styleDelLine
		markerStyle = styleDelMark
		marker = "-"
	default:
		gutterStyle = styleGutter
		lineStyle = stylePaneBg
		markerStyle = styleGutter
		marker = " "
	}

	gutter := gutterStyle.Render(gutterText)
	mark := markerStyle.Render(marker + " ")
	contentW := w - lipgloss.Width(gutter) - lipgloss.Width(mark)
	if contentW < 1 {
		contentW = 1
	}
	content := lineStyle.Width(contentW).Render(body)
	return gutter + mark + content
}

// numCol formats a line number into a 4-char right-aligned column, or four
// spaces when the number is 0 (i.e. the row doesn't exist on that side).
func numCol(n int) string {
	if n == 0 {
		return "    "
	}
	return fmt.Sprintf("%4d", n)
}

// parseHunkStart pulls the starting old-line and new-line numbers out of a
// hunk header like "@@ -56,4 +56,4 @@ jobs:". Returns 0,0 on parse failure.
func parseHunkStart(s string) (int, int) {
	// Look for "-N" then "+N" within the @@ ... @@ section.
	var oldStart, newStart int
	at := strings.Index(s, "@@")
	if at < 0 {
		return 0, 0
	}
	rest := s[at+2:]
	end := strings.Index(rest, "@@")
	if end < 0 {
		end = len(rest)
	}
	rest = rest[:end]
	for _, tok := range strings.Fields(rest) {
		if len(tok) < 2 {
			continue
		}
		sign := tok[0]
		num := tok[1:]
		if i := strings.IndexByte(num, ','); i >= 0 {
			num = num[:i]
		}
		var v int
		fmt.Sscanf(num, "%d", &v)
		switch sign {
		case '-':
			oldStart = v
		case '+':
			newStart = v
		}
	}
	return oldStart, newStart
}

// highlightForBg runs chroma on the body, then strips ANSI resets so the
// outer lipgloss background color isn't broken mid-line.
func (m Model) highlightForBg(path, src string) string {
	if m.highlighter == nil {
		return src
	}
	out := m.highlighter.Highlight(path, src)
	// chroma's terminal256 formatter can emit \x1b[0m which resets bg too.
	// Replace with a "default fg" reset (39m) so our bg survives.
	out = strings.ReplaceAll(out, "\x1b[0m", "\x1b[39m")
	return out
}

func stripMarker(text string) string {
	if len(text) == 0 {
		return ""
	}
	switch text[0] {
	case '+', '-', ' ':
		return text[1:]
	}
	return text
}

// ---------------------------------------------------------------------------
// Keybindings
// ---------------------------------------------------------------------------

func (m Model) renderKeybindings(w int) string {
	parts := []string{
		styleKey.Render("↑↓") + styleKeyDim.Render(" scroll"),
		styleKey.Render("tab") + styleKeyDim.Render(" next file"),
		styleKey.Render("shift+tab") + styleKeyDim.Render(" prev"),
		styleKey.Render("g/G") + styleKeyDim.Render(" top/bottom"),
		styleKey.Render("q") + styleKeyDim.Render(" quit"),
	}
	sep := styleKeyDim.Render("  ·  ")
	content := " " + strings.Join(parts, sep)
	return styleChrome.Width(w).Render(content)
}

// ---------------------------------------------------------------------------
// Geometry helpers
// ---------------------------------------------------------------------------

func (m Model) sidebarWidth() int {
	w := m.width / 4
	if w < 28 {
		w = 28
	}
	if w > 44 {
		w = 44
	}
	if w > m.width-20 {
		w = m.width - 20
		if w < 0 {
			w = 0
		}
	}
	return w
}

// bodyHeight is the height of the sidebar and diff panes (i.e. between the
// top header bar and the keybinding bar).
func (m Model) bodyHeight() int {
	h := m.height - 2
	if h < 5 {
		h = 5
	}
	return h
}

// diffContentHeight is the number of lines available for actual diff content
// (excludes the file-path header and the bottom diff footer).
func (m Model) diffContentHeight() int {
	h := m.bodyHeight() - 2
	if h < 1 {
		h = 1
	}
	return h
}

// diffPaneHeight is kept for backward compat (renamed conceptually). Used
// only by maxScroll.
func (m Model) diffPaneHeight() int {
	return m.diffContentHeight()
}

func (m Model) maxScroll() int {
	if m.cursor >= len(m.Files) {
		return 0
	}
	n := len(m.Files[m.cursor].Lines) - m.diffContentHeight()
	if n < 0 {
		return 0
	}
	return n
}

func truncatePath(p string, max int) string {
	if max < 4 || lipgloss.Width(p) <= max {
		return p
	}
	return "…" + p[len(p)-(max-1):]
}

func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
