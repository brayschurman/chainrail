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
