package cmd

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	crerrors "github.com/brayschurman/chainrail/internal/errors"
	"github.com/brayschurman/chainrail/internal/github"
)

func TestSubmitCommandIsRegistered(t *testing.T) {
	if !commandRegistered("submit") {
		t.Fatal("submit command not registered with root")
	}
}

// initStackRepoWithRemote builds a tempdir repo with chainrail initialized,
// a bare remote, the main branch pushed, and a 2-branch stack created locally
// with commits on each.
func initStackRepoWithRemote(t *testing.T) (workDir string) {
	t.Helper()
	dir := initTestRepoWithChainrail(t)
	remoteDir := t.TempDir() + "/remote.git"
	mustExec(t, "", "git", "init", "--bare", "-b", "main", remoteDir)
	mustExec(t, dir, "git", "remote", "add", "origin", remoteDir)
	mustExec(t, dir, "git", "push", "origin", "main")

	// Build a 2-layer stack: bray/foo-1-schema, bray/foo-2-api
	mustExec(t, dir, "git", "checkout", "-b", "bray/foo-1-schema")
	if err := os.WriteFile(filepath.Join(dir, "s.txt"), []byte("schema\n"), 0644); err != nil {
		t.Fatal(err)
	}
	mustExec(t, dir, "git", "add", ".")
	mustExec(t, dir, "git", "commit", "-m", "schema")

	mustExec(t, dir, "git", "checkout", "-b", "bray/foo-2-api")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("api\n"), 0644); err != nil {
		t.Fatal(err)
	}
	mustExec(t, dir, "git", "add", ".")
	mustExec(t, dir, "git", "commit", "-m", "api")

	return dir
}

func newSubmitDeps(dir string, mock *github.MockGhClient) submitDeps {
	return submitDeps{cwd: dir, gh: mock}
}

func TestRunSubmit_FreshStack_OpensAllPRs(t *testing.T) {
	dir := initStackRepoWithRemote(t)
	mock := github.NewMock()
	mock.User = "bray"

	var out bytes.Buffer
	if err := runSubmit(&out, newRendererForTests(), newSubmitDeps(dir, mock)); err != nil {
		t.Fatal(err)
	}
	prs, _ := mock.ListOpenPRs(context.Background())
	if len(prs) != 2 {
		t.Fatalf("expected 2 PRs, got %d", len(prs))
	}

	byHead := map[string]github.PullRequest{}
	for _, pr := range prs {
		byHead[pr.HeadRefName] = pr
	}
	schema, ok := byHead["bray/foo-1-schema"]
	if !ok {
		t.Fatal("schema PR not created")
	}
	if schema.BaseRefName != "main" {
		t.Fatalf("schema base: got %q want main", schema.BaseRefName)
	}
	api, ok := byHead["bray/foo-2-api"]
	if !ok {
		t.Fatal("api PR not created")
	}
	if api.BaseRefName != "bray/foo-1-schema" {
		t.Fatalf("api base: got %q want bray/foo-1-schema", api.BaseRefName)
	}
}

func TestRunSubmit_InjectsStackMapInBody(t *testing.T) {
	dir := initStackRepoWithRemote(t)
	mock := github.NewMock()
	mock.User = "bray"

	if err := runSubmit(&bytes.Buffer{}, newRendererForTests(), newSubmitDeps(dir, mock)); err != nil {
		t.Fatal(err)
	}
	for _, pr := range mock.PRs {
		if !strings.Contains(pr.Body, stackMapStartMarker) {
			t.Fatalf("PR #%d missing start marker: %q", pr.Number, pr.Body)
		}
		if !strings.Contains(pr.Body, stackMapEndMarker) {
			t.Fatalf("PR #%d missing end marker: %q", pr.Number, pr.Body)
		}
		if !strings.Contains(pr.Body, "bray/foo-1-schema") || !strings.Contains(pr.Body, "bray/foo-2-api") {
			t.Fatalf("PR #%d body missing branch listing: %q", pr.Number, pr.Body)
		}
	}
}

