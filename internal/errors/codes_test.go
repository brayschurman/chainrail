package errors

import (
	"errors"
	"testing"
)

func TestChainrailError_ErrorString_WithSuggestion(t *testing.T) {
	e := &ChainrailError{
		Code:       CodeDirtyWorktree,
		Message:    "working tree has uncommitted changes",
		Suggestion: "commit or stash before adding to the stack",
	}
	got := e.Error()
	if got == "" {
		t.Fatal("Error() returned empty string")
	}
	if !contains(got, "working tree") {
		t.Fatalf("Error() missing message: %q", got)
	}
	if !contains(got, "commit or stash") {
		t.Fatalf("Error() missing suggestion: %q", got)
	}
}

func TestChainrailError_ErrorString_NoSuggestion(t *testing.T) {
	e := &ChainrailError{
		Code:    CodeNotGitRepo,
		Message: "not a git repo",
	}
	got := e.Error()
	if got != "not a git repo" {
		t.Fatalf("expected just the message, got %q", got)
	}
}

func TestChainrailError_Unwrap(t *testing.T) {
	inner := errors.New("inner error")
	e := &ChainrailError{
		Code:    CodeGitCallFailed,
		Message: "git failed",
		Cause:   inner,
	}
	if !errors.Is(e, inner) {
		t.Fatal("errors.Is should find the wrapped cause")
	}
	if errors.Unwrap(e) != inner {
		t.Fatal("Unwrap should return the cause")
	}
}

func TestChainrailError_ImplementsError(t *testing.T) {
	var _ error = (*ChainrailError)(nil)
}

func TestCanonicalCodes_AllDefined(t *testing.T) {
	codes := []string{
		CodeNotGitRepo,
		CodeNoGhAuth,
		CodeTrunkMissing,
		CodeAlreadyInit,
		CodeDirtyWorktree,
		CodeNotOnStack,
		CodeSlugTaken,
		CodeGhCallFailed,
		CodeGitCallFailed,
		CodeSquashDetected,
		CodeRebaseConflict,
	}
	seen := map[string]bool{}
	for _, c := range codes {
		if c == "" {
			t.Fatal("code is empty string")
		}
		if seen[c] {
			t.Fatalf("duplicate code: %q", c)
		}
		seen[c] = true
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
