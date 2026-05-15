package cmd

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/brayschurman/chainrail/internal/github"
)

func TestStatusCommandIsRegistered(t *testing.T) {
	if !commandRegistered("status") {
		t.Fatal("status command not registered with root")
	}
}

// --- buildPRTree ---

func TestBuildPRTree_SimpleChain(t *testing.T) {
	prs := []github.PullRequest{
		{Number: 1, Title: "schema", HeadRefName: "bray/feat-1-schema", BaseRefName: "main", State: "OPEN"},
		{Number: 2, Title: "api", HeadRefName: "bray/feat-2-api", BaseRefName: "bray/feat-1-schema", State: "OPEN"},
		{Number: 3, Title: "ui", HeadRefName: "bray/feat-3-ui", BaseRefName: "bray/feat-2-api", State: "OPEN"},
	}
	layers := buildPRTree(prs, "bray/feat-2-api")

	if len(layers) != 3 {
		t.Fatalf("expected 3 layers, got %d", len(layers))
	}
	if layers[0].Depth != 0 || layers[1].Depth != 1 || layers[2].Depth != 2 {
		t.Errorf("depths wrong: %d %d %d", layers[0].Depth, layers[1].Depth, layers[2].Depth)
	}
	if !layers[1].IsCurrent {
		t.Error("layer 2 should be current")
	}
	if layers[0].Stack != "main" {
		t.Errorf("stack should be trunk 'main', got %q", layers[0].Stack)
	}
}

func TestBuildPRTree_TwoIndependentChains(t *testing.T) {
	prs := []github.PullRequest{
		{Number: 1, Title: "feat-a", HeadRefName: "bray/feat-a", BaseRefName: "main", State: "OPEN"},
		{Number: 2, Title: "feat-b", HeadRefName: "bray/feat-b", BaseRefName: "main", State: "OPEN"},
		{Number: 3, Title: "feat-b-2", HeadRefName: "bray/feat-b-2", BaseRefName: "bray/feat-b", State: "OPEN"},
	}
	layers := buildPRTree(prs, "")

	if len(layers) != 3 {
		t.Fatalf("expected 3 layers, got %d", len(layers))
	}
	// feat-a and feat-b are roots (depth 0), feat-b-2 is child of feat-b
	depths := []int{layers[0].Depth, layers[1].Depth, layers[2].Depth}
	if depths[0] != 0 || depths[1] != 0 || depths[2] != 1 {
		t.Errorf("unexpected depths: %v", depths)
	}
}

func TestBuildPRTree_Empty(t *testing.T) {
	layers := buildPRTree(nil, "main")
	if layers != nil {
		t.Errorf("expected nil for empty input, got %v", layers)
	}
}

// --- discoverAllStacks ---

func TestDiscoverAllStacks(t *testing.T) {
	branches := []string{
		"bray/feat-1-schema",
		"bray/feat-2-api",
		"bray/other-1-base",
		"main",
		"some-random-branch",
	}
	stacks := discoverAllStacks("bray", branches)
	if len(stacks) != 2 {
		t.Fatalf("expected 2 stacks, got %d: %v", len(stacks), stacks)
	}
	if len(stacks["feat"]) != 2 {
		t.Errorf("feat stack: expected 2 layers, got %v", stacks["feat"])
	}
	if stacks["feat"][0] != "bray/feat-1-schema" {
		t.Errorf("feat[0] = %q, want bray/feat-1-schema", stacks["feat"][0])
	}
}

// --- buildLayers ---

func TestBuildLayers_AllOpen(t *testing.T) {
	branches := []string{"bray/feat-1-schema", "bray/feat-2-api", "bray/feat-3-ui"}
	openByHead := map[string]github.PullRequest{
		"bray/feat-1-schema": {Number: 10, HeadRefName: "bray/feat-1-schema", State: "OPEN"},
		"bray/feat-2-api":    {Number: 11, HeadRefName: "bray/feat-2-api", State: "OPEN"},
		"bray/feat-3-ui":     {Number: 12, HeadRefName: "bray/feat-3-ui", State: "OPEN"},
	}
	layers := buildLayers(branches, "bray/feat-2-api", "main", "feat", openByHead, map[string]github.PullRequest{})

	if len(layers) != 3 {
		t.Fatalf("expected 3 layers, got %d", len(layers))
	}
	if !layers[1].IsCurrent {
		t.Error("layer 2 should be current")
	}
	for _, l := range layers {
		if l.NeedsSync {
			t.Errorf("no layer should need sync, but layer %d does", l.Position)
		}
	}
}

