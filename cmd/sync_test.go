package cmd

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	crerrors "github.com/brayschurman/chainrail/internal/errors"
	"github.com/brayschurman/chainrail/internal/git"
	"github.com/brayschurman/chainrail/internal/github"
)

func TestSyncCommandIsRegistered(t *testing.T) {
	if !commandRegistered("sync") {
		t.Fatal("sync command not registered with root")
	}
}

// submittedStack runs submit on the standard 2-branch stack so PRs exist in the
// mock, returns the workDir and the populated mock.
func submittedStack(t *testing.T) (string, *github.MockGhClient) {
	t.Helper()
	dir := initStackRepoWithRemote(t)
	mock := github.NewMock()
	mock.User = "bray"
	if err := runSubmit(&bytes.Buffer{}, newRendererForTests(), submitDeps{cwd: dir, gh: mock}); err != nil {
		t.Fatal(err)
	}
	return dir, mock
}

// advanceTrunkOnRemote pushes a new commit to origin/main from a fresh clone of
// the bare remote, then deletes the clone. This simulates a teammate landing
// on main while the user's stack stays put.
func advanceTrunkOnRemote(t *testing.T, dir string) {
	t.Helper()
	remoteURL := strings.TrimSpace(mustOutput(t, dir, "git", "remote", "get-url", "origin"))
	tmpClone := t.TempDir()
	mustExec(t, "", "git", "clone", remoteURL, tmpClone)
	mustExec(t, tmpClone, "git", "config", "user.email", "other@test.com")
	mustExec(t, tmpClone, "git", "config", "user.name", "other")
	if err := os.WriteFile(filepath.Join(tmpClone, "remote-advance.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	mustExec(t, tmpClone, "git", "add", ".")
	mustExec(t, tmpClone, "git", "commit", "-m", "remote advance")
	mustExec(t, tmpClone, "git", "push", "origin", "main")
}

func TestRunSync_HappyPath_TrunkAdvanced(t *testing.T) {
	dir, mock := submittedStack(t)
	advanceTrunkOnRemote(t, dir)

	mustExec(t, dir, "git", "checkout", "bray/foo-2-api")
	if err := runSync(&bytes.Buffer{}, newRendererForTests(), syncDeps{cwd: dir, gh: mock}); err != nil {
		t.Fatalf("sync failed: %v", err)
	}

	// After sync, the stack should have the remote-advance commit in its history.
	log := mustOutput(t, dir, "git", "log", "--oneline", "bray/foo-2-api")
	if !strings.Contains(log, "remote advance") {
		t.Fatalf("after sync, tip's history should include the trunk advance; got:\n%s", log)
	}
}

func TestRunSync_NoRemoteChanges_NoOp(t *testing.T) {
	dir, mock := submittedStack(t)
	mustExec(t, dir, "git", "checkout", "bray/foo-2-api")

	before := strings.TrimSpace(mustOutput(t, dir, "git", "rev-parse", "bray/foo-2-api"))
	if err := runSync(&bytes.Buffer{}, newRendererForTests(), syncDeps{cwd: dir, gh: mock}); err != nil {
		t.Fatalf("sync with no remote changes should succeed: %v", err)
	}
	after := strings.TrimSpace(mustOutput(t, dir, "git", "rev-parse", "bray/foo-2-api"))
	if before != after {
		t.Fatalf("no-op sync changed tip from %s to %s", before, after)
	}
}

func TestRunSync_WritesSnapshotRefs(t *testing.T) {
	dir, mock := submittedStack(t)
	advanceTrunkOnRemote(t, dir)
	mustExec(t, dir, "git", "checkout", "bray/foo-2-api")

	preTip := strings.TrimSpace(mustOutput(t, dir, "git", "rev-parse", "bray/foo-2-api"))

	if err := runSync(&bytes.Buffer{}, newRendererForTests(), syncDeps{cwd: dir, gh: mock}); err != nil {
		t.Fatal(err)
	}

	g := git.New(dir)
	snap, err := g.RevParse("refs/chainrail/snapshot/bray/foo-2-api")
	if err != nil {
		t.Fatalf("snapshot ref for api branch not found: %v", err)
	}
	if snap != preTip {
		t.Fatalf("snapshot points to %s, expected pre-rebase tip %s", snap, preTip)
	}
}

func TestRunSync_RestoresOriginalBranch(t *testing.T) {
	dir, mock := submittedStack(t)
	advanceTrunkOnRemote(t, dir)
	mustExec(t, dir, "git", "checkout", "bray/foo-2-api")

	if err := runSync(&bytes.Buffer{}, newRendererForTests(), syncDeps{cwd: dir, gh: mock}); err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(mustOutput(t, dir, "git", "rev-parse", "--abbrev-ref", "HEAD"))
	if got != "bray/foo-2-api" {
		t.Fatalf("expected to be back on bray/foo-2-api, got %q", got)
	}
}

func TestRunSync_RebaseConflict_SnapshotsStillWritten(t *testing.T) {
	dir, mock := submittedStack(t)

	remoteURL := strings.TrimSpace(mustOutput(t, dir, "git", "remote", "get-url", "origin"))
	tmpClone := t.TempDir()
	mustExec(t, "", "git", "clone", remoteURL, tmpClone)
	mustExec(t, tmpClone, "git", "config", "user.email", "other@test.com")
	mustExec(t, tmpClone, "git", "config", "user.name", "other")
	if err := os.WriteFile(filepath.Join(tmpClone, "s.txt"), []byte("REMOTE\n"), 0644); err != nil {
		t.Fatal(err)
	}
	mustExec(t, tmpClone, "git", "add", ".")
	mustExec(t, tmpClone, "git", "commit", "-m", "remote conflict")
	mustExec(t, tmpClone, "git", "push", "origin", "main")

	mustExec(t, dir, "git", "checkout", "bray/foo-2-api")
	preSchemaSHA := strings.TrimSpace(mustOutput(t, dir, "git", "rev-parse", "bray/foo-1-schema"))
	_ = runSync(&bytes.Buffer{}, newRendererForTests(), syncDeps{cwd: dir, gh: mock})

	g := git.New(dir)
	snap, err := g.RevParse("refs/chainrail/snapshot/bray/foo-1-schema")
	if err != nil {
		t.Fatalf("snapshot for schema not written before conflict: %v", err)
	}
	if snap != preSchemaSHA {
		t.Fatalf("snapshot points to %s, expected %s", snap, preSchemaSHA)
	}

	// Clean up the mid-rebase state so other tests aren't polluted (defensive).
	mustExec(t, dir, "git", "rebase", "--abort")
}

func TestRunSync_RebaseConflict_ReturnsCodeRebaseConflict(t *testing.T) {
	dir, mock := submittedStack(t)

	// Modify the same file (s.txt) on remote to conflict with the schema branch.
	remoteURL := strings.TrimSpace(mustOutput(t, dir, "git", "remote", "get-url", "origin"))
	tmpClone := t.TempDir()
	mustExec(t, "", "git", "clone", remoteURL, tmpClone)
	mustExec(t, tmpClone, "git", "config", "user.email", "other@test.com")
	mustExec(t, tmpClone, "git", "config", "user.name", "other")
	if err := os.WriteFile(filepath.Join(tmpClone, "s.txt"), []byte("REMOTE-OVERRIDES-SCHEMA\n"), 0644); err != nil {
		t.Fatal(err)
	}
	mustExec(t, tmpClone, "git", "add", ".")
	mustExec(t, tmpClone, "git", "commit", "-m", "remote conflicts schema")
	mustExec(t, tmpClone, "git", "push", "origin", "main")

	mustExec(t, dir, "git", "checkout", "bray/foo-2-api")
	err := runSync(&bytes.Buffer{}, newRendererForTests(), syncDeps{cwd: dir, gh: mock})
	if err == nil {
		t.Fatal("expected rebase conflict error, got nil")
	}
	var ce *crerrors.ChainrailError
	if !errors.As(err, &ce) || ce.Code != crerrors.CodeRebaseConflict {
		t.Fatalf("expected CodeRebaseConflict, got %v", err)
	}
}

func TestRunSync_FromTrunk_NotOnStack(t *testing.T) {
	dir, mock := submittedStack(t)
	mustExec(t, dir, "git", "checkout", "main")
	err := runSync(&bytes.Buffer{}, newRendererForTests(), syncDeps{cwd: dir, gh: mock})
	assertChainrailErr(t, err, crerrors.CodeNotOnStack)
}
