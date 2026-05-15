package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

var (
	faint    = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	bold     = lipgloss.NewStyle().Bold(true)
	pink     = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
	green    = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	red      = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	yellow   = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	blue     = lipgloss.NewStyle().Foreground(lipgloss.Color("75"))
	grey     = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	selRow   = lipgloss.NewStyle().Background(lipgloss.Color("236")).Foreground(lipgloss.Color("255")).Bold(true)
	selFaint = lipgloss.NewStyle().Background(lipgloss.Color("236")).Foreground(lipgloss.Color("241"))
	box      = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("238")).
			Padding(1, 3)
	tabActive = lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")).
			Bold(true).
			Border(lipgloss.Border{Bottom: "─"}, false, false, true, false).
			BorderForeground(lipgloss.Color("205")).
			Padding(0, 2)
	tabInactive = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Padding(0, 2)
)

func div() string  { return faint.Render(strings.Repeat("─", 60)) }
func head() string { return bold.Render("chainrail") + faint.Render(" · virtual-architect · staging") }

// ── mockup 1: current state ───────────────────────────────────────────────

func mockCurrent() string {
	var b strings.Builder
	b.WriteString(head() + "\n")
	b.WriteString(div() + "\n")
	b.WriteString(pink.Render("  ignorelogs") + "\n\n")

	rows := []struct{ num, title string }{
		{"#14", "brayschurman/ignorelogs-1-ignorelogs"},
		{"#15", "brayschurman/ignorelogs-2-toolscopy"},
	}
	for i, r := range rows {
		prefix := "   "
		if i == 0 {
			prefix = selRow.Render(" ❯ ")
		}
		b.WriteString(prefix + faint.Render(fmt.Sprintf("%-5s", r.num)) + r.title + "  " + blue.Render("● open") + "\n")
	}

	b.WriteString("\n" + faint.Render("  ↑↓ navigate  c checkout  o open PR  s sync  p submit  q quit"))
	return box.Render(b.String())
}

// ── mockup 2: CI + review state (#8) ─────────────────────────────────────

func mockCI() string {
	var b strings.Builder
	b.WriteString(head() + "\n")
	b.WriteString(div() + "\n")
	b.WriteString(pink.Render("  ignorelogs") + "\n\n")

	type row struct {
		num, title, ci, review string
		sel                    bool
	}
	rows := []row{
		{"#14", "add .logs/ to .gitignore", green.Render("✅"), green.Render("✓ approved"), true},
		{"#15", "document src/content/tools in README", yellow.Render("⏳"), faint.Render("👀 needs review"), false},
	}
	for _, r := range rows {
		var marker, numS, titleS, pad string
		if r.sel {
			marker = selRow.Render(" ❯ ")
			numS = selFaint.Render(fmt.Sprintf("%-5s", r.num))
			titleS = selRow.Render(r.title)
			pad = selRow.Render("  ")
		} else {
			marker = "   "
			numS = faint.Render(fmt.Sprintf("%-5s", r.num))
			titleS = r.title
			pad = "  "
		}
		_ = pad
		b.WriteString(marker + numS + titleS + "\n")
		b.WriteString("        " + r.ci + "  " + r.review + "\n")
	}

	b.WriteString("\n" + faint.Render("  ↑↓ navigate  c checkout  o open PR  s sync  p submit  q quit"))
	return box.Render(b.String())
}

// ── mockup 3: compact CI inline (#8 refined) ─────────────────────────────

func mockCIInline() string {
	var b strings.Builder
	b.WriteString(head() + "\n")
	b.WriteString(div() + "\n")

	b.WriteString(pink.Render("  ignorelogs") + "\n\n")

	type row struct {
		num, title, state, ci, review string
		sel                           bool
	}
	rows := []row{
		{"#14", "add .logs/ to .gitignore      ", blue.Render("● open"), green.Render("✅"), green.Render("approved"), true},
		{"#15", "document src/content/tools    ", blue.Render("● open"), yellow.Render("⏳"), faint.Render("needs review"), false},
	}
	for _, r := range rows {
		line := fmt.Sprintf("%-5s%s  %s  %s  %s", r.num, r.title, r.state, r.ci, r.review)
		if r.sel {
			b.WriteString(selRow.Render(" ❯ " + line) + "\n")
		} else {
			b.WriteString("   " + line + "\n")
		}
	}

	b.WriteString("\n" + faint.Render("  ↑↓ navigate  c checkout  o open PR  s sync  p submit  q quit"))
	return box.Render(b.String())
}

// ── mockup 4: virtual-architect stack with CI ─────────────────────────────

