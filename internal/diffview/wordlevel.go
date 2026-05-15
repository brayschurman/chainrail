package diffview

import (
	"strings"

	"github.com/sergi/go-diff/diffmatchpatch"
)

// wordSpan is one contiguous run within a content line. Kind says whether
// this run is shared with the paired line (Equal), specific to this line
// (Changed), or only present on one side of the pair (Inserted/Deleted —
// rendered the same way as Changed for our purposes).
type wordSpan struct {
	Kind wordSpanKind
	Text string
}

type wordSpanKind int

const (
	wordEqual   wordSpanKind = iota // unchanged across the pair
	wordChanged                      // present only on this side
)

// pairWordSpans takes the bodies of a paired LineDel + LineAdd (markers
// already stripped) and returns the per-side spans showing which substrings
// changed. Uses diff-match-patch's character-level diff with a "diff to lines
// by word boundary" preprocessing step to keep spans aligned to identifiers
// and tokens rather than individual characters.
//
// Returns nil, nil when the inputs are identical or empty — caller should
// fall back to the full-line tint in those cases.
func pairWordSpans(oldBody, newBody string) (oldSpans, newSpans []wordSpan) {
	if oldBody == newBody || oldBody == "" || newBody == "" {
		return nil, nil
	}
	dmp := diffmatchpatch.New()
	// Convert to per-word tokens to keep span boundaries readable.
	a, b, words := dmp.DiffLinesToRunes(splitForWordDiff(oldBody), splitForWordDiff(newBody))
	diffs := dmp.DiffMainRunes(a, b, false)
	diffs = dmp.DiffCharsToLines(diffs, words)
	diffs = dmp.DiffCleanupSemantic(diffs)

	for _, d := range diffs {
		// Strip our internal '\n' token separators — neither source line
		// contained newlines, so any \n in d.Text came from splitForWordDiff.
		text := strings.ReplaceAll(d.Text, "\n", "")
		switch d.Type {
		case diffmatchpatch.DiffEqual:
			if text != "" {
				oldSpans = append(oldSpans, wordSpan{Kind: wordEqual, Text: text})
				newSpans = append(newSpans, wordSpan{Kind: wordEqual, Text: text})
			}
		case diffmatchpatch.DiffDelete:
			if text != "" {
				oldSpans = append(oldSpans, wordSpan{Kind: wordChanged, Text: text})
			}
		case diffmatchpatch.DiffInsert:
			if text != "" {
				newSpans = append(newSpans, wordSpan{Kind: wordChanged, Text: text})
			}
		}
	}
	// If everything came out equal (rare given our identical guard above) or
	// one side is entirely empty in spans, signal "no useful intra-line"
	// pairing — caller will fall back.
	if !hasAnyChanged(oldSpans) && !hasAnyChanged(newSpans) {
		return nil, nil
	}
	return oldSpans, newSpans
}

// splitForWordDiff returns the input as a single string per "token" (a word,
// punctuation chunk, or whitespace run). diff-match-patch's
// DiffLinesToRunes treats each entry as an atomic unit, so this is how we
// get word-level boundaries from a char-level algorithm.
//
// Tokens preserve their whitespace and punctuation so reassembly is exact.
func splitForWordDiff(s string) string {
	// Wrap every token (word or non-word run) with \n so DiffLinesToRunes
	// treats it as an atomic line. Add a trailing \n so the last token is
	// included.
	var out []byte
	out = make([]byte, 0, len(s)+len(s)/4)
	type kind int
	const (
		kindWord kind = iota
		kindOther
	)
	classify := func(b byte) kind {
		switch {
		case (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_':
			return kindWord
		}
		return kindOther
	}
	if len(s) == 0 {
		return ""
	}
	start := 0
	cur := classify(s[0])
	for i := 1; i < len(s); i++ {
		k := classify(s[i])
		if k != cur {
			out = append(out, s[start:i]...)
			out = append(out, '\n')
			start = i
			cur = k
		}
	}
	out = append(out, s[start:]...)
	out = append(out, '\n')
	return string(out)
}

func hasAnyChanged(spans []wordSpan) bool {
	for _, s := range spans {
		if s.Kind == wordChanged {
			return true
		}
	}
	return false
}
