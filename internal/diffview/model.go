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

var (
	styleFaint   = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	styleBold    = lipgloss.NewStyle().Bold(true)
	styleHeader  = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
	styleAdd     = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	styleDel     = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	styleHunk    = lipgloss.NewStyle().Foreground(lipgloss.Color("75"))
	styleFile    = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	styleSel     = lipgloss.NewStyle().Background(lipgloss.Color("236")).Bold(true).Foreground(lipgloss.Color("255"))
	styleKey     = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	styleSidebar = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderRight(true).
			BorderForeground(lipgloss.Color("238")).
			PaddingRight(1)
)

const (
	defaultWidth  = 120
	defaultHeight = 30
	sidebarWidth  = 32
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
			m.scrollY += m.diffPaneHeight()
		case "pgup":
			m.scrollY -= m.diffPaneHeight()
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
		return "\n  " + styleFaint.Render("no changes in this PR") + "\n"
	}
	header := styleBold.Render("chainrail") + "  " + styleHeader.Render(m.Title) + "\n"
	side := m.renderSidebar()
	diff := m.renderDiff()
	body := lipgloss.JoinHorizontal(lipgloss.Top, styleSidebar.Render(side), diff)
	keys := m.renderKeybindings()
	return header + body + "\n" + keys + "\n"
}

func (m Model) renderSidebar() string {
	var b strings.Builder
	b.WriteString(styleFaint.Render("FILES") + "\n\n")
	for i, f := range m.Files {
		name := truncatePath(f.Path, sidebarWidth-12)
		counts := fmt.Sprintf("+%d-%d", f.Adds, f.Dels)
		line := fmt.Sprintf("%-*s %s", sidebarWidth-12, name, styleFaint.Render(counts))
		if i == m.cursor {
			b.WriteString(styleSel.Render(" ▸ "+line) + "\n")
		} else {
			b.WriteString("   " + line + "\n")
		}
	}
	b.WriteString("\n" + styleFaint.Render(fmt.Sprintf("%d file%s", len(m.Files), pluralS(len(m.Files)))))
	return b.String()
}

func (m Model) renderDiff() string {
	if m.cursor >= len(m.Files) {
		return ""
	}
	f := m.Files[m.cursor]
	maxLines := m.diffPaneHeight()
	end := m.scrollY + maxLines
	if end > len(f.Lines) {
		end = len(f.Lines)
	}
	if m.scrollY > len(f.Lines) {
		m.scrollY = len(f.Lines)
	}
	var b strings.Builder
	for i := m.scrollY; i < end; i++ {
		l := f.Lines[i]
		switch l.Kind {
		case LineAdd:
			b.WriteString(styleAdd.Render("+") + m.highlightRest(f.Path, l.Text))
		case LineDel:
			b.WriteString(styleDel.Render("-") + m.highlightRest(f.Path, l.Text))
		case LineContext:
			b.WriteString(" " + m.highlightRest(f.Path, l.Text))
		case LineHunk:
			b.WriteString(styleHunk.Render(l.Text))
		case LineFile:
			b.WriteString(styleFile.Render(l.Text))
		case LineNoNewLine:
			b.WriteString(styleFaint.Render(l.Text))
		default:
			b.WriteString(l.Text)
		}
		b.WriteString("\n")
	}
	// Pad to maxLines so the layout stays consistent.
	for i := end - m.scrollY; i < maxLines; i++ {
		b.WriteString("\n")
	}
	return b.String()
}

func (m Model) renderKeybindings() string {
	return styleFaint.Render("  ") +
		styleKey.Render("↑↓") + styleFaint.Render(" scroll  ") +
		styleKey.Render("tab") + styleFaint.Render(" next file  ") +
		styleKey.Render("shift+tab") + styleFaint.Render(" prev  ") +
		styleKey.Render("g/G") + styleFaint.Render(" top/bottom  ") +
		styleKey.Render("q") + styleFaint.Render(" quit")
}

func (m Model) diffPaneHeight() int {
	// Reserve 3 lines for the chainrail/title header + spacing + keybinding line.
	h := m.height - 4
	if h < 5 {
		return 5
	}
	return h
}

func (m Model) maxScroll() int {
	if m.cursor >= len(m.Files) {
		return 0
	}
	n := len(m.Files[m.cursor].Lines) - m.diffPaneHeight()
	if n < 0 {
		return 0
	}
	return n
}

// highlightRest highlights everything after the leading +/-/space marker of
// a content line. Returns the unmarked body either chroma-highlighted or
// plain if the highlighter is disabled / lexer is unknown.
func (m Model) highlightRest(path, text string) string {
	if len(text) == 0 {
		return ""
	}
	body := text
	if len(text) >= 1 && (text[0] == '+' || text[0] == '-' || text[0] == ' ') {
		body = text[1:]
	}
	if m.highlighter == nil {
		return body
	}
	return m.highlighter.Highlight(path, body)
}

func truncatePath(p string, max int) string {
	if max < 4 || len(p) <= max {
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
