package diffview

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/brayschurman/chainrail/internal/reviewstate"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// (context is used by future detectors; the import keeps room for them
// without churning later commits.)
var _ = context.Background

// Model is the bubbletea model for the PR diff viewer.
type Model struct {
	Title       string // e.g. "#687 wall drawing UX"
	Files       []File
	width       int
	height      int
	highlighter *Highlighter

	// BlobByPath maps file path -> blob SHA for the PR's current head, used
	// to detect "changed since you checked" against persisted review state.
	BlobByPath map[string]string

	// CISignals records the per-file CI risk classification computed once
	// at load time. The signal still drives the 100%-progress hard-block
	// (a typed waiver via shift+W is required when CI files are unreviewed),
	// but is no longer surfaced as a sidebar marker — that was too noisy
	// without an AI-based discriminator. Kept available for a future
	// `cn lint-pr` CI-mode subcommand.
	CISignals map[string]CISignal

	// PRBody is the PR description text — used by the plan detector.
	PRBody string
	// PlanSignal is the verdict from DetectPlan on PRBody.
	PlanSignal PlanSignal
	// PRNumber for posting the nudge comment.
	PRNumber int
	// PlanNudger sends the nudge comment when the reviewer presses P.
	// Optional; nil disables the nudge.
	PlanNudger func(number int, body string) error

	// displayOrder is the index permutation that re-orders Files so CI-
	// touching files come first. Computed once, used by all sidebar render
	// paths. Empty until ensureDisplayOrder runs.
	displayOrder []int

	// waiverInput is shown when the user presses shift+W on a CI-risk file
	// that the 100% gate is blocking on. Pattern mirrors the rename input.
	waiverInput   textinput.Model
	enteringWaiver bool

	// ReviewState is the loaded per-file checklist for this PR. Optional;
	// when nil, review-tracking UI is hidden entirely. The owner+repo+number
	// fields are kept here so Save can write back to the right key.
	ReviewState *reviewstate.PRState
	ReviewStore *reviewstate.Store
	RepoOwner   string
	RepoName    string

	cursor   int // file index in sidebar
	scrollY  int // line index at top of diff pane
	quitting bool

	// Render cache: scrolling and tabbing are hot paths, so we pre-render the
	// expensive (chroma + lipgloss) work per file/width and reuse it on every
	// frame. Invalidated when cursor or width changes.
	rc renderCache
	// Content-hash LRU cache so identical lines across files (a dep bump in
	// 12 package.json files, etc.) only get rendered once per width.
	lc *lineCache
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
	bgAddGut    = lipgloss.Color("#0e2418") // gutter on + lines (slightly darker than bgAdd)
	bgDelGut    = lipgloss.Color("#2e1414") // gutter on - lines (slightly darker than bgDel)
	bgAddBright = lipgloss.Color("#1f6b3a") // saturated green for the *changed* word spans
	bgDelBright = lipgloss.Color("#7a2a2a") // saturated red for the *changed* word spans
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
	styleSidebarSel  = lipgloss.NewStyle().Background(bgSel).Foreground(colorBright).Bold(true)
	styleSidebarDone = lipgloss.NewStyle().Background(bgPane).Foreground(lipgloss.Color("241")).Faint(true)
	styleProgressOn  = lipgloss.NewStyle().Background(bgChrome).Foreground(lipgloss.Color("82")) // bright green
	styleProgressOff = lipgloss.NewStyle().Background(bgChrome).Foreground(lipgloss.Color("238"))
	styleCIBlock     = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	styleCIWarn      = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	styleKey      = lipgloss.NewStyle().Background(bgChrome).Foreground(colorPink).Bold(true)
	styleKeyDim   = lipgloss.NewStyle().Background(bgChrome).Foreground(colorFaint)
	styleGutter   = lipgloss.NewStyle().Background(bgGutter).Foreground(colorFaint)
	styleAddGut       = lipgloss.NewStyle().Background(bgAddGut).Foreground(colorFaint)
	styleDelGut       = lipgloss.NewStyle().Background(bgDelGut).Foreground(colorFaint)
	styleAddBrightSpan = lipgloss.NewStyle().Background(bgAddBright).Foreground(colorBright).Bold(true)
	styleDelBrightSpan = lipgloss.NewStyle().Background(bgDelBright).Foreground(colorBright).Bold(true)
)

