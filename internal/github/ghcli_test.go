package github

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func newFakeCli(t *testing.T, handlers map[string][]byte) *GhCli {
	t.Helper()
	return &GhCli{run: func(name string, args ...string) ([]byte, error) {
		key := name + " " + strings.Join(args, " ")
		if out, ok := handlers[key]; ok {
			return out, nil
		}
		for k, out := range handlers {
			if strings.HasPrefix(key, k) {
				return out, nil
			}
		}
		return nil, errors.New("unexpected call: " + key)
	}}
}

func TestGhCli_CurrentUser(t *testing.T) {
	cli := newFakeCli(t, map[string][]byte{
		"gh api user --jq .login": []byte("brayschurman\n"),
	})
	got, err := cli.CurrentUser(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got != "brayschurman" {
		t.Fatalf("got %q want brayschurman", got)
	}
}

func TestGhCli_ListOpenPRs_Empty(t *testing.T) {
	cli := newFakeCli(t, map[string][]byte{
		"gh pr list": []byte("[]"),
	})
	prs, err := cli.ListOpenPRs(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(prs) != 0 {
		t.Fatalf("expected empty, got %v", prs)
	}
}

func TestGhCli_ListOpenPRs_Multiple(t *testing.T) {
	cli := newFakeCli(t, map[string][]byte{
		"gh pr list": []byte(`[
{"number":712,"title":"schema","baseRefName":"main","headRefName":"feat/schema","state":"OPEN","body":"b1","mergeCommit":null},
{"number":713,"title":"api","baseRefName":"feat/schema","headRefName":"feat/api","state":"OPEN","body":"b2","mergeCommit":null}
]`),
	})
	prs, err := cli.ListOpenPRs(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(prs) != 2 {
		t.Fatalf("got %d prs, want 2", len(prs))
	}
	if prs[0].Number != 712 || prs[0].HeadRefName != "feat/schema" {
		t.Fatalf("pr0 mismatch: %+v", prs[0])
	}
	if prs[1].BaseRefName != "feat/schema" {
		t.Fatalf("pr1 base mismatch: %+v", prs[1])
	}
}

func TestGhCli_GetPR_MergedWithSha(t *testing.T) {
	cli := newFakeCli(t, map[string][]byte{
		"gh pr view 100": []byte(`{"number":100,"title":"t","baseRefName":"main","headRefName":"feat/x","state":"MERGED","body":"b","mergeCommit":{"oid":"abc123"}}`),
	})
	pr, err := cli.GetPR(context.Background(), 100)
	if err != nil {
		t.Fatal(err)
	}
	if pr.State != "MERGED" || pr.MergeCommitSHA != "abc123" {
		t.Fatalf("got %+v", pr)
	}
}

func TestGhCli_GetPR_NotMerged_EmptySha(t *testing.T) {
	cli := newFakeCli(t, map[string][]byte{
		"gh pr view 101": []byte(`{"number":101,"title":"t","baseRefName":"main","headRefName":"feat/y","state":"OPEN","body":"b","mergeCommit":null}`),
	})
	pr, err := cli.GetPR(context.Background(), 101)
	if err != nil {
		t.Fatal(err)
	}
	if pr.MergeCommitSHA != "" {
		t.Fatalf("expected empty MergeCommitSHA, got %q", pr.MergeCommitSHA)
	}
}

func TestGhCli_CreatePR(t *testing.T) {
	created := false
	cli := &GhCli{run: func(name string, args ...string) ([]byte, error) {
		key := name + " " + strings.Join(args, " ")
		if strings.HasPrefix(key, "gh pr create") {
			created = true
			return []byte("https://github.com/o/r/pull/200\n"), nil
		}
		if strings.HasPrefix(key, "gh pr view 200") {
			return []byte(`{"number":200,"title":"new","baseRefName":"main","headRefName":"feat/new","state":"OPEN","body":"body","mergeCommit":null}`), nil
		}
		return nil, errors.New("unexpected: " + key)
	}}

	pr, err := cli.CreatePR(context.Background(), NewPR{
		Title: "new",
		Body:  "body",
		Head:  "feat/new",
		Base:  "main",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !created {
		t.Fatal("CreatePR did not invoke gh pr create")
	}
	if pr.Number != 200 {
		t.Fatalf("got %+v", pr)
	}
}

func TestGhCli_ListOpenPRs_ParsesCIAndReview(t *testing.T) {
	cli := newFakeCli(t, map[string][]byte{
		"gh pr list": []byte(`[
{"number":1,"title":"a","baseRefName":"main","headRefName":"feat/a","state":"OPEN","body":"","mergeCommit":null,"statusCheckRollup":[{"status":"COMPLETED","conclusion":"SUCCESS"}],"reviewDecision":"APPROVED","updatedAt":"2026-05-14T10:00:00Z"},
{"number":2,"title":"b","baseRefName":"main","headRefName":"feat/b","state":"OPEN","body":"","mergeCommit":null,"statusCheckRollup":[{"status":"COMPLETED","conclusion":"FAILURE"},{"status":"COMPLETED","conclusion":"SUCCESS"}],"reviewDecision":"REVIEW_REQUIRED","updatedAt":"2026-05-13T10:00:00Z"},
{"number":3,"title":"c","baseRefName":"main","headRefName":"feat/c","state":"OPEN","body":"","mergeCommit":null,"statusCheckRollup":[{"status":"IN_PROGRESS","conclusion":""}],"reviewDecision":"CHANGES_REQUESTED","updatedAt":"2026-05-12T10:00:00Z"},
{"number":4,"title":"d","baseRefName":"main","headRefName":"feat/d","state":"OPEN","body":"","mergeCommit":null,"statusCheckRollup":[],"reviewDecision":"","updatedAt":""}
]`),
	})
	prs, err := cli.ListOpenPRs(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(prs) != 4 {
		t.Fatalf("got %d prs, want 4", len(prs))
	}
	cases := []struct {
		num    int
		ci     string
		review string
	}{
		{1, "SUCCESS", "APPROVED"},
		{2, "FAILURE", "REVIEW_REQUIRED"},
		{3, "PENDING", "CHANGES_REQUESTED"},
		{4, "", ""},
	}
	for i, c := range cases {
		if prs[i].CIStatus != c.ci {
			t.Errorf("pr %d: CIStatus = %q, want %q", c.num, prs[i].CIStatus, c.ci)
		}
		if prs[i].ReviewDecision != c.review {
			t.Errorf("pr %d: ReviewDecision = %q, want %q", c.num, prs[i].ReviewDecision, c.review)
		}
	}
	if prs[0].UpdatedAt != "2026-05-14T10:00:00Z" {
		t.Errorf("UpdatedAt: got %q", prs[0].UpdatedAt)
	}
}

func TestGhCli_ChangesSinceReview_CountsCommitsAfterUserReview(t *testing.T) {
	cli := newFakeCli(t, map[string][]byte{
		"gh api user --jq .login": []byte("brayschurman\n"),
		"gh pr list --search reviewed-by:@me is:open --limit 50 --json number": []byte(`[
{"number":10},{"number":11},{"number":12}
]`),
		// PR 10: user reviewed at T1, 2 commits after, 1 before -> 2
		"gh pr view 10 --json reviews,commits": []byte(`{
"reviews":[
  {"author":{"login":"someone-else"},"submittedAt":"2026-05-13T18:00:00Z"},
  {"author":{"login":"brayschurman"},"submittedAt":"2026-05-13T10:00:00Z"}
],
"commits":[
  {"committedDate":"2026-05-12T09:00:00Z"},
  {"committedDate":"2026-05-13T11:00:00Z"},
  {"committedDate":"2026-05-13T20:00:00Z"}
]
}`),
		// PR 11: user reviewed at T2, no commits after -> absent from map
		"gh pr view 11 --json reviews,commits": []byte(`{
"reviews":[{"author":{"login":"brayschurman"},"submittedAt":"2026-05-14T00:00:00Z"}],
"commits":[{"committedDate":"2026-05-13T11:00:00Z"}]
}`),
		// PR 12: only someone else reviewed -> absent
		"gh pr view 12 --json reviews,commits": []byte(`{
"reviews":[{"author":{"login":"someone-else"},"submittedAt":"2026-05-13T18:00:00Z"}],
"commits":[{"committedDate":"2026-05-13T20:00:00Z"}]
}`),
	})

	got, err := cli.ChangesSinceReview(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected exactly 1 PR with changes, got %v", got)
	}
	if got[10] != 2 {
		t.Errorf("PR 10: got %d, want 2", got[10])
	}
	if _, ok := got[11]; ok {
		t.Error("PR 11 should be absent — no commits since user's review")
	}
	if _, ok := got[12]; ok {
		t.Error("PR 12 should be absent — user has not reviewed")
	}
}

func TestGhCli_UpdatePRTitle(t *testing.T) {
	var got []string
	cli := &GhCli{run: func(name string, args ...string) ([]byte, error) {
		got = append([]string{name}, args...)
		return nil, nil
	}}
	if err := cli.UpdatePRTitle(context.Background(), 42, "polished title"); err != nil {
		t.Fatal(err)
	}
	want := []string{"gh", "pr", "edit", "42", "--title", "polished title"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestGhCli_UpdatePRBody(t *testing.T) {
	var got []string
	cli := &GhCli{run: func(name string, args ...string) ([]byte, error) {
		got = append([]string{name}, args...)
		return nil, nil
	}}
	if err := cli.UpdatePRBody(context.Background(), 99, "new body"); err != nil {
		t.Fatal(err)
	}
	want := []string{"gh", "pr", "edit", "99", "--body", "new body"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestGhCli_UpdatePRBase(t *testing.T) {
	var got []string
	cli := &GhCli{run: func(name string, args ...string) ([]byte, error) {
		got = append([]string{name}, args...)
		return nil, nil
	}}
	if err := cli.UpdatePRBase(context.Background(), 99, "main"); err != nil {
		t.Fatal(err)
	}
	want := []string{"gh", "pr", "edit", "99", "--base", "main"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestGhCli_RunnerError_BecomesGhCallFailed(t *testing.T) {
	cli := &GhCli{run: func(name string, args ...string) ([]byte, error) {
		return nil, errors.New("gh not authenticated")
	}}
	_, err := cli.CurrentUser(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "gh") {
		t.Fatalf("error should mention gh: %v", err)
	}
}

func TestGhCli_ImplementsInterface(t *testing.T) {
	var _ GitHubClient = (*GhCli)(nil)
}
