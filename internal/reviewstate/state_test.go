package reviewstate

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStore_LoadEmptyReturnsFresh(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStoreAt(dir)
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.Load("o", "r", 42)
	if err != nil {
		t.Fatal(err)
	}
	if st.PR != 42 {
		t.Errorf("pr = %d, want 42", st.PR)
	}
	if len(st.Files) != 0 {
		t.Errorf("expected empty files, got %v", st.Files)
	}
}

func TestStore_SaveAndLoadRoundtrip(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewStoreAt(dir)

	st := emptyPRState(99)
	st.Reviewer = "bray"
	now := time.Date(2026, 5, 15, 14, 22, 0, 0, time.UTC)
	st.Toggle("src/foo.go", "abc123", now)
	st.Set("src/bar.go", "def456", "intentional removal", now)

	if err := store.Save("brayschurman", "chainrail", st); err != nil {
		t.Fatal(err)
	}

	loaded, err := store.Load("brayschurman", "chainrail", 99)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Reviewer != "bray" {
		t.Errorf("reviewer = %q", loaded.Reviewer)
	}
	if !loaded.IsChecked("src/foo.go") {
		t.Error("foo.go should be checked")
	}
	if !loaded.IsChecked("src/bar.go") {
		t.Error("bar.go should be checked")
	}
	if loaded.Files["src/bar.go"].Waiver != "intentional removal" {
		t.Errorf("waiver lost: %+v", loaded.Files["src/bar.go"])
	}
}

func TestPRState_ToggleFlipsState(t *testing.T) {
	st := emptyPRState(1)
	now := time.Now()

	checked := st.Toggle("a.go", "sha1", now)
	if !checked {
		t.Error("first toggle should check")
	}
	if !st.IsChecked("a.go") {
		t.Error("a.go should be checked")
	}

	checked = st.Toggle("a.go", "sha1", now)
	if checked {
		t.Error("second toggle should uncheck")
	}
	if st.IsChecked("a.go") {
		t.Error("a.go should be unchecked after second toggle")
	}
}

func TestPRState_ChangedSinceDetectsBlobDelta(t *testing.T) {
	st := emptyPRState(1)
	st.Toggle("a.go", "abc", time.Now())

	if st.ChangedSince("a.go", "abc") {
		t.Error("same blob should not be changed")
	}
	if !st.ChangedSince("a.go", "def") {
		t.Error("different blob should be changed")
	}
	if st.ChangedSince("b.go", "anything") {
		t.Error("unchecked file should never be 'changed since'")
	}
}

func TestStore_NilStoreIsNoOp(t *testing.T) {
	var s *Store
	st, err := s.Load("o", "r", 1)
	if err != nil {
		t.Fatal(err)
	}
	if st.PR != 1 {
		t.Errorf("pr = %d, want 1", st.PR)
	}
	if err := s.Save("o", "r", st); err != nil {
		t.Errorf("nil save should not error, got %v", err)
	}
}

func TestStore_FilesystemLayout(t *testing.T) {
	dir := t.TempDir()
	store, _ := NewStoreAt(dir)
	st := emptyPRState(123)
	st.Toggle("x.go", "sha", time.Now())

	if err := store.Save("My-Org", "my/repo", st); err != nil {
		t.Fatal(err)
	}

	expected := filepath.Join(dir, "my-org__my_repo", "123.json")
	if _, err := os.Stat(expected); err != nil {
		t.Errorf("expected file at %s, got %v", expected, err)
	}
}

func TestPRState_FirstCheckedAtRecordedOnce(t *testing.T) {
	st := emptyPRState(1)
	t1 := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	t2 := t1.Add(time.Hour)

	st.Toggle("a.go", "sha", t1)
	st.Toggle("b.go", "sha", t2)

	if !st.FirstCheckedAt.Equal(t1) {
		t.Errorf("FirstCheckedAt = %v, want %v", st.FirstCheckedAt, t1)
	}
}
