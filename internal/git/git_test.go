package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func runMust(t *testing.T, dir string, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s: %v\n%s", name, strings.Join(args, " "), err, out)
	}
	return string(out)
}

// newTestRepo creates a fresh git repo with one commit on the `main` branch.
func newTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runMust(t, dir, "git", "init", "-b", "main")
	runMust(t, dir, "git", "config", "user.email", "test@test.com")
	runMust(t, dir, "git", "config", "user.name", "test")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hi\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runMust(t, dir, "git", "add", ".")
	runMust(t, dir, "git", "commit", "-m", "init")
	return dir
}

func TestIsInsideRepo(t *testing.T) {
	dir := newTestRepo(t)
	g := New(dir)
	if !g.IsInsideRepo() {
		t.Fatal("expected inside repo to be true")
	}

	g2 := New(t.TempDir())
	if g2.IsInsideRepo() {
		t.Fatal("expected non-repo dir to return false")
	}
}

func TestCurrentBranch(t *testing.T) {
	dir := newTestRepo(t)
	g := New(dir)
	branch, err := g.CurrentBranch()
	if err != nil {
		t.Fatal(err)
	}
	if branch != "main" {
		t.Fatalf("got %q want main", branch)
	}
}

func TestIsDirty_Clean(t *testing.T) {
	dir := newTestRepo(t)
	g := New(dir)
	dirty, err := g.IsDirty()
	if err != nil {
		t.Fatal(err)
	}
	if dirty {
		t.Fatal("expected clean repo, got dirty")
	}
}

func TestIsDirty_UntrackedFile(t *testing.T) {
	dir := newTestRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	g := New(dir)
	dirty, err := g.IsDirty()
	if err != nil {
		t.Fatal(err)
	}
	if !dirty {
		t.Fatal("expected dirty after untracked file")
	}
}

func TestCreateBranch_AndBranchExists(t *testing.T) {
	dir := newTestRepo(t)
	g := New(dir)
	if exists, _ := g.BranchExists("feature/x"); exists {
		t.Fatal("branch should not exist yet")
	}
	if err := g.CreateBranch("feature/x"); err != nil {
		t.Fatal(err)
	}
	exists, err := g.BranchExists("feature/x")
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Fatal("branch should exist after CreateBranch")
	}
}

func TestRevParse(t *testing.T) {
	dir := newTestRepo(t)
	g := New(dir)
	sha, err := g.RevParse("HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if len(sha) != 40 {
		t.Fatalf("expected 40-char sha, got %q (len %d)", sha, len(sha))
	}
}

func TestConfigGetSet(t *testing.T) {
	dir := newTestRepo(t)
	g := New(dir)
	if err := g.ConfigSet("chainrail.trunk", "main"); err != nil {
		t.Fatal(err)
	}
	val, err := g.ConfigGet("chainrail.trunk")
	if err != nil {
		t.Fatal(err)
	}
	if val != "main" {
		t.Fatalf("got %q want main", val)
	}
}

func TestConfigGet_MissingKey(t *testing.T) {
	dir := newTestRepo(t)
	g := New(dir)
	_, err := g.ConfigGet("nope.nada")
	if err == nil {
		t.Fatal("expected error for missing key")
	}
}

func TestRebase_HappyPath(t *testing.T) {
	dir := newTestRepo(t)
	runMust(t, dir, "git", "checkout", "-b", "child")
	if err := os.WriteFile(filepath.Join(dir, "c.txt"), []byte("c"), 0644); err != nil {
		t.Fatal(err)
	}
	runMust(t, dir, "git", "add", ".")
	runMust(t, dir, "git", "commit", "-m", "c")

	runMust(t, dir, "git", "checkout", "main")
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b"), 0644); err != nil {
		t.Fatal(err)
	}
	runMust(t, dir, "git", "add", ".")
	runMust(t, dir, "git", "commit", "-m", "b")

	runMust(t, dir, "git", "checkout", "child")
	g := New(dir)
	if err := g.Rebase("main"); err != nil {
		t.Fatalf("rebase failed: %v", err)
	}
	out := runMust(t, dir, "git", "log", "--oneline")
	if !strings.Contains(out, " b") || !strings.Contains(out, " c") {
		t.Fatalf("after rebase, log missing commits: %q", out)
	}
}

func TestRebaseOnto_HappyPath(t *testing.T) {
	dir := newTestRepo(t)
	// build: main -> A; mid: main + B; tip: mid + C
	runMust(t, dir, "git", "checkout", "-b", "mid")
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b"), 0644); err != nil {
		t.Fatal(err)
	}
	runMust(t, dir, "git", "add", ".")
	runMust(t, dir, "git", "commit", "-m", "B")
	midTip := strings.TrimSpace(runMust(t, dir, "git", "rev-parse", "HEAD"))

	runMust(t, dir, "git", "checkout", "-b", "tip")
	if err := os.WriteFile(filepath.Join(dir, "c.txt"), []byte("c"), 0644); err != nil {
		t.Fatal(err)
	}
	runMust(t, dir, "git", "add", ".")
	runMust(t, dir, "git", "commit", "-m", "C")

	// Advance main with a new commit S (simulating squash-merge of mid).
	runMust(t, dir, "git", "checkout", "main")
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b"), 0644); err != nil {
		t.Fatal(err)
	}
	runMust(t, dir, "git", "add", ".")
	runMust(t, dir, "git", "commit", "-m", "squash-of-B")
	squashSHA := strings.TrimSpace(runMust(t, dir, "git", "rev-parse", "HEAD"))

	// rebase tip onto squashSHA, dropping commits in mid (midTip..tip is just C).
	g := New(dir)
	if err := g.RebaseOnto(squashSHA, midTip, "tip"); err != nil {
		t.Fatalf("rebase --onto failed: %v", err)
	}
	out := runMust(t, dir, "git", "log", "--oneline", "tip")
	// tip should now contain squash-of-B and C, not the original B commit-message.
	if !strings.Contains(out, "C") || !strings.Contains(out, "squash-of-B") {
		t.Fatalf("expected tip log to contain C and squash-of-B, got: %q", out)
	}
	if strings.Count(out, "\n") > 3 {
		t.Fatalf("expected ~3 commits, got: %q", out)
	}
}

func TestRemoteExists_NoRemote(t *testing.T) {
	dir := newTestRepo(t)
	g := New(dir)
	exists, err := g.RemoteExists("main")
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatal("no remote configured, should return false")
	}
}