// rowKind tells styledRowSpans which palette to use.
type rowKind int

const (
	kindAdd rowKind = iota
	kindDel
)

const (
	defaultWidth  = 160
	defaultHeight = 48
)

func New(title string, files []File) Model {
	m := Model{
		Title:       title,
		Files:       files,
		width:       defaultWidth,
		height:      defaultHeight,
		highlighter: NewHighlighter(),
		lc:          newLineCache(4096),
		CISignals:   make(map[string]CISignal, len(files)),
	}
	m.runDetectors()
	m.buildDisplayOrder()
	return m
}

// runDetectors invokes every available file-level detector once for each
// file in the PR. Results live on the Model so the render path is allocation-
// free per frame.
func (m *Model) runDetectors() {
	m.PlanSignal = DetectPlan(m.PRBody)
	for _, f := range m.Files {
		if sig := DetectCIRisk(f.Path, f.Lines); sig.Risk > CIRiskNone {
			m.CISignals[f.Path] = sig
		}
	}
}


// buildDisplayOrder is currently a passthrough — files render in the order
// the diff parser produced them. Earlier versions reordered by severity
// markers but that turned out to be more noise than signal. The function is
// kept so future re-orderings (e.g. tree view) have a single seam.
func (m *Model) buildDisplayOrder() {
	order := make([]int, len(m.Files))
	for i := range m.Files {
		order[i] = i
	}
	m.displayOrder = order
}

// fileAt returns the File at the visual sidebar position vis.
func (m Model) fileAt(vis int) File {
	if len(m.displayOrder) > 0 {
		return m.Files[m.displayOrder[vis]]
	}
	return m.Files[vis]
}

// hasUnwaivedCIBlockers reports whether any CI-touching or prompt-injection
// file is unreviewed AND unwaived. Used to gate 100% progress.
func (m Model) hasUnwaivedCIBlockers() bool {
	if m.ReviewState == nil {
		return false
	}
	for _, f := range m.Files {
		if !m.isBlockerFile(f.Path) {
			continue
		}
		if mark, ok := m.ReviewState.Files[f.Path]; ok {
			if mark.Waiver != "" {
				continue
			}
			if !m.ReviewState.ChangedSince(f.Path, m.BlobByPath[f.Path]) {
				continue
			}
		}
		return true
	}
	return false
}

// isBlockerFile reports whether a file carries a detection severe enough to
// block 100% progress without a typed waiver. Currently only CI-touching
// files; the criterion is intentionally narrow so the gate stays meaningful.
func (m Model) isBlockerFile(path string) bool {
	return m.CISignals[path].Risk > CIRiskNone
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.enteringWaiver {
		if key, ok := msg.(tea.KeyMsg); ok {
			return m.updateWaiver(key)
		}
	}
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
		case "x":
			m.toggleReviewed()
		case "N":
			m.jumpToNextUnreviewed()
		case "W":
			return m.startWaiver()
		case "P":
			return m, m.nudgeForPlan()
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
		f := m.fileAt(m.cursor)
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

	var foot string
	if mp.enteringWaiver {
		foot = styleChrome.Width(m.width).Render(" waiver: " + mp.waiverInput.View())
	} else {
		foot = mp.renderKeybindings(m.width)
	}

	return header + "\n" + body + "\n" + foot
}

// ---------------------------------------------------------------------------
// Top bar
// ---------------------------------------------------------------------------

func (m Model) renderTopBar() string {
	left := " " + styleTitle.Render("chainrail") + "  " + styleHeading.Render(m.Title)

	// Plan-presence indicator (small, before the progress bar)
	planBadge := m.planBadge()
	if planBadge != "" {
		left += "  " + planBadge
	}

	if m.ReviewState == nil {
		return styleChrome.Width(m.width).Render(left)
	}

	done, total := m.reviewedCount()
	if total == 0 {
		return styleChrome.Width(m.width).Render(left)
	}
	pct := 0
	if total > 0 {
		pct = (done * 100) / total
	}
	// Hard-block: even if every file is marked, refuse to surface 100% while
	// CI-risk files are unwaived. The reviewer has to make an explicit
	// waiver decision via shift+W.
	if pct >= 100 && m.hasUnwaivedCIBlockers() {
		pct = 99
	}
	bar := progressBar(done, total, 12)
	elapsed := m.reviewElapsed()
	right := bar + " " + styleChrome.Render(fmt.Sprintf("%d/%d (%d%%)", done, total, pct))
	if elapsed != "" {
		right += styleKeyDim.Render("  · " + elapsed)
	}

	// Pad left+right to fill width.
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right) - 1
	if gap < 1 {
		gap = 1
	}
	return styleChrome.Render(left) + styleChrome.Render(strings.Repeat(" ", gap)) + right + styleChrome.Render(" ")
}

