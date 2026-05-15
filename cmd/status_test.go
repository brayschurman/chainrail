package cmd

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/brayschurman/chainrail/internal/github"
	"github.com/brayschurman/chainrail/internal/tui"
)

func TestStatusCommandIsRegistered(t *testing.T) {
	if !commandRegistered("status") {
		t.Fatal("status command not registered with root")
	}
}

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
	if len(stacks["other"]) != 1 {
		t.Errorf("other stack: expected 1 layer, got %v", stacks["other"])
	}
}

func TestBuildLayers_AllOpen(t *testing.T) {
	branches := []string{"bray/feat-1-schema", "bray/feat-2-api", "bray/feat-3-ui"}
	openByHead := map[string]github.PullRequest{
		"bray/feat-1-schema": {Number: 10, HeadRefName: "bray/feat-1-schema", State: "OPEN"},
		"bray/feat-2-api":    {Number: 11, HeadRefName: "bray/feat-2-api", State: "OPEN"},
		"bray/feat-3-ui":     {Number: 12, HeadRefName: "bray/feat-3-ui", State: "OPEN"},
	}
	mergedByHead := map[string]github.PullRequest{}

	layers := buildLayers(branches, "bray/feat-2-api", "main", "feat", openByHead, mergedByHead)

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
		t.Error("layer 2 should need sync (parent was squash-merged)")
	}
}

func TestRunStatus_JSON_FromStackBranch(t *testing.T) {
	dir := initStackRepoWithRemote(t) // ends on bray/foo-2-api
	mock := github.NewMock()
	mock.User = "bray"
	mock.PRs[5] = github.PullRequest{Number: 5, HeadRefName: "bray/foo-1-schema", BaseRefName: "main", State: "OPEN"}
	mock.PRs[6] = github.PullRequest{Number: 6, HeadRefName: "bray/foo-2-api", BaseRefName: "bray/foo-1-schema", State: "OPEN"}

	var buf bytes.Buffer
	if err := runStatus(&buf, newRendererForTests(), true, statusDeps{cwd: dir, gh: mock}); err != nil {
		t.Fatalf("runStatus: %v", err)
	}

	var out struct {
		Layers []tui.Layer `json:"layers"`
	}
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
	dir := initStackRepoWithRemote(t) // ends on bray/foo-2-api
	// switch to trunk — status should still work
	mustExec(t, dir, "git", "checkout", "main")

	mock := github.NewMock()
	mock.User = "bray"
	mock.PRs[5] = github.PullRequest{Number: 5, HeadRefName: "bray/foo-1-schema", State: "OPEN"}
	mock.PRs[6] = github.PullRequest{Number: 6, HeadRefName: "bray/foo-2-api", State: "OPEN"}

	var buf bytes.Buffer
	if err := runStatus(&buf, newRendererForTests(), true, statusDeps{cwd: dir, gh: mock}); err != nil {
		t.Fatalf("runStatus from trunk: %v", err)
	}

	var out struct {
		Layers []tui.Layer `json:"layers"`
	}
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("JSON decode: %v\nraw: %s", err, buf.String())
	}
	if len(out.Layers) != 2 {
		t.Fatalf("expected 2 layers even from trunk, got %d", len(out.Layers))
	}
	for _, l := range out.Layers {
		if l.IsCurrent {
			t.Error("no layer should be current when on trunk")
		}
	}
}
