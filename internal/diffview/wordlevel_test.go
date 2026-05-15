package diffview

import (
	"strings"
	"testing"
)

func TestPairWordSpans_IdenticalReturnsNil(t *testing.T) {
	o, n := pairWordSpans("foo bar", "foo bar")
	if o != nil || n != nil {
		t.Errorf("identical input should return nil spans, got old=%v new=%v", o, n)
	}
}

func TestPairWordSpans_SimpleReplace(t *testing.T) {
	old := "const start = points[0]"
	new := "const start = snapTo(points[0])"
	oldSpans, newSpans := pairWordSpans(old, new)
	if oldSpans == nil || newSpans == nil {
		t.Fatalf("expected non-nil spans for changed line; old=%v new=%v", oldSpans, newSpans)
	}
	// Reconstruct each side and check we got the original strings back.
	if got := reconstruct(oldSpans); got != old {
		t.Errorf("old reconstruction = %q, want %q", got, old)
	}
	if got := reconstruct(newSpans); got != new {
		t.Errorf("new reconstruction = %q, want %q", got, new)
	}
	// "const start = " should be in the equal spans on both sides.
	if !hasEqualContaining(oldSpans, "const") || !hasEqualContaining(newSpans, "const") {
		t.Errorf("expected 'const' to be equal on both sides")
	}
}

func TestPairWordSpans_DependencyListChange(t *testing.T) {
	// The exact case from the cn view screenshot — only one token differs.
	old := "--exclude=@repo/infra,@repo/db-old,@repo/floorplan-studio"
	new := "--exclude=@repo/infra,@repo/floorplan-studio"
	oldSpans, newSpans := pairWordSpans(old, new)
	if oldSpans == nil {
		t.Fatal("expected non-nil spans")
	}
	if got := reconstruct(oldSpans); got != old {
		t.Errorf("old reconstruction = %q, want %q", got, old)
	}
	if got := reconstruct(newSpans); got != new {
		t.Errorf("new reconstruction = %q, want %q", got, new)
	}
	// db-old token should be a changed span on the old side.
	if !hasChangedContaining(oldSpans, "db") {
		t.Errorf("expected 'db' to be in a Changed span on the old side; got %+v", oldSpans)
	}
}

func TestPairWordSpans_EmptyOnEitherSideReturnsNil(t *testing.T) {
	o, n := pairWordSpans("", "foo")
	if o != nil || n != nil {
		t.Errorf("empty old should return nil, got old=%v new=%v", o, n)
	}
	o, n = pairWordSpans("foo", "")
	if o != nil || n != nil {
		t.Errorf("empty new should return nil, got old=%v new=%v", o, n)
	}
}

// helpers

func reconstruct(spans []wordSpan) string {
	var b strings.Builder
	for _, s := range spans {
		b.WriteString(s.Text)
	}
	return b.String()
}

func hasEqualContaining(spans []wordSpan, needle string) bool {
	for _, s := range spans {
		if s.Kind == wordEqual && strings.Contains(s.Text, needle) {
			return true
		}
	}
	return false
}

func hasChangedContaining(spans []wordSpan, needle string) bool {
	for _, s := range spans {
		if s.Kind == wordChanged && strings.Contains(s.Text, needle) {
			return true
		}
	}
	return false
}