// renderPreReviewPanel produces the chip-style aggregator that sits between
// planBadge renders a one-chip indicator for plan presence. Empty when PR
// body is unknown (PlanSignal zero-value with Chars=0 happens both for "no
// body" and "we never set PRBody"; the unset case is treated as silence).
func (m Model) planBadge() string {
	switch m.PlanSignal.Severity {
	case PlanPresent:
		return styleChrome.Foreground(lipgloss.Color("82")).Render("✓ plan")
	case PlanThin:
		return styleChrome.Foreground(lipgloss.Color("214")).Render("⚠ thin plan")
	case PlanMissing:
		if m.PRBody == "" && len(m.Files) == 0 {
			return ""
		}
		// Show the P keystroke hint when a nudge hasn't been sent yet.
		hint := ""
		if m.ReviewState != nil && m.ReviewState.NudgedForPlanAt.IsZero() && m.PlanNudger != nil {
			hint = styleChrome.Foreground(lipgloss.Color("245")).Render("  [P nudge]")
		}
		return styleChrome.Foreground(lipgloss.Color("196")).Render("🚨 no plan") + hint
	}
	return ""
}

// progressBar renders a small ▰▰▰▱▱▱ bar of `width` cells.
func progressBar(done, total, width int) string {
	if total <= 0 || width <= 0 {
		return ""
	}
	filled := (done * width) / total
	if done > 0 && filled == 0 {
		filled = 1
	}
	if filled > width {
		filled = width
	}
	on := styleProgressOn.Render(strings.Repeat("▰", filled))
	off := styleProgressOff.Render(strings.Repeat("▱", width-filled))
	return on + off
}

// reviewElapsed returns "Nh Mm elapsed" since FirstCheckedAt, or "" if no
// progress has started yet.
func (m Model) reviewElapsed() string {
	if m.ReviewState == nil || m.ReviewState.FirstCheckedAt.IsZero() {
		return ""
	}
	d := time.Since(m.ReviewState.FirstCheckedAt)
	if d < time.Minute {
		return "just started"
	}
	h := int(d / time.Hour)
	mins := int(d/time.Minute) % 60
	if h == 0 {
		return fmt.Sprintf("%dm elapsed", mins)
	}
	return fmt.Sprintf("%dh %dm elapsed", h, mins)
}

// ---------------------------------------------------------------------------
// Sidebar
// ---------------------------------------------------------------------------

// buildSidebarRows pre-renders one styled string per file plus a header. The
// rows are reused frame-to-frame and only rebuilt when width or cursor moves.
//
// Layout per row: "▸ [✓] path  +A -D  Δ?". Reviewed files render faded.
func (m Model) buildSidebarRows(w int) []string {
	if w <= 0 {
		return nil
	}
	rows := make([]string, 0, len(m.Files))

	// Reserve: " " (1) + selection marker (1) + " " (1) + checkbox "[✓]" (3)
	// + " " (1) + name + " " + "+ddd -ddd" (10).
	const countsW = 11
	const fixedW = 1 + 1 + 1 + 3 + 1 + 1 + countsW
	nameW := w - fixedW
	if nameW < 6 {
		nameW = 6
	}

	for i := 0; i < len(m.Files); i++ {
		f := m.fileAt(i)
		name := truncatePath(f.Path, nameW)
		counts := fmt.Sprintf("+%-3d -%-3d", f.Adds, f.Dels)
		box := m.checkboxFor(f.Path)
		row := fmt.Sprintf(" %s %s %-*s %s",
			selectionMarker(i == m.cursor),
			box,
			nameW, name,
			counts,
		)

		// Tint by state.
		switch {
		case i == m.cursor:
			rows = append(rows, styleSidebarSel.Width(w).Render(row))
		case m.isReviewed(f.Path):
			rows = append(rows, styleSidebarDone.Width(w).Render(row))
		default:
			rows = append(rows, styleFaintBg.Width(w).Render(row))
		}
	}
	return rows
}


