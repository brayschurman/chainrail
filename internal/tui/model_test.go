package tui

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestStackSummary_EmptyWhenNoOpenPRs(t *testing.T) {
	layers := []Layer{
		{Stack: "feat", Branch: "feat/1"},
	}
	if got := stackSummary(layers, "feat"); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestStackSummary_CountsAndBadges(t *testing.T) {
	Now = func() time.Time { return time.Date(2026, 5, 14, 0, 0, 0, 0, time.UTC) }
	t.Cleanup(func() { Now = time.Now })

	layers := []Layer{
		{Stack: "feat", PRNumber: 1, PRState: "OPEN", CIStatus: "FAILURE", ReviewDecision: "REVIEW_REQUIRED", UpdatedAt: "2026-04-30T00:00:00Z"},
		{Stack: "feat", PRNumber: 2, PRState: "OPEN", CIStatus: "SUCCESS", ReviewDecision: "APPROVED", UpdatedAt: "2026-05-05T00:00:00Z"},
		// Newest PR drives staleness — set it 4 days before Now (2026-05-14).
		{Stack: "feat", PRNumber: 3, PRState: "OPEN", CIStatus: "SUCCESS", ReviewDecision: "APPROVED", UpdatedAt: "2026-05-10T00:00:00Z"},
		// Wrong stack — should be ignored.
		{Stack: "other", PRNumber: 9, PRState: "OPEN", CIStatus: "FAILURE"},
	}

	got := stripANSI(stackSummary(layers, "feat"))
	wantParts := []string{"3 PRs", "✗ 1 failing", "✓ 2 approved", "◐ 4d"}
	for _, p := range wantParts {
		if !strings.Contains(got, p) {
			t.Errorf("summary %q missing %q", got, p)
		}
	}
}

func TestStalenessLabel_Buckets(t *testing.T) {
	Now = func() time.Time { return time.Date(2026, 5, 14, 0, 0, 0, 0, time.UTC) }
	t.Cleanup(func() { Now = time.Now })

	cases := []struct {
		days int
		want string
	}{
		{0, ""},
		{2, ""},
		{3, "◐ 3d"},
		{7, "◐ 7d"},
		{8, "● 8d"},
		{30, "● 30d"},
	}
	for _, c := range cases {
		got := stripANSI(stalenessLabel(Now().Add(-time.Duration(c.days) * 24 * time.Hour)))
		if got != c.want {
			t.Errorf("days=%d: got %q want %q", c.days, got, c.want)
		}
	}
}

func TestCIAndReviewBadges(t *testing.T) {
	cases := []struct {
		ci, review, ciWant, revWant string
	}{
		{"SUCCESS", "APPROVED", "✓ CI", "✓ approved"},
		{"FAILURE", "CHANGES_REQUESTED", "✗ CI", "✗ changes"},
		{"PENDING", "REVIEW_REQUIRED", "· CI", "· review"},
		{"", "", "", ""},
	}
	for _, c := range cases {
		if got := stripANSI(ciBadge(c.ci)); got != c.ciWant {
			t.Errorf("ciBadge(%q) = %q want %q", c.ci, got, c.ciWant)
		}
		if got := stripANSI(reviewBadge(c.review)); got != c.revWant {
			t.Errorf("reviewBadge(%q) = %q want %q", c.review, got, c.revWant)
		}
	}
}

func TestRenameFlow_EnterTriggersUpdaterAndAppliesNewTitle(t *testing.T) {
	var calledNum int
	var calledTitle string
	m := Model{
		Layers: []Layer{
			{Stack: "feat", Branch: "feat/x", Title: "old title", PRNumber: 42, PRState: "OPEN"},
		},
		Cursor: 0,
		Updater: func(_ context.Context, num int, title string) error {
			calledNum = num
			calledTitle = title
			return nil
		},
	}

	// 'r' enters editing mode.
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = next.(Model)
	if !m.editingTitle {
		t.Fatal("expected editingTitle = true after pressing r")
	}
	if m.titleInput.Value() != "old title" {
		t.Fatalf("input prefilled with %q want %q", m.titleInput.Value(), "old title")
	}

	// Replace the input value and press enter — the model returns a Cmd that
	// runs the updater and produces a titleUpdatedMsg.
	m.titleInput.SetValue("new title")
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if m.editingTitle {
		t.Fatal("expected editingTitle = false after Enter")
	}
	if cmd == nil {
		t.Fatal("expected a tea.Cmd to run the updater")
	}
	msg := cmd()
	upd, ok := msg.(titleUpdatedMsg)
	if !ok {
		t.Fatalf("expected titleUpdatedMsg, got %T", msg)
	}
	if upd.err != nil {
		t.Fatalf("unexpected updater error: %v", upd.err)
	}
	if calledNum != 42 || calledTitle != "new title" {
		t.Fatalf("updater called with num=%d title=%q", calledNum, calledTitle)
	}

	// Feed the msg back through Update — the layer's Title should update.
	next, _ = m.Update(upd)
	m = next.(Model)
	if m.Layers[0].Title != "new title" {
		t.Errorf("layer title = %q, want %q", m.Layers[0].Title, "new title")
	}
}

func TestRenameFlow_EscCancelsWithoutCallingUpdater(t *testing.T) {
	called := false
	m := Model{
		Layers: []Layer{{Title: "old", PRNumber: 1, PRState: "OPEN"}},
		Updater: func(_ context.Context, _ int, _ string) error {
			called = true
			return nil
		},
	}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = next.(Model)
	m.titleInput.SetValue("changed")

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(Model)
	if m.editingTitle {
		t.Fatal("expected editing to end on esc")
	}
	if cmd != nil {
		t.Fatal("expected no Cmd on cancel")
	}
	if called {
		t.Fatal("updater should not run on cancel")
	}
	if m.Layers[0].Title != "old" {
		t.Errorf("title should be unchanged: %q", m.Layers[0].Title)
	}
}

func TestRenameFlow_UpdaterErrorSurfacesAsTitleErr(t *testing.T) {
	m := Model{
		Layers: []Layer{{Title: "old", PRNumber: 1, PRState: "OPEN"}},
	}
	next, _ := m.Update(titleUpdatedMsg{number: 1, title: "x", err: errors.New("boom")})
	m = next.(Model)
	if m.titleErr != "boom" {
		t.Errorf("titleErr = %q want %q", m.titleErr, "boom")
	}
	if m.Layers[0].Title != "old" {
		t.Errorf("title should not change on error: %q", m.Layers[0].Title)
	}
}

func TestTabSwitching_PreservesPerTabCursor(t *testing.T) {
	mine := []Layer{
		{Stack: "a", Branch: "feat/a1", PRNumber: 1, PRState: "OPEN"},
		{Stack: "a", Branch: "feat/a2", PRNumber: 2, PRState: "OPEN"},
		{Stack: "a", Branch: "feat/a3", PRNumber: 3, PRState: "OPEN"},
	}
	review := []Layer{
		{Stack: "main", Branch: "other/x", PRNumber: 9, PRState: "OPEN"},
	}
	all := []Layer{
		{Stack: "main", Branch: "feat/a1", PRNumber: 1, PRState: "OPEN"},
		{Stack: "main", Branch: "other/x", PRNumber: 9, PRState: "OPEN"},
	}
	m := Model{
		Layers: mine, Cursor: 0,
		Tabs: []Tab{
			{Label: "Mine", Layers: mine, Cursor: 0},
			{Label: "Review", Layers: review, Cursor: 0},
			{Label: "All", Layers: all, Cursor: 0},
		},
	}

	// Move cursor down twice in Mine.
	for i := 0; i < 2; i++ {
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = next.(Model)
	}
	if m.Cursor != 2 {
		t.Fatalf("Mine cursor = %d, want 2", m.Cursor)
	}

	// Tab → Review. Cursor resets to that tab's stored value (0).
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = next.(Model)
	if m.ActiveTab != 1 {
		t.Errorf("ActiveTab = %d, want 1", m.ActiveTab)
	}
	if m.Cursor != 0 {
		t.Errorf("Review cursor = %d, want 0", m.Cursor)
	}
	if len(m.Layers) != 1 {
		t.Errorf("active layers len = %d, want 1", len(m.Layers))
	}

	// Press '3' → jump to All.
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	m = next.(Model)
	if m.ActiveTab != 2 {
		t.Errorf("ActiveTab = %d, want 2", m.ActiveTab)
	}

	// Press '1' → back to Mine. Cursor should be restored to 2.
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	m = next.(Model)
	if m.ActiveTab != 0 {
		t.Errorf("ActiveTab = %d, want 0", m.ActiveTab)
	}
	if m.Cursor != 2 {
		t.Errorf("restored Mine cursor = %d, want 2", m.Cursor)
	}
}

func TestRenameFlow_NoPRNumberDisablesRename(t *testing.T) {
	m := Model{
		Layers:  []Layer{{Branch: "feat/x"}}, // no PR
		Updater: func(_ context.Context, _ int, _ string) error { return nil },
	}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if next.(Model).editingTitle {
		t.Fatal("rename should be disabled when row has no PR")
	}
}

// stripANSI removes ANSI escape sequences so assertions don't depend on
// lipgloss styling.
func stripANSI(s string) string {
	var b strings.Builder
	in := false
	for _, r := range s {
		if r == 0x1b {
			in = true
			continue
		}
		if in {
			if r == 'm' {
				in = false
			}
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
