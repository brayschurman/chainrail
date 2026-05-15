package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TitleUpdater pushes a new PR title to the remote. status.go wires this to
// the gh client; tests inject a fake.
type TitleUpdater func(ctx context.Context, number int, newTitle string) error

// titleUpdatedMsg is dispatched after a UpdatePRTitle round-trip completes.
type titleUpdatedMsg struct {
	number int
	title  string
	err    error
}

// Now is overridable in tests for stable staleness output.
var Now = time.Now

// Layer is one branch in a stack, also used as JSON output.
type Layer struct {
	Stack          string `json:"stack"`
	Position       int    `json:"position"`
	Branch         string `json:"branch"`
	Title          string `json:"title"` // PR title (shown in --all mode)
	PRNumber       int    `json:"pr_number"`
	PRState        string `json:"pr_state"`        // "OPEN", "MERGED", or ""
	CIStatus       string `json:"ci_status"`       // SUCCESS, FAILURE, PENDING, ""
	ReviewDecision string `json:"review_decision"` // APPROVED, CHANGES_REQUESTED, REVIEW_REQUIRED, ""
	UpdatedAt      string `json:"updated_at"`      // RFC3339; "" if unknown
	IsCurrent      bool   `json:"is_current"`
	NeedsSync      bool   `json:"needs_sync"`
	Depth          int    `json:"depth"` // nesting depth for tree view (--all mode)
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

// Tab is one view in the status TUI (Mine / Review / All).
type Tab struct {
	Label  string
	Layers []Layer
	Cursor int
}

// Model is the bubbletea model for cn status.
type Model struct {
	// Layers and Cursor describe the currently visible view. When Tabs is
	// non-empty these mirror m.Tabs[m.ActiveTab] and are swapped on tab change.
	Layers []Layer
	Cursor int
	result Result

	// Tabs, when set, enables the tab bar across the top. Leave nil for a
	// single-view TUI (e.g. cn status --all without tabs).
	Tabs      []Tab
	ActiveTab int

	// Updater is invoked to push a new PR title. If nil, the rename action is
	// disabled.
	Updater TitleUpdater

	editingTitle bool
	titleInput   textinput.Model
	titleErr     string
}

// activate switches to the tab at idx, saving the current cursor first.
func (m Model) activate(idx int) Model {
	if len(m.Tabs) == 0 || idx < 0 || idx >= len(m.Tabs) {
		return m
	}
	// Save current cursor back into the previously active tab.
	if m.ActiveTab >= 0 && m.ActiveTab < len(m.Tabs) {
		m.Tabs[m.ActiveTab].Cursor = m.Cursor
		m.Tabs[m.ActiveTab].Layers = m.Layers
	}
	m.ActiveTab = idx
	m.Layers = m.Tabs[idx].Layers
	m.Cursor = m.Tabs[idx].Cursor
	if m.Cursor >= len(m.Layers) {
		m.Cursor = 0
	}
	return m
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
	styleStale   = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	styleKey     = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	styleBox     = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("238")).
			Padding(1, 3)
)

func (m Model) Result() Result { return m.result }

// findParent returns the layer that the given index's PR is stacked onto, or
// nil if it's a root. Works for both stack mode (prior layer in same stack)
// and tree mode (most recent earlier layer with Depth = l.Depth - 1).
func findParent(layers []Layer, idx int) *Layer {
	if idx <= 0 || idx >= len(layers) {
		return nil
	}
	cur := layers[idx]
	if cur.Depth > 0 {
		// Tree mode: walk back to the first earlier layer with depth-1.
		for j := idx - 1; j >= 0; j-- {
			if layers[j].Depth == cur.Depth-1 {
				return &layers[j]
			}
		}
		return nil
	}
	// cur.Depth == 0. If any earlier layer has Depth > 0, we're in tree mode
	// and depth-0 rows are independent roots — no parent.
	for j := 0; j < idx; j++ {
		if layers[j].Depth > 0 {
			return nil
		}
	}
	// Stack mode: previous layer in the same stack.
	prev := layers[idx-1]
	if prev.Stack == cur.Stack {
		return &prev
	}
	return nil
}