// checkboxFor returns the 3-char box marker for a file. "[✓]" reviewed,
// "[ ]" not. "[~]" reviewed-but-changed-since (stale).
func (m Model) checkboxFor(path string) string {
	if m.ReviewState == nil {
		return "[ ]"
	}
	if !m.ReviewState.IsChecked(path) {
		return "[ ]"
	}
	if m.ReviewState.ChangedSince(path, m.BlobByPath[path]) {
		return "[~]"
	}
	return "[✓]"
}

func (m Model) isReviewed(path string) bool {
	if m.ReviewState == nil {
		return false
	}
	return m.ReviewState.IsChecked(path) && !m.ReviewState.ChangedSince(path, m.BlobByPath[path])
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

// toggleReviewed flips the reviewed state for the current sidebar file.
// Persists immediately so a crash or quit doesn't lose progress.
func (m *Model) toggleReviewed() {
	if m.ReviewState == nil || m.cursor >= len(m.Files) {
		return
	}
	f := m.fileAt(m.cursor)
	blob := m.BlobByPath[f.Path]
	m.ReviewState.Toggle(f.Path, blob, time.Now())
	if m.ReviewStore != nil {
		_ = m.ReviewStore.Save(m.RepoOwner, m.RepoName, m.ReviewState)
	}
	// Invalidate sidebar cache so the checkmark redraws on the next frame.
	m.rc.sideRows = nil
}

// nudgeMsg comes back when the plan-nudge comment finishes posting.
type nudgeMsg struct {
	err error
}

// nudgeForPlan posts the templated "request a plan" comment to the PR. If a
// nudge has already been recorded in the review state, this is a no-op so we
// don't double-prompt the author.
func (m *Model) nudgeForPlan() tea.Cmd {
	if m.PlanNudger == nil || m.PRNumber == 0 {
		return nil
	}
	if m.ReviewState != nil && !m.ReviewState.NudgedForPlanAt.IsZero() {
		return nil
	}
	nudger := m.PlanNudger
	num := m.PRNumber
	body := PlanNudgeMessage
	// Mark the nudge BEFORE the network call so a slow/failed request still
	// updates state — the reviewer can manually retry if needed.
	if m.ReviewState != nil {
		m.ReviewState.NudgedForPlanAt = time.Now()
		if m.ReviewStore != nil {
			_ = m.ReviewStore.Save(m.RepoOwner, m.RepoName, m.ReviewState)
		}
	}
	return func() tea.Msg {
		err := nudger(num, body)
		return nudgeMsg{err: err}
	}
}

// startWaiver opens the waiver textinput on a CI-risk file. No-op if the
// current row isn't CI-touching.
func (m Model) startWaiver() (tea.Model, tea.Cmd) {
	if m.cursor >= len(m.Files) {
		return m, nil
	}
	f := m.fileAt(m.cursor)
	if !m.isBlockerFile(f.Path) {
		return m, nil
	}
	ti := textinput.New()
	ti.Placeholder = "reason for waiving CI-risk check"
	ti.Focus()
	ti.CharLimit = 200
	ti.Width = 60
	m.waiverInput = ti
	m.enteringWaiver = true
	return m, textinput.Blink
}

func (m Model) updateWaiver(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "enter":
		reason := strings.TrimSpace(m.waiverInput.Value())
		m.enteringWaiver = false
		if reason == "" {
			return m, nil
		}
		f := m.fileAt(m.cursor)
		blob := m.BlobByPath[f.Path]
		if m.ReviewState != nil {
			m.ReviewState.Set(f.Path, blob, reason, time.Now())
			if m.ReviewStore != nil {
				_ = m.ReviewStore.Save(m.RepoOwner, m.RepoName, m.ReviewState)
			}
			m.rc.sideRows = nil
		}
		return m, nil
	case "esc":
		m.enteringWaiver = false
		return m, nil
	}
	var cmd tea.Cmd
	m.waiverInput, cmd = m.waiverInput.Update(key)
	return m, cmd
}

