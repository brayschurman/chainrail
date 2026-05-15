package cmd

import (
	"bytes"
	"context"
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

// TestSync_SquashMergedParent is the marquee feature: PR #683 fix.
//
// Setup:
//   - 2-branch stack: schema (PR #100) and api (PR #101)
//   - schema's PR was squash-merged into main on GitHub (we simulate by
//     advancing remote main with a commit that produces the same content)
//   - api is still open; its base on GitHub is still "bray/foo-1-schema"
//   - Locally, the user has both branches at their pre-merge tips
//
// Expected after sync:
//   - api's history contains the new main HEAD (the "squash commit") plus
//     api's unique commits, with NO duplicated schema commits
//   - PR #101's base on GitHub has been flipped to "main"
//   - No conflicts thrown
func TestSync_SquashMergedParent(t *testing.T) {
	dir, mock := submittedStack(t)

	// Find the open PRs in the mock and grab the schema one.
	var schemaPR, apiPR github.PullRequest
	for _, pr := range mock.PRs {
		if pr.HeadRefName == "bray/foo-1-schema" {
			schemaPR = pr
		}
		if pr.HeadRefName == "bray/foo-2-api" {
			apiPR = pr
		}
	}

	// Simulate squash-merge of schema: advance remote main with a commit whose
	// tree matches what schema produced (so api's unique commits will replay cleanly).
	remoteURL := strings.TrimSpace(mustOutput(t, dir, "git", "remote", "get-url", "origin"))
	tmpClone := t.TempDir()
	mustExec(t, "", "git", "clone", remoteURL, tmpClone)
	mustExec(t, tmpClone, "git", "config", "user.email", "merger@test.com")
	mustExec(t, tmpClone, "git", "config", "user.name", "merger")
	// Mirror schema's effect: write s.txt with the schema branch's content.
	if err := os.WriteFile(filepath.Join(tmpClone, "s.txt"), []byte("schema\n"), 0644); err != nil {
		t.Fatal(err)
	}
	mustExec(t, tmpClone, "git", "add", ".")
	mustExec(t, tmpClone, "git", "commit", "-m", "squash of schema (#100)")
	mustExec(t, tmpClone, "git", "push", "origin", "main")

	// Now we need the squash SHA — fetch it from the bare remote.
	cloneSHA := strings.TrimSpace(mustOutput(t, tmpClone, "git", "rev-parse", "HEAD"))

	// Mark schema's PR as MERGED with that SHA in the mock.
	mock.SetState(schemaPR.Number, "MERGED")
	pr := mock.PRs[schemaPR.Number]
	pr.MergeCommitSHA = cloneSHA
	mock.PRs[schemaPR.Number] = pr

	// Run sync from the api branch.
	mustExec(t, dir, "git", "checkout", "bray/foo-2-api")
	if err := runSync(&bytes.Buffer{}, newRendererForTests(), syncDeps{cwd: dir, gh: mock}); err != nil {
		t.Fatalf("sync should succeed via squash recovery, got: %v", err)
	}

	// (1) PR #101's base should now be "main".
	updatedAPI, _ := mock.GetPR(context.Background(), apiPR.Number)
	if updatedAPI.BaseRefName != "main" {
		t.Fatalf("api PR base: got %q want main", updatedAPI.BaseRefName)
	}

	// (2) api branch history should contain the squash commit and the api commit,
	//     but NOT a duplicate schema commit (only one s.txt-writing commit reachable).
	log := mustOutput(t, dir, "git", "log", "--format=%s", "bray/foo-2-api")
	if !strings.Contains(log, "squash of schema") {
		t.Fatalf("api history should include the squash commit, got:\n%s", log)
	}
	if !strings.Contains(log, "api") {
		t.Fatalf("api history should still include the api commit, got:\n%s", log)
	}
	// Count occurrences of "schema" lines that aren't the squash. Original schema branch
	// commit message was "schema" — if duplication happened that string would appear.
	schemaCommits := strings.Count(log, "\nschema\n") + strings.Count(log, "schema\n")
	// We expect exactly 1 occurrence (the "squash of schema (#100)" line, which contains "schema").
	// Tighter check: the original "schema" commit message (line equals "schema" exactly) should not appear.
	for _, line := range strings.Split(log, "\n") {
		if line == "schema" {
			t.Fatalf("original schema commit leaked into api after squash recovery:\n%s", log)
		}
	}
	_ = schemaCommits
}

// TestSync_DoubleSquashedParents handles two squash-merges in a row (rare but
// real: PRs #1 and #2 both merged via squash while #3 was still pending).
func TestSync_DoubleSquashedParents(t *testing.T) {
	dir := initTestRepoWithChainrail(t)
	remoteDir := t.TempDir() + "/remote.git"
	mustExec(t, "", "git", "init", "--bare", "-b", "main", remoteDir)
	mustExec(t, dir, "git", "remote", "add", "origin", remoteDir)
	mustExec(t, dir, "git", "push", "origin", "main")

	// 3-branch stack
	mustExec(t, dir, "git", "checkout", "-b", "bray/zz-1-schema")
	if err := os.WriteFile(filepath.Join(dir, "s.txt"), []byte("schema\n"), 0644); err != nil {
		t.Fatal(err)
	}
	mustExec(t, dir, "git", "add", ".")
	mustExec(t, dir, "git", "commit", "-m", "schema")

	mustExec(t, dir, "git", "checkout", "-b", "bray/zz-2-api")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("api\n"), 0644); err != nil {
		t.Fatal(err)
	}
	mustExec(t, dir, "git", "add", ".")
	mustExec(t, dir, "git", "commit", "-m", "api")

	mustExec(t, dir, "git", "checkout", "-b", "bray/zz-3-ui")
	if err := os.WriteFile(filepath.Join(dir, "u.txt"), []byte("ui\n"), 0644); err != nil {
		t.Fatal(err)
	}
	mustExec(t, dir, "git", "add", ".")
	mustExec(t, dir, "git", "commit", "-m", "ui")

	mock := github.NewMock()
	mock.User = "bray"
	if err := runSubmit(&bytes.Buffer{}, newRendererForTests(), submitDeps{cwd: dir, gh: mock}); err != nil {
		t.Fatal(err)
	}

	var schemaPR, apiPR, uiPR github.PullRequest
	for _, pr := range mock.PRs {
		switch pr.HeadRefName {
		case "bray/zz-1-schema":
			schemaPR = pr
		case "bray/zz-2-api":
			apiPR = pr
		case "bray/zz-3-ui":
			uiPR = pr
		}
	}

	// Simulate squash-merge of schema and api in succession.
	remoteURL := strings.TrimSpace(mustOutput(t, dir, "git", "remote", "get-url", "origin"))
	tmpClone := t.TempDir()
	mustExec(t, "", "git", "clone", remoteURL, tmpClone)
	mustExec(t, tmpClone, "git", "config", "user.email", "merger@test.com")
	mustExec(t, tmpClone, "git", "config", "user.name", "merger")

	if err := os.WriteFile(filepath.Join(tmpClone, "s.txt"), []byte("schema\n"), 0644); err != nil {
		t.Fatal(err)
	}
	mustExec(t, tmpClone, "git", "add", ".")
	mustExec(t, tmpClone, "git", "commit", "-m", "squash of schema")
	schemaSquashSHA := strings.TrimSpace(mustOutput(t, tmpClone, "git", "rev-parse", "HEAD"))

	if err := os.WriteFile(filepath.Join(tmpClone, "a.txt"), []byte("api\n"), 0644); err != nil {
		t.Fatal(err)
	}
	mustExec(t, tmpClone, "git", "add", ".")
	mustExec(t, tmpClone, "git", "commit", "-m", "squash of api")
	apiSquashSHA := strings.TrimSpace(mustOutput(t, tmpClone, "git", "rev-parse", "HEAD"))

	mustExec(t, tmpClone, "git", "push", "origin", "main")

	mock.SetState(schemaPR.Number, "MERGED")
	sp := mock.PRs[schemaPR.Number]
	sp.MergeCommitSHA = schemaSquashSHA
	mock.PRs[schemaPR.Number] = sp

	mock.SetState(apiPR.Number, "MERGED")
	ap := mock.PRs[apiPR.Number]
	ap.MergeCommitSHA = apiSquashSHA
	mock.PRs[apiPR.Number] = ap

	mustExec(t, dir, "git", "checkout", "bray/zz-3-ui")
	if err := runSync(&bytes.Buffer{}, newRendererForTests(), syncDeps{cwd: dir, gh: mock}); err != nil {
		t.Fatalf("sync with two squashed parents should succeed, got: %v", err)
	}

	// PR #ui's base should now be main.
	updatedUI, _ := mock.GetPR(context.Background(), uiPR.Number)
	if updatedUI.BaseRefName != "main" {
		t.Fatalf("ui PR base after double squash recovery: got %q want main", updatedUI.BaseRefName)
	}

	// ui should have main + squash of schema + squash of api + ui (and nothing else).
	log := mustOutput(t, dir, "git", "log", "--format=%s", "bray/zz-3-ui")
	for _, line := range strings.Split(log, "\n") {
		if line == "schema" || line == "api" {
			t.Fatalf("original %q commit leaked into ui after double squash recovery:\n%s", line, log)
		}
	}
}