// derivedState collapses CIStatus, ReviewDecision, parent state, and
// staleness into a single headline word per row.
func derivedState(l Layer, parent *Layer, now time.Time) string {
	if l.PRNumber == 0 {
		return ""
	}
	if l.PRState == "MERGED" {
		return "merged"
	}
	if l.PRState != "OPEN" {
		return strings.ToLower(l.PRState)
	}
	if l.CIStatus == "FAILURE" {
		return "failing"
	}
	parentBlocks := parent != nil && parent.PRNumber > 0 && parent.PRState == "OPEN" &&
		(parent.CIStatus == "FAILURE" || parent.ReviewDecision != "APPROVED")
	if parentBlocks {
		return "blocked"
	}
	parentOk := parent == nil || parent.PRNumber == 0 || parent.PRState == "MERGED"
	if l.ReviewDecision == "APPROVED" && l.CIStatus == "SUCCESS" && parentOk {
		return "ready"
	}
	if l.UpdatedAt != "" {
		if t, err := time.Parse(time.RFC3339, l.UpdatedAt); err == nil {
			if int(now.Sub(t).Hours()/24) > 7 {
				return "stale"
			}
		}
	}
	return "open"
}

// renderState renders the derived state with appropriate color. Pads to 8
// chars so the badges line up across rows.
func renderState(state string, selected bool) string {
	var style lipgloss.Style
	switch state {
	case "merged":
		style = styleMerged
	case "ready":
		style = styleMerged
	case "failing":
		style = styleStale
	case "blocked":
		style = styleWarn
	case "stale":
		style = styleFaint
	case "open":
		style = styleOpen
	default:
		style = styleFaint
	}
	text := fmt.Sprintf("%-8s", state)
	if selected {
		return styleSel.Render(text)
	}
	return style.Render(text)
}

// renderTabBar renders a horizontal tab bar with the active tab highlighted.
func renderTabBar(tabs []Tab, active int) string {
	parts := make([]string, len(tabs))
	for i, t := range tabs {
		label := fmt.Sprintf("[%s]", t.Label)
		if i == active {
			parts[i] = styleHeader.Render(label)
		} else {
			parts[i] = styleFaint.Render(label)
		}
	}
	return strings.Join(parts, " ")
}

// stackSummary returns a one-line health summary for the given stack: PR count,
// CI failures, approvals, and a staleness indicator based on the most recently
// updated PR in the stack. Returns "" if the stack has no PRs.
func stackSummary(layers []Layer, stack string) string {
	prCount := 0
	failing := 0
	approved := 0
	var newest time.Time
	for _, l := range layers {
		if l.Stack != stack {
			continue
		}
		if l.PRNumber == 0 || l.PRState != "OPEN" {
			continue
		}
		prCount++
		if l.CIStatus == "FAILURE" {
			failing++
		}
		if l.ReviewDecision == "APPROVED" {
			approved++
		}
		if l.UpdatedAt != "" {
			if t, err := time.Parse(time.RFC3339, l.UpdatedAt); err == nil && t.After(newest) {
				newest = t
			}
		}
	}
	if prCount == 0 {
		return ""
	}
	parts := []string{fmt.Sprintf("%d PR%s", prCount, pluralS(prCount))}
	if failing > 0 {
		parts = append(parts, styleWarn.Render(fmt.Sprintf("✗ %d failing", failing)))
	}
	if approved > 0 {
		parts = append(parts, styleMerged.Render(fmt.Sprintf("✓ %d approved", approved)))
	}
	if stale := stalenessLabel(newest); stale != "" {
		parts = append(parts, stale)
	}
	return strings.Join(parts, " · ")
}

func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// stalenessLabel returns "" for <3 days, "yellow" tag for 3-7 days,
// "red" tag for >7 days. Empty when t is zero.
func stalenessLabel(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	days := int(Now().Sub(t).Hours() / 24)
	if days < 3 {
		return ""
	}
	if days <= 7 {
		return styleWarn.Render(fmt.Sprintf("◐ %dd", days))
	}
	return styleStale.Render(fmt.Sprintf("● %dd", days))
}

// ciBadge renders a compact CI rollup indicator. Empty string when no checks.
func ciBadge(status string) string {
	switch status {
	case "SUCCESS":
		return styleMerged.Render("✓ CI")
	case "FAILURE":
		return styleWarn.Render("✗ CI")
	case "PENDING":
		return styleFaint.Render("· CI")
	}
	return ""
}

