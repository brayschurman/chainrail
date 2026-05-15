// Package reviewstate persists per-file PR-review progress on disk.
//
// State is keyed by (owner/repo, PR number, file path, blob_sha). The blob_sha
// is recorded at the moment the reviewer marks a file done — so a later
// commit to the same file leaves the mark intact (and surfaces a "changed
// since you checked" badge) instead of silently invalidating progress.
package reviewstate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// FileMark records that a reviewer checked off one file at a specific blob.
type FileMark struct {
	BlobSHA   string    `json:"blob_sha"`
	CheckedAt time.Time `json:"checked_at"`
	// Waiver is non-empty when the reviewer used shift+W to override a
	// hard-block detector (e.g. CI changes). Stored for audit.
	Waiver string `json:"waiver,omitempty"`
}

// PRState is the on-disk shape for one PR's review progress.
type PRState struct {
	PR       int                 `json:"pr"`
	Reviewer string              `json:"reviewer"`
	Files    map[string]FileMark `json:"files"`
	// FirstCheckedAt is the timestamp of the first file mark for this PR.
	// Used to render "elapsed time" in the progress meter.
	FirstCheckedAt time.Time `json:"first_checked_at,omitempty"`
	// NudgedForPlanAt is set when the reviewer pressed P to request a plan
	// from the author; we don't double-prompt.
	NudgedForPlanAt time.Time `json:"nudged_for_plan_at,omitempty"`
}

// Store reads/writes per-PR state files under a single base directory.
//
// Layout:
//
//	<base>/<owner>__<repo>/<pr>.json
type Store struct {
	base string
}

// NewStore returns a Store rooted at ~/.config/chainrail/reviews, creating
// the directory if needed. If $HOME is unset, falls back to /tmp.
func NewStore() (*Store, error) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		home = os.TempDir()
	}
	base := filepath.Join(home, ".config", "chainrail", "reviews")
	if err := os.MkdirAll(base, 0o755); err != nil {
		return nil, fmt.Errorf("create review-state dir: %w", err)
	}
	return &Store{base: base}, nil
}

// NewStoreAt is for tests — explicit base dir, no home lookup.
func NewStoreAt(base string) (*Store, error) {
	if err := os.MkdirAll(base, 0o755); err != nil {
		return nil, fmt.Errorf("create review-state dir: %w", err)
	}
	return &Store{base: base}, nil
}

// Load returns the PRState for the given repo/PR, or a fresh empty state if
// no file exists yet. A nil Store returns an empty state and never errors;
// the caller can use this to silently degrade when state persistence fails.
func (s *Store) Load(owner, repo string, pr int) (*PRState, error) {
	if s == nil {
		return emptyPRState(pr), nil
	}
	path := s.pathFor(owner, repo, pr)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return emptyPRState(pr), nil
		}
		return nil, fmt.Errorf("read review state: %w", err)
	}
	var st PRState
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, fmt.Errorf("parse review state %s: %w", path, err)
	}
	if st.Files == nil {
		st.Files = map[string]FileMark{}
	}
	return &st, nil
}

// Save writes the PRState to disk atomically. Nil Store is a no-op so the
// rest of the UI keeps working when persistence fails at init time.
func (s *Store) Save(owner, repo string, st *PRState) error {
	if s == nil || st == nil {
		return nil
	}
	dir := filepath.Join(s.base, repoSlug(owner, repo))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create review-state subdir: %w", err)
	}
	path := filepath.Join(dir, fmt.Sprintf("%d.json", st.PR))
	tmp := path + ".tmp"
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal review state: %w", err)
	}
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write review state tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename review state: %w", err)
	}
	return nil
}

// Toggle flips the reviewed state for path. When marking checked, blobSHA is
// recorded so we can later detect "this file changed since you reviewed it."
// Returns the new state (true = now checked).
func (st *PRState) Toggle(path, blobSHA string, now time.Time) bool {
	if st.Files == nil {
		st.Files = map[string]FileMark{}
	}
	if _, was := st.Files[path]; was {
		delete(st.Files, path)
		return false
	}
	st.Files[path] = FileMark{BlobSHA: blobSHA, CheckedAt: now}
	if st.FirstCheckedAt.IsZero() {
		st.FirstCheckedAt = now
	}
	return true
}

// Set marks path as reviewed at blobSHA, overwriting any existing mark.
// Used by "mark all in chunk" bulk operations and waiver writes.
func (st *PRState) Set(path, blobSHA, waiver string, now time.Time) {
	if st.Files == nil {
		st.Files = map[string]FileMark{}
	}
	st.Files[path] = FileMark{BlobSHA: blobSHA, CheckedAt: now, Waiver: waiver}
	if st.FirstCheckedAt.IsZero() {
		st.FirstCheckedAt = now
	}
}

// IsChecked reports whether path has any mark — regardless of blob_sha
// freshness. Use ChangedSince for staleness checks.
func (st *PRState) IsChecked(path string) bool {
	_, ok := st.Files[path]
	return ok
}

// ChangedSince returns true when path is checked but at a different blob_sha
// than the current one (i.e. the file has been updated since the reviewer
// last marked it).
func (st *PRState) ChangedSince(path, currentBlobSHA string) bool {
	mark, ok := st.Files[path]
	if !ok {
		return false
	}
	return mark.BlobSHA != "" && currentBlobSHA != "" && mark.BlobSHA != currentBlobSHA
}

func emptyPRState(pr int) *PRState {
	return &PRState{
		PR:    pr,
		Files: map[string]FileMark{},
	}
}

func (s *Store) pathFor(owner, repo string, pr int) string {
	return filepath.Join(s.base, repoSlug(owner, repo), fmt.Sprintf("%d.json", pr))
}

// repoSlug produces a filesystem-safe directory name for an owner/repo pair.
func repoSlug(owner, repo string) string {
	clean := func(s string) string {
		s = strings.ToLower(s)
		s = strings.ReplaceAll(s, "/", "_")
		s = strings.ReplaceAll(s, "\\", "_")
		return s
	}
	return clean(owner) + "__" + clean(repo)
}
