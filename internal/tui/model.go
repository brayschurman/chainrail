package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Layer is the data for one stack layer, also used as JSON output.
type Layer struct {
	Position  int    `json:"position"`
	Branch    string `json:"branch"`
	PRNumber  int    `json:"pr_number"`
	PRState   string `json:"pr_state"` // "OPEN", "MERGED", or ""
	IsCurrent bool   `json:"is_current"`
	NeedsSync bool   `json:"needs_sync"`
}

// Model is the bubbletea model for cn status.
type Model struct {
	StackName string
	Trunk     string
	Layers    []Layer
	quitting  bool
}

var (
	styleFaint   = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	styleBold    = lipgloss.NewStyle().Bold(true)
	styleCurrent = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
	styleMerged  = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	styleOpen    = lipgloss.NewStyle().Foreground(lipgloss.Color("75"))
	styleWarn    = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	styleBox     = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("238")).
			Padding(1, 3)
)

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "q", "ctrl+c", "esc":
			m.quitting = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m Model) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder

	header := styleBold.Render("chainrail") + styleFaint.Render(" · ") + styleBold.Render(m.StackName)
	b.WriteString(header + "\n")
	b.WriteString(styleFaint.Render(strings.Repeat("─", 54)) + "\n\n")

	for _, l := range m.Layers {
		// cursor
		if l.IsCurrent {
			b.WriteString(styleCurrent.Render("▶ "))
		} else {
			b.WriteString("  ")
		}

		// position
		b.WriteString(styleFaint.Render(fmt.Sprintf("%d  ", l.Position)))

		// branch name
		if l.IsCurrent {
			b.WriteString(styleCurrent.Render(l.Branch))
		} else {
			b.WriteString(l.Branch)
		}

		// right-align PR info at col 52
		pad := 44 - len(l.Branch)
		if pad < 2 {
			pad = 2
		}
		b.WriteString(strings.Repeat(" ", pad))

		if l.PRNumber > 0 {
			b.WriteString(styleFaint.Render(fmt.Sprintf("#%-4d ", l.PRNumber)))
			switch l.PRState {
			case "MERGED":
				b.WriteString(styleMerged.Render("✓ merged"))
			case "OPEN":
				b.WriteString(styleOpen.Render("● open  "))
			default:
				b.WriteString(styleFaint.Render(l.PRState))
			}
		} else {
			b.WriteString(styleFaint.Render("         no PR"))
		}

		if l.NeedsSync {
			b.WriteString("  " + styleWarn.Render("⚠ needs sync"))
		}

		b.WriteString("\n")
	}

	b.WriteString("\n" + styleFaint.Render("  q quit"))

	return styleBox.Render(b.String())
}
