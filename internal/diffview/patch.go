// Package diffview renders unified diffs from `gh pr diff` in a TUI.
package diffview

import (
	"strings"
)

// File is one file's worth of changes from a unified diff.
type File struct {
	Path  string
	Adds  int
	Dels  int
	Lines []Line
}

// Line is one rendered line of the diff (file header, hunk header, or
// content). Kind drives coloring; Text is the raw text without trailing
// newline.
type Line struct {
	Kind LineKind
	Text string
}

type LineKind int

const (
	LineContext  LineKind = iota // " foo"
	LineAdd                      // "+foo"
	LineDel                      // "-foo"
	LineFile                     // "diff --git ..." / "--- a/" / "+++ b/"
	LineHunk                     // "@@ -1,3 +1,4 @@"
	LineNoNewLine                // "\ No newline at end of file"
)

// Parse splits a unified diff (as produced by `gh pr diff --patch` or
// `git diff`) into per-file blocks. Adds/Dels are counted only for content
// lines (not file/hunk headers). The parser is intentionally lenient: any
// content that doesn't match an expected prefix is treated as context.
func Parse(diff string) []File {
	var files []File
	var cur *File

	flush := func() {
		if cur != nil {
			files = append(files, *cur)
			cur = nil
		}
	}

	for _, raw := range strings.Split(diff, "\n") {
		if strings.HasPrefix(raw, "diff --git ") {
			flush()
			cur = &File{Path: pathFromDiffHeader(raw), Lines: []Line{{Kind: LineFile, Text: raw}}}
			continue
		}
		if cur == nil {
			// Skip preamble before the first "diff --git".
			continue
		}
		switch {
		case strings.HasPrefix(raw, "--- ") || strings.HasPrefix(raw, "+++ ") ||
			strings.HasPrefix(raw, "index ") || strings.HasPrefix(raw, "new file mode") ||
			strings.HasPrefix(raw, "deleted file mode") || strings.HasPrefix(raw, "rename ") ||
			strings.HasPrefix(raw, "similarity index"):
			cur.Lines = append(cur.Lines, Line{Kind: LineFile, Text: raw})
			// If the +++ line has a better path (e.g. for renames), prefer it.
			if strings.HasPrefix(raw, "+++ b/") && cur.Path == "" {
				cur.Path = strings.TrimPrefix(raw, "+++ b/")
			}
		case strings.HasPrefix(raw, "@@"):
			cur.Lines = append(cur.Lines, Line{Kind: LineHunk, Text: raw})
		case strings.HasPrefix(raw, "+"):
			cur.Adds++
			cur.Lines = append(cur.Lines, Line{Kind: LineAdd, Text: raw})
		case strings.HasPrefix(raw, "-"):
			cur.Dels++
			cur.Lines = append(cur.Lines, Line{Kind: LineDel, Text: raw})
		case strings.HasPrefix(raw, `\ `):
			cur.Lines = append(cur.Lines, Line{Kind: LineNoNewLine, Text: raw})
		default:
			cur.Lines = append(cur.Lines, Line{Kind: LineContext, Text: raw})
		}
	}
	flush()
	return files
}

// pathFromDiffHeader extracts the b/ path from a "diff --git a/foo b/foo"
// header. Falls back to the a/ path if b/ isn't present (deletions).
func pathFromDiffHeader(line string) string {
	// "diff --git a/path b/path"
	parts := strings.Fields(line)
	for _, p := range parts {
		if strings.HasPrefix(p, "b/") {
			return strings.TrimPrefix(p, "b/")
		}
	}
	for _, p := range parts {
		if strings.HasPrefix(p, "a/") {
			return strings.TrimPrefix(p, "a/")
		}
	}
	return ""
}
