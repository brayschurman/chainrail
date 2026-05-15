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

	// Backgrounds: kept subtle so chroma's foreground tokens stay readable.
	bgChrome  = lipgloss.Color("236") // top header / keybinding bar
	bgPane    = lipgloss.Color("234") // diff pane / sidebar background
	bgFileHdr = lipgloss.Color("237") // per-file path header
	bgSel     = lipgloss.Color("238") // sidebar selected row
	bgHunk    = lipgloss.Color("235") // hunk header background
	bgAdd     = lipgloss.Color("22")  // dark green for + lines
	bgDel     = lipgloss.Color("52")  // dark red for - lines
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

func (m Model) View() string {
	if m.quitting {
		return ""
	}
	if len(m.Files) == 0 {
		return styleChrome.Width(m.width).Render(" no changes in this PR ")
	}

	sideW := m.sidebarWidth()
	diffW := m.width - sideW
	bodyH := m.bodyHeight()

	header := m.renderTopBar()
	sidebar := m.renderSidebar(sideW, bodyH)
	diff := m.renderDiff(diffW, bodyH)
	body := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, diff)
	keys := m.renderKeybindings(m.width)

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

func (m Model) renderSidebar(w, h int) string {
	if w <= 0 || h <= 0 {
		return ""
	}
	var lines []string
	lines = append(lines, styleFileHdr.Width(w).Render(" FILES"))

	// Width budget: " ▸ " (3) + name + " " + "+ddd -ddd" (10) + right pad
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
			lines = append(lines, styleSidebarSel.Width(w).Render(row))
		} else {
			lines = append(lines, styleFaintBg.Width(w).Render(row))
		}
	}

	// Pad to bodyHeight - 1 (we'll cap with a footer row below).
	for len(lines) < h-1 {
		lines = append(lines, styleFaintBg.Width(w).Render(""))
	}
	// Footer.
	footer := fmt.Sprintf(" %d file%s", len(m.Files), pluralS(len(m.Files)))
	lines = append(lines, styleFileHdr.Width(w).Render(footer))

	// Crop if we somehow overshot.
	if len(lines) > h {
		lines = lines[:h]
	}
	return strings.Join(lines, "\n")
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

func (m Model) renderDiff(w, h int) string {
	if w <= 0 || h <= 0 || m.cursor >= len(m.Files) {
		return ""
	}
	f := m.Files[m.cursor]

	// One row for the file path header, one for the hunk-position footer.
	contentH := h - 2
	if contentH < 1 {
		contentH = 1
	}

	end := m.scrollY + contentH
	if end > len(f.Lines) {
		end = len(f.Lines)
	}

	var lines []string

	// Top-of-pane header — file path on the left, +/- counts on the right.
	lines = append(lines, m.renderFileHeader(f, w))

	// Diff content.
	for i := m.scrollY; i < end; i++ {
		lines = append(lines, m.renderDiffLine(f.Lines[i], f.Path, w))
	}
	// Pad with subtle background so we don't render into a void.
	for len(lines) < h-1 {
		lines = append(lines, stylePaneBg.Width(w).Render(""))
	}
	// Bottom footer — file counter + scroll position. Keeps the pane bounded.
	lines = append(lines, m.renderDiffFooter(f, w))

	if len(lines) > h {
		lines = lines[:h]
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderFileHeader(f File, w int) string {
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

// renderDiffLine renders one diff line padded to the full pane width, with
// full-width background coloring so the diff state extends to the right edge.
func (m Model) renderDiffLine(l Line, path string, w int) string {
	switch l.Kind {
	case LineAdd:
		body := stripMarker(l.Text)
		hl := m.highlightForBg(path, body)
		return styleAddLine.Width(w).Render(styleAddMark.Render("+") + " " + hl)
	case LineDel:
		body := stripMarker(l.Text)
		hl := m.highlightForBg(path, body)
		return styleDelLine.Width(w).Render(styleDelMark.Render("-") + " " + hl)
	case LineContext:
		body := stripMarker(l.Text)
		hl := m.highlightForBg(path, body)
		return stylePaneBg.Width(w).Render("  " + hl)
	case LineHunk:
		return styleHunkLine.Width(w).Render(" " + l.Text)
	case LineNoNewLine:
		return styleFaintBg.Width(w).Render(" " + l.Text)
	case LineFile:
		// Skip raw "--- a/foo" / "+++ b/foo" / "index abc..def" lines —
		// they're redundant with the per-file header.
		return ""
	default:
		return stylePaneBg.Width(w).Render(l.Text)
	}
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
