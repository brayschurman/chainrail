package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Layer is one branch in a stack, also used as JSON output.
type Layer struct {
	Stack     string `json:"stack"`
	Position  int    `json:"position"`
	Branch    string `json:"branch"`
	Title     string `json:"title"` // PR title (shown in --all mode)
	PRNumber  int    `json:"pr_number"`
	PRState   string `json:"pr_state"` // "OPEN", "MERGED", or ""
	IsCurrent bool   `json:"is_current"`
	NeedsSync bool   `json:"needs_sync"`
	Depth     int    `json:"depth"` // nesting depth for tree view (--all mode)
}

// PendingAction is an action the user triggered that the caller should run
// after the TUI exits.
type PendingAction int

const (
	ActionNone     PendingAction = iota
	ActionCheckout               // git checkout the selected branch
	ActionOpenPR                 // open the PR in the browser
	ActionSync                   // run cn sync from the selected branch
	ActionSubmit                 // run cn submit
)

// Result is returned after the TUI exits and carries any pending action.
type Result struct {
	Action   PendingAction
	Branch   string
	PRNumber int
}

// Model is the bubbletea model for cn status.
type Model struct {
	Layers []Layer
	Cursor int
	result Result
}

var (
	styleFaint   = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	styleBold    = lipgloss.NewStyle().Bold(true)
	styleHeader  = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
	styleSel     = lipgloss.NewStyle().Background(lipgloss.Color("236"))
	styleSelBold = lipgloss.NewStyle().Background(lipgloss.Color("236")).Bold(true).Foreground(lipgloss.Color("255"))
	styleSelPos  = lipgloss.NewStyle().Background(lipgloss.Color("236")).Foreground(lipgloss.Color("241"))
	styleCurrent = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
	styleMerged  = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	styleOpen    = lipgloss.NewStyle().Foreground(lipgloss.Color("75"))
	styleWarn    = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	styleKey     = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	styleBox     = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("238")).
			Padding(1, 3)
)

func (m Model) Result() Result { return m.result }

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "up", "k":
		if m.Cursor > 0 {
			m.Cursor--
		}
	case "down", "j":
		if m.Cursor < len(m.Layers)-1 {
			m.Cursor++
		}
	case "c", "enter":
		if len(m.Layers) > 0 {
			m.result = Result{Action: ActionCheckout, Branch: m.Layers[m.Cursor].Branch}
			return m, tea.Quit
		}
	case "o":
		if len(m.Layers) > 0 && m.Layers[m.Cursor].PRNumber > 0 {
			m.result = Result{Action: ActionOpenPR, PRNumber: m.Layers[m.Cursor].PRNumber}
			return m, tea.Quit
		}
	case "s":
		if len(m.Layers) > 0 {
			m.result = Result{Action: ActionSync, Branch: m.Layers[m.Cursor].Branch}
			return m, tea.Quit
		}
	case "p":
		if len(m.Layers) > 0 {
			m.result = Result{Action: ActionSubmit, Branch: m.Layers[m.Cursor].Branch}
			return m, tea.Quit
		}
	case "q", "ctrl+c", "esc":
		m.result = Result{Action: ActionNone}
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) View() string {
	if len(m.Layers) == 0 {
		return styleBox.Render(
			styleBold.Render("chainrail") + "\n\n" +
				styleFaint.Render("  no stack branches found in this repo\n\n") +
				styleFaint.Render("  get started: ") + "cn add <slug>",
		)
	}

	var b strings.Builder
	b.WriteString(styleBold.Render("chainrail") + "\n")

	prevStack := ""
	for i, l := range m.Layers {
		// stack group header
		if l.Stack != prevStack {
			if prevStack != "" {
				b.WriteString("\n")
			}
			b.WriteString(styleFaint.Render(strings.Repeat("─", 54)) + "\n")
			b.WriteString(styleHeader.Render("  "+l.Stack) + "\n\n")
			prevStack = l.Stack
		}

		sel := i == m.Cursor

		// tree indent for --all mode
		indent := strings.Repeat("   ", l.Depth)
		connector := ""
		if l.Depth > 0 {
			connector = styleFaint.Render("└── ")
		}

		// cursor / current marker
		var marker string
		if sel {
			marker = styleSelBold.Render(" ❯ ")
		} else if l.IsCurrent {
			marker = styleCurrent.Render(" ▶ ")
		} else {
			marker = "   "
		}
		b.WriteString(marker + indent + connector)

		// label: prefer title in tree mode, branch name in stack mode
		label := l.Branch
		if l.Title != "" && l.Depth >= 0 {
			label = l.Title
		}
		maxLabel := 42 - l.Depth*3
		if len(label) > maxLabel && maxLabel > 8 {
			label = label[:maxLabel-1] + "…"
		}
		if sel {
			b.WriteString(styleSelBold.Render(label))
		} else if l.IsCurrent {
			b.WriteString(styleCurrent.Render(label))
		} else {
			b.WriteString(label)
		}

		pad := maxLabel - len([]rune(label)) + 2
		if pad < 2 {
			pad = 2
		}
		if sel {
			b.WriteString(styleSel.Render(strings.Repeat(" ", pad)))
		} else {
			b.WriteString(strings.Repeat(" ", pad))
		}

		// PR state
		if l.PRNumber > 0 {
			prNum := fmt.Sprintf("#%-4d ", l.PRNumber)
			if sel {
				b.WriteString(styleSel.Render(prNum))
			} else {
				b.WriteString(styleFaint.Render(prNum))
			}
			switch l.PRState {
			case "MERGED":
				b.WriteString(styleMerged.Render("✓ merged"))
			case "OPEN":
				b.WriteString(styleOpen.Render("● open  "))
			default:
				b.WriteString(styleFaint.Render(l.PRState))
			}
		} else {
			nopr := "  no PR yet"
			if sel {
				b.WriteString(styleSel.Render(nopr))
			} else {
				b.WriteString(styleFaint.Render(nopr))
			}
		}

		if l.NeedsSync {
			b.WriteString("  " + styleWarn.Render("⚠ sync"))
		}

		b.WriteString("\n")
	}

	// keybindings
	b.WriteString("\n")
	b.WriteString(styleFaint.Render("  "))
	b.WriteString(styleKey.Render("↑↓") + styleFaint.Render(" navigate  "))
	b.WriteString(styleKey.Render("c") + styleFaint.Render(" checkout  "))
	b.WriteString(styleKey.Render("o") + styleFaint.Render(" open PR  "))
	b.WriteString(styleKey.Render("s") + styleFaint.Render(" sync  "))
	b.WriteString(styleKey.Render("p") + styleFaint.Render(" submit  "))
	b.WriteString(styleKey.Render("q") + styleFaint.Render(" quit"))

	return styleBox.Render(b.String())
}