// reviewBadge renders a compact review decision indicator.
func reviewBadge(decision string) string {
	switch decision {
	case "APPROVED":
		return styleMerged.Render("✓ approved")
	case "CHANGES_REQUESTED":
		return styleWarn.Render("✗ changes")
	case "REVIEW_REQUIRED":
		return styleFaint.Render("· review")
	}
	return ""
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if upd, ok := msg.(titleUpdatedMsg); ok {
		if upd.err != nil {
			m.titleErr = upd.err.Error()
			return m, nil
		}
		m.titleErr = ""
		for i := range m.Layers {
			if m.Layers[i].PRNumber == upd.number {
				m.Layers[i].Title = upd.title
			}
		}
		return m, nil
	}

	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	if m.editingTitle {
		return m.updateEditing(key)
	}

	switch key.String() {
	case "tab":
		if len(m.Tabs) > 0 {
			return m.activate((m.ActiveTab + 1) % len(m.Tabs)), nil
		}
	case "shift+tab":
		if len(m.Tabs) > 0 {
			return m.activate((m.ActiveTab - 1 + len(m.Tabs)) % len(m.Tabs)), nil
		}
	case "1", "2", "3":
		if len(m.Tabs) > 0 {
			idx := int(key.String()[0] - '1')
			return m.activate(idx), nil
		}
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
	case "r":
		if m.Updater != nil && len(m.Layers) > 0 && m.Layers[m.Cursor].PRNumber > 0 {
			ti := textinput.New()
			ti.SetValue(m.Layers[m.Cursor].Title)
			ti.Focus()
			ti.CharLimit = 256
			ti.Width = 60
			m.titleInput = ti
			m.editingTitle = true
			m.titleErr = ""
			return m, textinput.Blink
		}
	case "q", "ctrl+c", "esc":
		m.result = Result{Action: ActionNone}
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) updateEditing(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "enter":
		newTitle := strings.TrimSpace(m.titleInput.Value())
		layer := m.Layers[m.Cursor]
		m.editingTitle = false
		if newTitle == "" || newTitle == layer.Title {
			return m, nil
		}
		num := layer.PRNumber
		updater := m.Updater
		return m, func() tea.Msg {
			err := updater(context.Background(), num, newTitle)
			return titleUpdatedMsg{number: num, title: newTitle, err: err}
		}
	case "esc":
		m.editingTitle = false
		m.titleErr = ""
		return m, nil
	}
	var cmd tea.Cmd
	m.titleInput, cmd = m.titleInput.Update(key)
	return m, cmd
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
	b.WriteString(styleBold.Render("chainrail"))
	if len(m.Tabs) > 0 {
		b.WriteString("   " + renderTabBar(m.Tabs, m.ActiveTab))
	}
	b.WriteString("\n")

	if len(m.Layers) == 0 && len(m.Tabs) > 0 {
		b.WriteString("\n  " + styleFaint.Render("nothing here") + "\n")
	}

	prevStack := ""
	for i, l := range m.Layers {
		// stack group header
		if l.Stack != prevStack {
			if prevStack != "" {
				b.WriteString("\n")
			}
			b.WriteString(styleFaint.Render(strings.Repeat("─", 54)) + "\n")
			header := styleHeader.Render("  " + l.Stack)
			if summary := stackSummary(m.Layers, l.Stack); summary != "" {
				header += "  " + styleFaint.Render(summary)
			}
			b.WriteString(header + "\n\n")
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

		if sel && m.editingTitle {
			b.WriteString(m.titleInput.View())
			b.WriteString("\n")
			continue
		}

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
			parent := findParent(m.Layers, i)
			b.WriteString(renderState(derivedState(l, parent, Now()), sel))
		} else {
			nopr := "  no PR yet"
			if sel {
				b.WriteString(styleSel.Render(nopr))
			} else {
				b.WriteString(styleFaint.Render(nopr))
			}
		}

		if badge := ciBadge(l.CIStatus); badge != "" {
			b.WriteString("  " + badge)
		}
		if badge := reviewBadge(l.ReviewDecision); badge != "" {
			b.WriteString(" " + badge)
		}

		if l.NeedsSync {
			b.WriteString("  " + styleWarn.Render("⚠ sync"))
		}

		b.WriteString("\n")
	}

	if m.titleErr != "" {
		b.WriteString("\n  " + styleWarn.Render("rename failed: "+m.titleErr) + "\n")
	}

	// keybindings
	b.WriteString("\n")
	b.WriteString(styleFaint.Render("  "))
	if m.editingTitle {
		b.WriteString(styleKey.Render("enter") + styleFaint.Render(" save  "))
		b.WriteString(styleKey.Render("esc") + styleFaint.Render(" cancel"))
	} else {
		if len(m.Tabs) > 0 {
			b.WriteString(styleKey.Render("tab") + styleFaint.Render(" switch  "))
		}
		b.WriteString(styleKey.Render("↑↓") + styleFaint.Render(" navigate  "))
		b.WriteString(styleKey.Render("c") + styleFaint.Render(" checkout  "))
		b.WriteString(styleKey.Render("o") + styleFaint.Render(" open PR  "))
		b.WriteString(styleKey.Render("r") + styleFaint.Render(" rename  "))
		b.WriteString(styleKey.Render("s") + styleFaint.Render(" sync  "))
		b.WriteString(styleKey.Render("p") + styleFaint.Render(" submit  "))
		b.WriteString(styleKey.Render("q") + styleFaint.Render(" quit"))
	}

	return styleBox.Render(b.String())
}