func mockVAStack() string {
	var b strings.Builder
	b.WriteString(head() + "\n")
	b.WriteString(div() + "\n")

	type stackRow struct {
		num, title, ci, review string
		depth                  int
		sel                    bool
	}
	stacks := []struct {
		name string
		rows []stackRow
	}{
		{
			"floorplan-export",
			[]stackRow{
				{"#685", "DEV-61: delete unused legacy db fields", green.Render("✅"), green.Render("approved"), 0, false},
				{"#686", "AI-299: expand floorplan checks", red.Render("❌"), faint.Render("needs review"), 1, true},
				{"#687", "wall-drawing UX tweaks", faint.Render("—"), faint.Render("blocked"), 2, false},
			},
		},
		{
			"voice-mode",
			[]stackRow{
				{"#502", "Voice mode", green.Render("✅"), green.Render("approved"), 0, false},
				{"#614", "Voice recording UI", green.Render("✅"), faint.Render("needs review"), 1, false},
			},
		},
	}

	for _, stack := range stacks {
		b.WriteString("\n" + pink.Render("  "+stack.name) + "\n\n")
		for _, r := range stack.rows {
			indent := strings.Repeat("   ", r.depth)
			connector := ""
			if r.depth > 0 {
				connector = faint.Render("└── ")
			}
			line := fmt.Sprintf("%-5s%-40s%s  %s", r.num, r.title, r.ci, r.review)
			if r.sel {
				b.WriteString(selRow.Render(" ❯ ") + indent + connector + selRow.Render(line) + "\n")
			} else {
				b.WriteString("    " + indent + connector + faint.Render(r.num) + "  " + r.title + "  " + r.ci + "  " + r.review + "\n")
			}
		}
	}

	b.WriteString("\n" + faint.Render("  ↑↓ navigate  c checkout  o open PR  s sync  p submit  q quit"))
	return box.Render(b.String())
}

// ── mockup 5: stack health summary (#10) ─────────────────────────────────

func mockStackHealth() string {
	var b strings.Builder
	b.WriteString(head() + "\n")
	b.WriteString(div() + "\n\n")

	type healthRow struct {
		name, count, ci, reviews, age string
	}
	stacks := []healthRow{
		{"floorplan-export", "3 PRs", red.Render("❌ 1 failing"), green.Render("✓ 1 approved"), red.Render("🔴 14d")},
		{"voice-mode", "2 PRs", green.Render("✅ passing"), green.Render("✓ 2 approved"), yellow.Render("🟡 6d")},
		{"ignorelogs", "2 PRs", yellow.Render("⏳ pending"), faint.Render("👀 needs review"), grey.Render("1d")},
	}

	for i, s := range stacks {
		marker := "▸ "
		if i == 0 {
			marker = selRow.Render(" ❯ ")
			b.WriteString(marker + pink.Render(s.name) + "  " + faint.Render(s.count) + "  " + s.ci + "  " + s.reviews + "  " + s.age + "\n")
		} else {
			b.WriteString("  " + marker + pink.Render(s.name) + "  " + faint.Render(s.count) + "  " + s.ci + "  " + s.reviews + "  " + s.age + "\n")
		}
	}

	b.WriteString("\n" + faint.Render("  ↑↓ navigate  enter expand  o open  s sync  q quit"))
	return box.Render(b.String())
}

// ── mockup 6: tab views (#9) ─────────────────────────────────────────────

func mockTabs() string {
	var b strings.Builder

	tabs := tabActive.Render("Mine") + tabInactive.Render("Review") + tabInactive.Render("All")
	b.WriteString(bold.Render("chainrail") + faint.Render(" · virtual-architect") + "\n")
	b.WriteString(tabs + "\n")
	b.WriteString(div() + "\n\n")

	b.WriteString(pink.Render("  floorplan-export") + "\n")
	rows := []struct{ num, title, ci, review string }{
		{"#685", "DEV-61: delete unused legacy db fields", red.Render("❌"), faint.Render("needs review")},
		{"#686", "AI-299: expand floorplan checks", red.Render("❌"), faint.Render("blocked")},
		{"#687", "wall-drawing UX tweaks", faint.Render("—"), faint.Render("blocked")},
	}
	for i, r := range rows {
		indent := strings.Repeat("   ", i)
		conn := ""
		if i > 0 {
			conn = faint.Render("└── ")
		}
		b.WriteString("   " + indent + conn + faint.Render(r.num) + "  " + r.title + "  " + r.ci + "  " + r.review + "\n")
	}

	b.WriteString("\n" + pink.Render("  voice-mode") + "\n")
	b.WriteString("   " + faint.Render("#502") + "  Voice mode            " + green.Render("✅") + "  " + green.Render("approved") + "\n")
	b.WriteString("   " + faint.Render("└── ") + faint.Render("#614") + "  Voice recording UI     " + green.Render("✅") + "  " + faint.Render("needs review") + "\n")

	b.WriteString("\n" + faint.Render("  tab switch view  ↑↓ navigate  c checkout  o open PR  s sync  q quit"))
	return box.Render(b.String())
}

