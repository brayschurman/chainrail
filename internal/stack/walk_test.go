package stack

import (
	"testing"

	"github.com/brayschurman/chainrail/internal/github"
)

func TestWalk_CurrentIsTrunk_Empty(t *testing.T) {
	got, err := Walk("main", "main", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty, got %v", got)
	}
}

func TestWalk_OnePRStack(t *testing.T) {
	prs := []github.PullRequest{
		{Number: 1, HeadRefName: "feat/x", BaseRefName: "main", State: "OPEN"},
	}
	got, err := Walk("feat/x", "main", prs, map[string]bool{"feat/x": true})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 layer, got %d", len(got))
	}
	if got[0].Branch != "feat/x" {
		t.Fatalf("got branch %q", got[0].Branch)
	}
	if got[0].PR == nil || got[0].PR.Number != 1 {
		t.Fatalf("PR not attached correctly: %+v", got[0].PR)
	}
	if got[0].Parent != "" {
		t.Fatalf("expected empty Parent (trunk-rooted), got %q", got[0].Parent)
	}
}

func TestWalk_ThreePRStack_BottomUp(t *testing.T) {
	prs := []github.PullRequest{
		{Number: 100, HeadRefName: "feat/a", BaseRefName: "main"},
		{Number: 101, HeadRefName: "feat/b", BaseRefName: "feat/a"},
		{Number: 102, HeadRefName: "feat/c", BaseRefName: "feat/b"},
	}
	got, err := Walk("feat/c", "main", prs, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 layers, got %d", len(got))
	}
	wantBranches := []string{"feat/a", "feat/b", "feat/c"}
	wantParents := []string{"", "feat/a", "feat/b"}
	for i, layer := range got {
		if layer.Branch != wantBranches[i] {
			t.Fatalf("layer %d: branch=%q want %q", i, layer.Branch, wantBranches[i])
		}
		if layer.Parent != wantParents[i] {
			t.Fatalf("layer %d: parent=%q want %q", i, layer.Parent, wantParents[i])
		}
	}
}

func TestWalk_IgnoresUnrelatedPRs(t *testing.T) {
	prs := []github.PullRequest{
		{Number: 200, HeadRefName: "feat/a", BaseRefName: "main"},
		{Number: 201, HeadRefName: "feat/b", BaseRefName: "feat/a"},
		// unrelated: open PR on a different branch
		{Number: 999, HeadRefName: "other/thing", BaseRefName: "main"},
	}
	got, err := Walk("feat/b", "main", prs, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 layers, got %d", len(got))
	}
	for _, layer := range got {
		if layer.Branch == "other/thing" {
			t.Fatal("unrelated branch leaked into the chain")
		}
	}
}

func TestWalk_NoPRForCurrent_Empty(t *testing.T) {
	prs := []github.PullRequest{
		{Number: 300, HeadRefName: "other/x", BaseRefName: "main"},
	}
	got, err := Walk("feat/unpublished", "main", prs, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty when current branch has no PR, got %v", got)
	}
}

func TestWalk_BrokenChain_Empty(t *testing.T) {
	// feat/c has a PR but its parent feat/b does NOT — chain is broken.
	prs := []github.PullRequest{
		{Number: 400, HeadRefName: "feat/c", BaseRefName: "feat/b"},
	}
	got, err := Walk("feat/c", "main", prs, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty for broken chain, got %v", got)
	}
}

func TestWalk_CycleDetected_Errors(t *testing.T) {
	prs := []github.PullRequest{
		{Number: 500, HeadRefName: "a", BaseRefName: "b"},
		{Number: 501, HeadRefName: "b", BaseRefName: "a"},
	}
	_, err := Walk("a", "main", prs, nil)
	if err == nil {
		t.Fatal("expected cycle error")
	}
}
