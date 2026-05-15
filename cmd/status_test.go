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

func TestBuildLayers_AllOpen(t *testing.T) {
	branches := []string{"bray/feat-1-schema", "bray/feat-2-api", "bray/feat-3-ui"}
	openByHead := map[string]github.PullRequest{
		"bray/feat-1-schema": {Number: 10, HeadRefName: "bray/feat-1-schema", State: "OPEN"},
		"bray/feat-2-api":    {Number: 11, HeadRefName: "bray/feat-2-api", State: "OPEN"},
		"bray/feat-3-ui":     {Number: 12, HeadRefName: "bray/feat-3-ui", State: "OPEN"},
	}
	mergedByHead := map[string]github.PullRequest{}

	layers := buildLayers(branches, "bray/feat-2-api", "main", openByHead, mergedByHead)

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
	if layers[0].PRNumber != 10 || layers[0].PRState != "OPEN" {
		t.Errorf("layer 1 PR unexpected: %+v", layers[0])
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

	layers := buildLayers(branches, "bray/feat-2-api", "main", openByHead, mergedByHead)

	if layers[0].PRState != "MERGED" {
		t.Errorf("layer 1 should be MERGED, got %q", layers[0].PRState)
	}
	if !layers[1].NeedsSync {
		t.Error("layer 2 should need sync (parent was squash-merged)")
	}
}

func TestRunStatus_JSON(t *testing.T) {
	dir := initStackRepoWithRemote(t) // 2-layer stack: bray/foo-1-schema, bray/foo-2-api
	mock := github.NewMock()
	mock.User = "bray"
	// seed PRs so ListOpenPRs returns them
	mock.PRs[5] = github.PullRequest{Number: 5, HeadRefName: "bray/foo-1-schema", BaseRefName: "main", State: "OPEN"}
	mock.PRs[6] = github.PullRequest{Number: 6, HeadRefName: "bray/foo-2-api", BaseRefName: "bray/foo-1-schema", State: "OPEN"}

	var buf bytes.Buffer
	if err := runStatus(&buf, true, statusDeps{cwd: dir, gh: mock}); err != nil {
		t.Fatalf("runStatus: %v", err)
	}

	var out struct {
		Stack  string      `json:"stack"`
		Trunk  string      `json:"trunk"`
		Layers []tui.Layer `json:"layers"`
	}
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("JSON decode: %v\nraw: %s", err, buf.String())
	}
	if out.Stack != "foo" {
		t.Errorf("stack = %q, want %q", out.Stack, "foo")
	}
	if len(out.Layers) != 2 {
		t.Fatalf("expected 2 layers, got %d", len(out.Layers))
	}
	if !out.Layers[1].IsCurrent {
		t.Error("layer 2 should be current (bray/foo-2-api checked out)")
	}
	if out.Layers[0].PRNumber != 5 {
		t.Errorf("layer 1 PR number = %d, want 5", out.Layers[0].PRNumber)
	}
}