// ── mockup 7: full air traffic control view ───────────────────────────────

func mockATC() string {
	var b strings.Builder

	tabs := tabActive.Render("Mine") + tabInactive.Render("Review") + tabInactive.Render("All")
	b.WriteString(bold.Render("chainrail") + faint.Render(" · virtual-architect · staging") + "\n")
	b.WriteString(tabs + "\n")
	b.WriteString(div() + "\n\n")

	// stack 1
	b.WriteString(pink.Render("  floorplan-export") + "  " +
		faint.Render("3 PRs") + "  " +
		red.Render("❌ 1 failing") + "  " +
		faint.Render("next: fix #685") +
		red.Render("  🔴 14d") + "\n")

	atcRows := []struct {
		indent      int
		num, title  string
		ci, review  string
		sel         bool
	}{
		{0, "#685", "DEV-61: delete unused legacy db fields", red.Render("❌ CI failing"), faint.Render("needs review"), true},
		{1, "#686", "AI-299: expand floorplan checks", faint.Render("— blocked"), faint.Render("blocked"), false},
		{2, "#687", "wall-drawing UX tweaks", faint.Render("— blocked"), faint.Render("blocked"), false},
	}
	for _, r := range atcRows {
		indent := strings.Repeat("   ", r.indent)
		conn := ""
		if r.indent > 0 {
			conn = faint.Render("└── ")
		}
		if r.sel {
			b.WriteString(selRow.Render(" ❯ ")+indent+conn+selRow.Render(fmt.Sprintf("%-5s%-38s", r.num, r.title))+r.ci+"  "+r.review+"\n")
		} else {
			b.WriteString("    "+indent+conn+faint.Render(fmt.Sprintf("%-5s", r.num))+r.title+"  "+r.ci+"  "+r.review+"\n")
		}
	}

	b.WriteString("\n")

	// stack 2
	b.WriteString(pink.Render("  voice-mode") + "  " +
		faint.Render("2 PRs") + "  " +
		green.Render("✅ passing") + "  " +
		green.Render("✓ 2 approved") +
		yellow.Render("  🟡 6d") + "\n")
	b.WriteString("    " + faint.Render("#502") + "  Voice mode               " + green.Render("✅") + "  " + green.Render("approved") + "\n")
	b.WriteString("    " + faint.Render("└── #614") + "  Voice recording UI       " + green.Render("✅") + "  " + faint.Render("needs review") + "\n")

	// context bar for selected row
	b.WriteString("\n" + faint.Render(strings.Repeat("─", 60)) + "\n")
	b.WriteString(bold.Render("#685") + " · " + faint.Render("DEV-61: delete unused legacy db fields") + "\n")
	b.WriteString(red.Render("❌ CI failing") + "  " + faint.Render("👀 needs review") + "  " + faint.Render("14d old") + "  " + faint.Render("+284 −91") + "\n")
	b.WriteString("\n" + faint.Render("  c checkout  o open PR  r review  s sync stack  q quit"))

	return box.Render(b.String())
}

func main() {
	which := "all"
	if len(os.Args) > 1 {
		which = os.Args[1]
	}

	renders := map[string]func() string{
		"current":   mockCurrent,
		"ci":        mockCIInline,
		"va-stack":  mockVAStack,
		"health":    mockStackHealth,
		"tabs":      mockTabs,
		"atc":       mockATC,
	}

	if which == "all" {
		labels := []string{"current", "ci", "va-stack", "health", "tabs", "atc"}
		titles := map[string]string{
			"current":  "CURRENT STATE",
			"ci":       "#8 — CI + review state per row",
			"va-stack": "#8 — CI on real VA-style stack",
			"health":   "#10 — Stack health summary",
			"tabs":     "#9 — Tab views (Mine / Review / All)",
			"atc":      "COMBINED — Air traffic control view",
		}
		for _, k := range labels {
			_ = time.Now() // keep import
			fmt.Println()
			fmt.Println(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("226")).Render("  " + titles[k]))
			fmt.Println(renders[k]())
			fmt.Println()
		}
	} else if fn, ok := renders[which]; ok {
		fmt.Println(fn())
	} else {
		fmt.Fprintf(os.Stderr, "unknown mockup %q\n", which)
		os.Exit(1)
	}
}