func TestRunSubmit_Idempotent(t *testing.T) {
	dir := initStackRepoWithRemote(t)
	mock := github.NewMock()
	mock.User = "bray"

	if err := runSubmit(&bytes.Buffer{}, newRendererForTests(), newSubmitDeps(dir, mock)); err != nil {
		t.Fatal(err)
	}
	prsBefore := len(mock.PRs)
	callsBefore := len(mock.Calls)

	if err := runSubmit(&bytes.Buffer{}, newRendererForTests(), newSubmitDeps(dir, mock)); err != nil {
		t.Fatal(err)
	}

	if len(mock.PRs) != prsBefore {
		t.Fatalf("expected 0 new PRs on second run, went from %d to %d", prsBefore, len(mock.PRs))
	}
	createCallsAfter := 0
	for _, c := range mock.Calls[callsBefore:] {
		if strings.HasPrefix(c, "CreatePR") {
			createCallsAfter++
		}
	}
	if createCallsAfter != 0 {
		t.Fatalf("expected 0 CreatePR calls on second run, got %d", createCallsAfter)
	}
	updateBodyCallsAfter := 0
	for _, c := range mock.Calls[callsBefore:] {
		if strings.HasPrefix(c, "UpdatePRBody") {
			updateBodyCallsAfter++
		}
	}
	if updateBodyCallsAfter != 0 {
		t.Fatalf("expected 0 UpdatePRBody calls on second idempotent run, got %d", updateBodyCallsAfter)
	}
}

func TestRunSubmit_FromTrunk_NotOnStack(t *testing.T) {
	dir := initStackRepoWithRemote(t)
	mustExec(t, dir, "git", "checkout", "main")
	mock := github.NewMock()
	mock.User = "bray"
	err := runSubmit(&bytes.Buffer{}, newRendererForTests(), newSubmitDeps(dir, mock))
	assertChainrailErr(t, err, crerrors.CodeNotOnStack)
}

func TestRunSubmit_DirtyWorktree(t *testing.T) {
	dir := initStackRepoWithRemote(t)
	if err := os.WriteFile(filepath.Join(dir, "dirty.txt"), []byte("d"), 0644); err != nil {
		t.Fatal(err)
	}
	mock := github.NewMock()
	mock.User = "bray"
	err := runSubmit(&bytes.Buffer{}, newRendererForTests(), newSubmitDeps(dir, mock))
	assertChainrailErr(t, err, crerrors.CodeDirtyWorktree)
}

func TestRunSubmit_PreservesUserBodyOutsideMarkers(t *testing.T) {
	dir := initStackRepoWithRemote(t)
	mock := github.NewMock()
	mock.User = "bray"

	if err := runSubmit(&bytes.Buffer{}, newRendererForTests(), newSubmitDeps(dir, mock)); err != nil {
		t.Fatal(err)
	}

	// Hand-edit one PR body with the user's own text outside markers.
	var schemaPR github.PullRequest
	for _, pr := range mock.PRs {
		if pr.HeadRefName == "bray/foo-1-schema" {
			schemaPR = pr
			break
		}
	}
	userText := "## My summary\n\nWhat this PR does.\n"
	newBody := userText + "\n" + schemaPR.Body
	if err := mock.UpdatePRBody(context.Background(), schemaPR.Number, newBody); err != nil {
		t.Fatal(err)
	}

	// Re-run submit; user text must survive.
	if err := runSubmit(&bytes.Buffer{}, newRendererForTests(), newSubmitDeps(dir, mock)); err != nil {
		t.Fatal(err)
	}
	got, _ := mock.GetPR(context.Background(), schemaPR.Number)
	if !strings.Contains(got.Body, "## My summary") {
		t.Fatalf("user-authored body content lost: %q", got.Body)
	}
}
