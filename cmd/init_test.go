package cmd

import (
	"bytes"
	stdlibErrors "errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	crerrors "github.com/brayschurman/chainrail/internal/errors"
	"github.com/brayschurman/chainrail/internal/output"
)

func TestInitCommandIsRegistered(t *testing.T) {
	if !commandRegistered("init") {
		t.Fatal("init command not registered with root")
	}
}

func TestInitCommand_MissingBaseFlag(t *testing.T) {
	rootCmd.SetArgs([]string{"init"})
	var out, errOut bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&errOut)
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when --base is missing")
	}
}

func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	mustExec(t, dir, "git", "init", "-b", "main")
	mustExec(t, dir, "git", "config", "user.email", "test@test.com")
	mustExec(t, dir, "git", "config", "user.name", "test")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hi\n"), 0644); err != nil {
		t.Fatal(err)
	}
	mustExec(t, dir, "git", "add", ".")
	mustExec(t, dir, "git", "commit", "-m", "init")
	return dir
}

func mustExec(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %s: %v\n%s", name, strings.Join(args, " "), err, out)
	}
}

func authOK() error  { return nil }
func authBad() error { return stdlibErrors.New("not authenticated") }

func newRendererForTests() output.Renderer {
	return output.NewTextRenderer(false)
}

func assertChainrailErr(t *testing.T, err error, wantCode string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error with code %s, got nil", wantCode)
	}
	var ce *crerrors.ChainrailError
	if !stdlibErrors.As(err, &ce) {
		t.Fatalf("expected *ChainrailError, got %T: %v", err, err)
	}
	if ce.Code != wantCode {
		t.Fatalf("got code %q want %q", ce.Code, wantCode)
	}
}

func TestRunInit_NotInGitRepo(t *testing.T) {
	dir := t.TempDir()
	var out bytes.Buffer
	err := runInit(&out, newRendererForTests(), "main", initDeps{cwd: dir, checkAuth: authOK})
	assertChainrailErr(t, err, crerrors.CodeNotGitRepo)
}

func TestRunInit_GhAuthFails(t *testing.T) {
	dir := initTestRepo(t)
	var out bytes.Buffer
	err := runInit(&out, newRendererForTests(), "main", initDeps{cwd: dir, checkAuth: authBad})
	assertChainrailErr(t, err, crerrors.CodeNoGhAuth)
}

func TestRunInit_TrunkMissing(t *testing.T) {
	dir := initTestRepo(t)
	var out bytes.Buffer
	err := runInit(&out, newRendererForTests(), "nope", initDeps{cwd: dir, checkAuth: authOK})
	assertChainrailErr(t, err, crerrors.CodeTrunkMissing)
}

func TestRunInit_Success(t *testing.T) {
	dir := initTestRepo(t)
	var out bytes.Buffer
	err := runInit(&out, newRendererForTests(), "main", initDeps{cwd: dir, checkAuth: authOK})
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "config", "--get", "chainrail.trunk")
	cmd.Dir = dir
	got, _ := cmd.Output()
	if strings.TrimSpace(string(got)) != "main" {
		t.Fatalf("expected chainrail.trunk=main, got %q", string(got))
	}
	if !strings.Contains(out.String(), "main") {
		t.Fatalf("expected success message to mention main: %q", out.String())
	}
}

func TestRunInit_IdempotentSameBase(t *testing.T) {
	dir := initTestRepo(t)
	var out1, out2 bytes.Buffer
	if err := runInit(&out1, newRendererForTests(), "main", initDeps{cwd: dir, checkAuth: authOK}); err != nil {
		t.Fatal(err)
	}
	err := runInit(&out2, newRendererForTests(), "main", initDeps{cwd: dir, checkAuth: authOK})
	if err != nil {
		t.Fatalf("second init with same base should succeed, got %v", err)
	}
	if !strings.Contains(strings.ToLower(out2.String()), "already") {
		t.Fatalf("expected 'already' wording on idempotent run: %q", out2.String())
	}
}

func TestRunInit_AlreadyInitDifferentBase(t *testing.T) {
	dir := initTestRepo(t)
	mustExec(t, dir, "git", "branch", "develop", "main")
	var out bytes.Buffer
	if err := runInit(&out, newRendererForTests(), "main", initDeps{cwd: dir, checkAuth: authOK}); err != nil {
		t.Fatal(err)
	}
	err := runInit(&bytes.Buffer{}, newRendererForTests(), "develop", initDeps{cwd: dir, checkAuth: authOK})
	assertChainrailErr(t, err, crerrors.CodeAlreadyInit)
}