// jumpToNextUnreviewed advances the cursor to the next file that hasn't
// been marked reviewed. Wraps to the top.
func (m *Model) jumpToNextUnreviewed() {
	if m.ReviewState == nil || len(m.Files) == 0 {
		return
	}
	n := len(m.Files)
	for i := 1; i <= n; i++ {
		idx := (m.cursor + i) % n
		if !m.ReviewState.IsChecked(m.fileAt(idx).Path) {
			m.cursor = idx
			m.scrollY = 0
			m.rc.fileIdx = -1 // force cache rebuild
			return
		}
	}
}

// reviewedCount returns how many of the PR's files have been marked done.
func (m Model) reviewedCount() (done, total int) {
	if m.ReviewState == nil {
		return 0, len(m.Files)
	}
	for _, f := range m.Files {
		if m.ReviewState.IsChecked(f.Path) {
			done++
		}
	}
	return done, len(m.Files)
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
	f := m.fileAt(m.cursor)
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
//
// When a LineDel is immediately followed by one or more LineAdds, the two
// sides are run through word-level diff so the changed characters inside each
// line stand out against the muted line-level tint.
func (m Model) renderFileLines(f File, w int) []string {
	out := make([]string, 0, len(f.Lines))
	var oldNo, newNo int
	for i := 0; i < len(f.Lines); i++ {
		l := f.Lines[i]
		switch l.Kind {
		case LineFile:
			continue
		case LineHunk:
			oldNo, newNo = parseHunkStart(l.Text)
			out = append(out, styleHunkLine.Width(w).Render(" "+l.Text))
		case LineDel:
			// Look ahead — if the very next line is an Add, pair them and
			// render both with word-level highlighting. Word-paired rows are
			// not cached (gutter+span composition makes the cache key fragile);
			// the per-line cache below still covers unpaired ± lines.
			if i+1 < len(f.Lines) && f.Lines[i+1].Kind == LineAdd {
				addLine := f.Lines[i+1]
				oldBody := stripMarker(l.Text)
				newBody := stripMarker(addLine.Text)
				oldSpans, newSpans := pairWordSpans(oldBody, newBody)
				if oldSpans != nil {
					out = append(out,
						m.styledRowSpans(oldSpans, f.Path, oldNo, 0, w, kindDel),
						m.styledRowSpans(newSpans, f.Path, 0, newNo, w, kindAdd),
					)
					oldNo++
					newNo++
					i++
					continue
				}
			}
			out = append(out, m.styledRow(l, f.Path, oldNo, 0, w))
			oldNo++
		case LineAdd:
			out = append(out, m.styledRow(l, f.Path, 0, newNo, w))
			newNo++
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
//
// Chroma highlights each token with both its syntax fg AND the diff bg, so
// the syntax coloring stays vivid on top of the tinted row.
func (m Model) styledRow(l Line, path string, oldNo, newNo, w int) string {
	body := stripMarker(l.Text)
	gutterText := fmt.Sprintf("%s %s ", numCol(oldNo), numCol(newNo))

	// Cache check — line numbers are part of the key only via the gutter
	// text we prepend, so we hash the body separately and re-attach gutter.
	ext := pathExt(path)
	bodyKey := lineKey(l.Kind, ext, w, body)
	if m.lc != nil {
		if cached, ok := m.lc.Get(bodyKey); ok {
			return prependGutter(cached, gutterText, l.Kind)
		}
	}

	var gutterStyle, lineStyle, markerStyle lipgloss.Style
	var bg lipgloss.Color
	var marker string

	switch l.Kind {
	case LineAdd:
		gutterStyle = styleAddGut
		lineStyle = styleAddLine
		markerStyle = styleAddMark
		bg = bgAdd
		marker = "+"
	case LineDel:
		gutterStyle = styleDelGut
		lineStyle = styleDelLine
		markerStyle = styleDelMark
		bg = bgDel
		marker = "-"
	default:
		gutterStyle = styleGutter
		lineStyle = stylePaneBg
		markerStyle = styleGutter
		bg = bgPane
		marker = " "
	}

	gutter := gutterStyle.Render(gutterText)
	mark := markerStyle.Render(marker + " ")
	contentW := w - lipgloss.Width(gutter) - lipgloss.Width(mark)
	if contentW < 1 {
		contentW = 1
	}

	// Token-level highlighting: chroma + bg composed per span, not layered
	// after the fact. Pad the right side with bg-tinted spaces to fill width.
	highlighted := m.highlighter.HighlightWithBg(path, body, bg)
	bodyW := lipgloss.Width(highlighted)
	padN := contentW - bodyW
	if padN < 0 {
		padN = 0
		// Body is wider than pane — defer to lipgloss.Width to truncate via
		// Render's Width clamp. (Long-line handling is its own ticket.)
	}
	pad := lineStyle.Render(strings.Repeat(" ", padN))
	bodySegment := mark + highlighted + pad
	if m.lc != nil {
		m.lc.Put(bodyKey, bodySegment)
	}
	return gutter + bodySegment
}

// prependGutter rebuilds the leading gutter for a cached body segment so we
// can show different line numbers without re-rendering the body itself.
func prependGutter(bodySegment, gutterText string, kind LineKind) string {
	var st lipgloss.Style
	switch kind {
	case LineAdd:
		st = styleAddGut
	case LineDel:
		st = styleDelGut
	default:
		st = styleGutter
	}
	return st.Render(gutterText) + bodySegment
}

// pathExt returns the lowercased file extension used as part of the cache
// key. Two files sharing the same extension share a lexer and therefore
// produce identical renders for identical line content.
func pathExt(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '.' {
			return strings.ToLower(path[i:])
		}
		if path[i] == '/' {
			break
		}
	}
	return ""
}

// styledRowSpans renders a paired +/- line using word-level spans. Unchanged
// spans get the muted line tint; spans flagged as changed get the bright
// span style. Syntax highlighting is intentionally skipped for spans rows —
// chroma's ANSI codes would clash with span-level backgrounds, and word-level
// diff is most useful on lines where the *change* matters more than the
// language coloring (config files, dep lists, paths).
func (m Model) styledRowSpans(spans []wordSpan, path string, oldNo, newNo, w int, kind rowKind) string {
	gutterText := fmt.Sprintf("%s %s ", numCol(oldNo), numCol(newNo))

	var gutterStyle, lineStyle, markerStyle, brightStyle lipgloss.Style
	var marker string
	switch kind {
	case kindAdd:
		gutterStyle = styleAddGut
		lineStyle = styleAddLine
		markerStyle = styleAddMark
		brightStyle = styleAddBrightSpan
		marker = "+"
	case kindDel:
		gutterStyle = styleDelGut
		lineStyle = styleDelLine
		markerStyle = styleDelMark
		brightStyle = styleDelBrightSpan
		marker = "-"
	}

	gutter := gutterStyle.Render(gutterText)
	mark := markerStyle.Render(marker + " ")

	// Render the spans: equal spans use the line tint (lineStyle as the
	// surrounding background), changed spans get the brighter span style.
	var body strings.Builder
	for _, s := range spans {
		if s.Kind == wordChanged {
			body.WriteString(brightStyle.Render(s.Text))
		} else {
			body.WriteString(lineStyle.Render(s.Text))
		}
	}

	contentW := w - lipgloss.Width(gutter) - lipgloss.Width(mark)
	if contentW < 1 {
		contentW = 1
	}
	// Pad the right with the muted line color so the row fills the pane width.
	bodyText := body.String()
	bodyW := lipgloss.Width(bodyText)
	padN := contentW - bodyW
	if padN < 0 {
		padN = 0
	}
	pad := lineStyle.Render(strings.Repeat(" ", padN))
	return gutter + mark + bodyText + pad
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
	}
	if m.ReviewState != nil {
		parts = append(parts,
			styleKey.Render("x") + styleKeyDim.Render(" reviewed"),
			styleKey.Render("N") + styleKeyDim.Render(" next unreviewed"),
		)
	}
	parts = append(parts,
		styleKey.Render("g/G") + styleKeyDim.Render(" top/bottom"),
		styleKey.Render("q") + styleKeyDim.Render(" quit"),
	)
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
	n := len(m.fileAt(m.cursor).Lines) - m.diffContentHeight()
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
