package github

import (
	"context"
	"strings"
	"testing"
)

func TestMockGhClient_ImplementsInterface(t *testing.T) {
	var _ GitHubClient = (*MockGhClient)(nil)
}

func TestMockGhClient_CreatePR_ThenGetPR(t *testing.T) {
	m := NewMock()
	ctx := context.Background()
	pr, err := m.CreatePR(ctx, NewPR{
		Title: "fix",
		Body:  "fixes a thing",
		Head:  "feat/x",
		Base:  "main",
	})
	if err != nil {
		t.Fatal(err)
	}
	if pr.Number == 0 {
		t.Fatal("expected non-zero PR number")
	}
	if pr.State != "OPEN" {
		t.Fatalf("got state %q want OPEN", pr.State)
	}

	got, err := m.GetPR(ctx, pr.Number)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "fix" || got.HeadRefName != "feat/x" || got.BaseRefName != "main" {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
}

func TestMockGhClient_UpdatePRBody(t *testing.T) {
	m := NewMock()
	ctx := context.Background()
	pr, _ := m.CreatePR(ctx, NewPR{Title: "t", Body: "old", Head: "h", Base: "b"})

	if err := m.UpdatePRBody(ctx, pr.Number, "new body"); err != nil {
		t.Fatal(err)
	}
	got, _ := m.GetPR(ctx, pr.Number)
	if got.Body != "new body" {
		t.Fatalf("got body %q want %q", got.Body, "new body")
	}
}

func TestMockGhClient_UpdatePRBase(t *testing.T) {
	m := NewMock()
	ctx := context.Background()
	pr, _ := m.CreatePR(ctx, NewPR{Title: "t", Body: "b", Head: "h", Base: "old-base"})

	if err := m.UpdatePRBase(ctx, pr.Number, "main"); err != nil {
		t.Fatal(err)
	}
	got, _ := m.GetPR(ctx, pr.Number)
	if got.BaseRefName != "main" {
		t.Fatalf("got base %q want main", got.BaseRefName)
	}
}

func TestMockGhClient_ListOpenPRs_OnlyOpen(t *testing.T) {
	m := NewMock()
	ctx := context.Background()
	open1, _ := m.CreatePR(ctx, NewPR{Title: "a", Head: "h1", Base: "main"})
	closed, _ := m.CreatePR(ctx, NewPR{Title: "b", Head: "h2", Base: "main"})
	merged, _ := m.CreatePR(ctx, NewPR{Title: "c", Head: "h3", Base: "main"})

	m.SetState(closed.Number, "CLOSED")
	m.SetState(merged.Number, "MERGED")

	prs, err := m.ListOpenPRs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(prs) != 1 {
		t.Fatalf("got %d open PRs, want 1", len(prs))
	}
	if prs[0].Number != open1.Number {
		t.Fatalf("got PR %d want %d", prs[0].Number, open1.Number)
	}
}

func TestMockGhClient_CallsRecordedInOrder(t *testing.T) {
	m := NewMock()
	ctx := context.Background()
	_, _ = m.CurrentUser(ctx)
	pr, _ := m.CreatePR(ctx, NewPR{Title: "t", Head: "h", Base: "main"})
	_ = m.UpdatePRBody(ctx, pr.Number, "x")
	_ = m.UpdatePRBase(ctx, pr.Number, "y")
	_, _ = m.GetPR(ctx, pr.Number)
	_, _ = m.ListOpenPRs(ctx)

	wantPrefixes := []string{"CurrentUser", "CreatePR", "UpdatePRBody", "UpdatePRBase", "GetPR", "ListOpenPRs"}
	if len(m.Calls) != len(wantPrefixes) {
		t.Fatalf("got %d calls, want %d: %v", len(m.Calls), len(wantPrefixes), m.Calls)
	}
	for i, want := range wantPrefixes {
		if !strings.HasPrefix(m.Calls[i], want) {
			t.Fatalf("call %d: got %q, want prefix %q", i, m.Calls[i], want)
		}
	}
}

func TestMockGhClient_CurrentUserDefault(t *testing.T) {
	m := NewMock()
	user, err := m.CurrentUser(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if user == "" {
		t.Fatal("expected a default user")
	}
}

func TestMockGhClient_GetPR_NotFound(t *testing.T) {
	m := NewMock()
	_, err := m.GetPR(context.Background(), 999)
	if err == nil {
		t.Fatal("expected error for unknown PR")
	}
}
