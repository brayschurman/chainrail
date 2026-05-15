package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	crerrors "github.com/brayschurman/chainrail/internal/errors"
)

func TestAddCommandIsRegistered(t *testing.T) {
	if !commandRegistered("add") {
		t.Fatal("add command not registered with root")
	}
}

func TestAddCommand_RequiresOneArg(t *testing.T) {
	rootCmd.SetArgs([]string{"add"})
	var out, errOut bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&errOut)
	if err := rootCmd.Execute(); err == nil {
		t.Fatal("expected error when no arg passed")
	}
}

// initTestRepoWithChainrail extends initTestRepo with chainrail.trunk config.
func initTestRepoWithChainrail(t *testing.T) string {
	t.Helper()
	dir := initTestRepo(t)
	mustExec(t, dir, "git", "config", "chainrail.trunk", "main")
	return dir
}

func userResolver(name string) func() (string, error) {
	return func() (string, error) { return name, nil }
}

func TestRunAdd_FromTrunk_CreatesFirstBranch(t *testing.T) {
	dir := initTestRepoWithChainrail(t)
	var out bytes.Buffer
	err := runAdd(&out, newRendererForTests(), "schema", addDeps{
		cwd:     dir,
		getUser: userResolver("bray"),
	})
	if err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(mustOutput(t, dir, "git", "rev-parse", "--abbrev-ref", "HEAD"))
	want := "bray/schema-1-schema"
	if got != want {
		t.Fatalf("got branch %q want %q", got, want)
	}
}

func TestRunAdd_FromExistingStack_IncrementsPosition(t *testing.T) {
	dir := initTestRepoWithChainrail(t)
	if err := runAdd(&bytes.Buffer{}, newRendererForTests(), "schema", addDeps{cwd: dir, getUser: userResolver("bray")}); err != nil {
		t.Fatal(err)
	}
	// Need a commit before we can add the next layer (the new branch will be from the current tip).
	if err := os.WriteFile(filepath.Join(dir, "schema.txt"), []byte("s"), 0644); err != nil {
		t.Fatal(err)
	}
	mustExec(t, dir, "git", "add", ".")
	mustExec(t, dir, "git", "commit", "-m", "schema work")

	var out bytes.Buffer
	if err := runAdd(&out, newRendererForTests(), "api", addDeps{cwd: dir, getUser: userResolver("bray")}); err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(mustOutput(t, dir, "git", "rev-parse", "--abbrev-ref", "HEAD"))
	want := "bray/schema-2-api"
	if got != want {
		t.Fatalf("got branch %q want %q", got, want)
	}
}

func TestRunAdd_FromNonStackBranch_NotOnStack(t *testing.T) {
	dir := initTestRepoWithChainrail(t)
	mustExec(t, dir, "git", "checkout", "-b", "random-branch")
	err := runAdd(&bytes.Buffer{}, newRendererForTests(), "x", addDeps{cwd: dir, getUser: userResolver("bray")})
	assertChainrailErr(t, err, crerrors.CodeNotOnStack)
}

func TestRunAdd_DirtyWorktree(t *testing.T) {
	dir := initTestRepoWithChainrail(t)
	if err := os.WriteFile(filepath.Join(dir, "dirty.txt"), []byte("d"), 0644); err != nil {
		t.Fatal(err)
	}
	err := runAdd(&bytes.Buffer{}, newRendererForTests(), "x", addDeps{cwd: dir, getUser: userResolver("bray")})
	assertChainrailErr(t, err, crerrors.CodeDirtyWorktree)
}

func TestRunAdd_BranchAlreadyExists(t *testing.T) {
	dir := initTestRepoWithChainrail(t)
	mustExec(t, dir, "git", "branch", "bray/foo-1-foo", "main")
	err := runAdd(&bytes.Buffer{}, newRendererForTests(), "foo", addDeps{cwd: dir, getUser: userResolver("bray")})
	assertChainrailErr(t, err, crerrors.CodeSlugTaken)
}

func TestRunAdd_NoChainrailInit_NotOnStack(t *testing.T) {
	dir := initTestRepo(t) // no chainrail.trunk set
	err := runAdd(&bytes.Buffer{}, newRendererForTests(), "x", addDeps{cwd: dir, getUser: userResolver("bray")})
	assertChainrailErr(t, err, crerrors.CodeNotOnStack)
}

func TestRunAdd_DifferentUser_NotOnStack(t *testing.T) {
	dir := initTestRepoWithChainrail(t)
	// build alice's stack
	if err := runAdd(&bytes.Buffer{}, newRendererForTests(), "schema", addDeps{cwd: dir, getUser: userResolver("alice")}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "s.txt"), []byte("s"), 0644); err != nil {
		t.Fatal(err)
	}
	mustExec(t, dir, "git", "add", ".")
	mustExec(t, dir, "git", "commit", "-m", "s")

	// now bray tries to add — alice's branch doesn't belong to him
	err := runAdd(&bytes.Buffer{}, newRendererForTests(), "api", addDeps{cwd: dir, getUser: userResolver("bray")})
	assertChainrailErr(t, err, crerrors.CodeNotOnStack)
}

func TestParseStackBranch(t *testing.T) {
	cases := []struct {
		branch   string
		user     string
		wantOK   bool
		wantBase string
		wantPos  int
		wantTask string
	}{
		// simple slug, no hyphens
		{"bray/feat-1-schema", "bray", true, "feat", 1, "schema"},
		// base slug with hyphens
		{"bray/ignore-logs-1-ignore-logs", "bray", true, "ignore-logs", 1, "ignore-logs"},
		// task slug with hyphens
		{"bray/feat-2-my-task", "bray", true, "feat", 2, "my-task"},
		// both slugs with hyphens
		{"bray/dev-200-3-fix-null-check", "bray", true, "dev-200", 3, "fix-null-check"},
		// position > 1
		{"bray/ignore-logs-2-readme", "bray", true, "ignore-logs", 2, "readme"},
		// wrong user
		{"alice/feat-1-schema", "bray", false, "", 0, ""},
		// no position number
		{"bray/feat-schema", "bray", false, "", 0, ""},
		// trunk branch
		{"main", "bray", false, "", 0, ""},
	}
	for _, tc := range cases {
		t.Run(tc.branch, func(t *testing.T) {
			got, ok := parseStackBranch(tc.branch, tc.user)
			if ok != tc.wantOK {
				t.Fatalf("ok=%v want %v", ok, tc.wantOK)
			}
			if !ok {
				return
			}
			if got.baseSlug != tc.wantBase || got.position != tc.wantPos || got.taskSlug != tc.wantTask {
				t.Errorf("got {%q %d %q}, want {%q %d %q}",
					got.baseSlug, got.position, got.taskSlug,
					tc.wantBase, tc.wantPos, tc.wantTask)
			}
		})
	}
}

func mustOutput(t *testing.T, dir, name string, args ...string) string {
	t.Helper()
	cmd := commandFor(dir, name, args...)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("%s %s: %v", name, strings.Join(args, " "), err)
	}
	return string(out)
}
