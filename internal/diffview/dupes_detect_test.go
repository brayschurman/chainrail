package diffview

import "testing"

func TestDetectNewSymbols_Go(t *testing.T) {
	lines := []Line{
		{Kind: LineAdd, Text: "+func FormatDate(t time.Time) string {"},
		{Kind: LineAdd, Text: "+type WallShape struct {"},
		{Kind: LineAdd, Text: "+const someConst = 1"}, // lowercase → not exported
		{Kind: LineAdd, Text: "+var ExportedVar = true"},
		{Kind: LineAdd, Text: "+    return s"}, // not a top-level decl
		{Kind: LineDel, Text: "-func OldName() {}"},
	}
	got := DetectNewSymbols("foo.go", lines)
	want := []string{"FormatDate", "WallShape", "ExportedVar"}
	assertSymbols(t, got, want)
}

func TestDetectNewSymbols_TypeScript(t *testing.T) {
	lines := []Line{
		{Kind: LineAdd, Text: "+export function formatDate(d: Date): string {"},
		// 2-char names like "PI" are filtered as too-short — grep would be noisy.
		{Kind: LineAdd, Text: "+export const ColorPI = 3.14"},
		{Kind: LineAdd, Text: "+export class Wall {"},
		{Kind: LineAdd, Text: "+export interface Point {"},
		{Kind: LineAdd, Text: "+export type Snap = (p: Point) => Point"},
		{Kind: LineAdd, Text: "+export default function App() {"},
		{Kind: LineAdd, Text: "+const internal = 1"}, // not exported
	}
	got := DetectNewSymbols("foo.ts", lines)
	want := []string{"formatDate", "ColorPI", "Wall", "Point", "Snap", "App"}
	assertSymbols(t, got, want)
}

func TestDetectNewSymbols_Python(t *testing.T) {
	lines := []Line{
		{Kind: LineAdd, Text: "+def format_date(t):"},
		{Kind: LineAdd, Text: "+class WallShape:"},
		{Kind: LineAdd, Text: "+    helper = 1"}, // not a top-level def
		{Kind: LineAdd, Text: "+def _private():"}, // underscore prefix ok per regex
	}
	got := DetectNewSymbols("foo.py", lines)
	want := []string{"format_date", "WallShape", "_private"}
	assertSymbols(t, got, want)
}

func TestDetectNewSymbols_UnknownExtension(t *testing.T) {
	lines := []Line{
		{Kind: LineAdd, Text: "+func main() {}"},
	}
	got := DetectNewSymbols("foo.zzzz", lines)
	if got != nil {
		t.Errorf("unknown ext should return nil, got %v", got)
	}
}

func TestDetectNewSymbols_TooShortFiltered(t *testing.T) {
	// Single/double-char identifiers tend to be noisy false positives; we
	// require 3+ chars to make grep meaningful.
	lines := []Line{
		{Kind: LineAdd, Text: "+func A() {}"},
		{Kind: LineAdd, Text: "+func XY() {}"},
		{Kind: LineAdd, Text: "+func ABC() {}"},
	}
	got := DetectNewSymbols("foo.go", lines)
	want := []string{"ABC"}
	assertSymbols(t, got, want)
}

func assertSymbols(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d] got %q, want %q", i, got[i], want[i])
		}
	}
}