func TestBuildLayers_SquashedParent_NeedsSync(t *testing.T) {
	branches := []string{"bray/feat-1-schema", "bray/feat-2-api"}
	openByHead := map[string]github.PullRequest{
		"bray/feat-2-api": {Number: 11, HeadRefName: "bray/feat-2-api", State: "OPEN"},
	}
	mergedByHead := map[string]github.PullRequest{
		"bray/feat-1-schema": {Number: 10, HeadRefName: "bray/feat-1-schema", State: "MERGED", MergeCommitSHA: "abc123"},
	}

	layers := buildLayers(branches, "bray/feat-2-api", "main", "feat", openByHead, mergedByHead)

	if layers[0].PRState != "MERGED" {
		t.Errorf("layer 1 should be MERGED, got %q", layers[0].PRState)
	}
	if !layers[1].NeedsSync {
		t.Error("layer 2 should need sync")
	}
}

// --- runStatus integration ---

func TestRunStatus_JSON_FromStackBranch(t *testing.T) {
	dir := initStackRepoWithRemote(t)
	mock := github.NewMock()
	mock.User = "bray"
	mock.PRs[5] = github.PullRequest{Number: 5, HeadRefName: "bray/foo-1-schema", BaseRefName: "main", State: "OPEN"}
	mock.PRs[6] = github.PullRequest{Number: 6, HeadRefName: "bray/foo-2-api", BaseRefName: "bray/foo-1-schema", State: "OPEN"}

	var buf bytes.Buffer
	if err := runStatus(&buf, newRendererForTests(), true, false, statusDeps{cwd: dir, gh: mock}); err != nil {
		t.Fatalf("runStatus: %v", err)
	}

	var out statusOutput
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("JSON decode: %v\nraw: %s", err, buf.String())
	}
	if len(out.Layers) != 2 {
		t.Fatalf("expected 2 layers, got %d", len(out.Layers))
	}
	if !out.Layers[1].IsCurrent {
		t.Error("layer 2 should be current")
	}
}

func TestRunStatus_JSON_FromTrunk(t *testing.T) {
	dir := initStackRepoWithRemote(t)
	mustExec(t, dir, "git", "checkout", "main")

	mock := github.NewMock()
	mock.User = "bray"
	mock.PRs[5] = github.PullRequest{Number: 5, HeadRefName: "bray/foo-1-schema", State: "OPEN"}
	mock.PRs[6] = github.PullRequest{Number: 6, HeadRefName: "bray/foo-2-api", State: "OPEN"}

	var buf bytes.Buffer
	if err := runStatus(&buf, newRendererForTests(), true, false, statusDeps{cwd: dir, gh: mock}); err != nil {
		t.Fatalf("runStatus from trunk: %v", err)
	}

	var out statusOutput
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("JSON decode: %v\nraw: %s", err, buf.String())
	}
	if len(out.Layers) != 2 {
		t.Fatalf("expected 2 layers even from trunk, got %d", len(out.Layers))
	}
}

func TestRunStatus_NotInitialized_FriendlyMessage(t *testing.T) {
	// Repo with no chainrail init — should not error, should print friendly hint.
	dir := t.TempDir()
	mustExec(t, dir, "git", "init", "-b", "main")
	mustExec(t, dir, "git", "config", "user.email", "t@t.com")
	mustExec(t, dir, "git", "config", "user.name", "t")

	mock := github.NewMock()
	mock.User = "bray"

	var buf bytes.Buffer
	err := runStatus(&buf, newRendererForTests(), false, false, statusDeps{cwd: dir, gh: mock})
	if err != nil {
		t.Fatalf("expected no error for uninitialized repo, got: %v", err)
	}
}

func TestRunStatus_All_JSON(t *testing.T) {
	dir := t.TempDir()
	mustExec(t, dir, "git", "init", "-b", "main")
	mustExec(t, dir, "git", "config", "user.email", "t@t.com")
	mustExec(t, dir, "git", "config", "user.name", "t")

	mock := github.NewMock()
	mock.User = "bray"
	mock.PRs[1] = github.PullRequest{Number: 1, Title: "schema", HeadRefName: "bray/feat-1", BaseRefName: "main", State: "OPEN"}
	mock.PRs[2] = github.PullRequest{Number: 2, Title: "api", HeadRefName: "bray/feat-2", BaseRefName: "bray/feat-1", State: "OPEN"}

	var buf bytes.Buffer
	if err := runStatus(&buf, newRendererForTests(), true, true, statusDeps{cwd: dir, gh: mock}); err != nil {
		t.Fatalf("runStatus --all: %v", err)
	}

	var out statusOutput
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("JSON decode: %v\nraw: %s", err, buf.String())
	}
	if len(out.Layers) != 2 {
		t.Fatalf("expected 2 layers, got %d", len(out.Layers))
	}
	if out.Layers[0].Depth != 0 || out.Layers[1].Depth != 1 {
		t.Errorf("unexpected depths: %d %d", out.Layers[0].Depth, out.Layers[1].Depth)
	}
	if out.Layers[0].Stack != "main" {
		t.Errorf("root stack should be 'main', got %q", out.Layers[0].Stack)
	}
}
