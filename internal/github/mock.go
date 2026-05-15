package github

import (
	"context"
	"fmt"
)

type MockGhClient struct {
	User               string
	PRs                map[int]PullRequest
	NextNum            int
	Calls              []string
	reviewRequested    map[int]bool
	changesSinceReview map[int]int
	prDiffs            map[int]string
}

func NewMock() *MockGhClient {
	return &MockGhClient{
		User:    "test-user",
		PRs:     map[int]PullRequest{},
		NextNum: 100,
	}
}

func (m *MockGhClient) record(call string) {
	m.Calls = append(m.Calls, call)
}

func (m *MockGhClient) SetState(number int, state string) {
	pr, ok := m.PRs[number]
	if !ok {
		return
	}
	pr.State = state
	m.PRs[number] = pr
}

func (m *MockGhClient) CurrentUser(_ context.Context) (string, error) {
	m.record("CurrentUser")
	return m.User, nil
}

func (m *MockGhClient) ListOpenPRs(_ context.Context) ([]PullRequest, error) {
	m.record("ListOpenPRs")
	out := make([]PullRequest, 0, len(m.PRs))
	for _, pr := range m.PRs {
		if pr.State == "OPEN" {
			out = append(out, pr)
		}
	}
	return out, nil
}

func (m *MockGhClient) ListAllOpenPRs(_ context.Context) ([]PullRequest, error) {
	m.record("ListAllOpenPRs")
	out := make([]PullRequest, 0, len(m.PRs))
	for _, pr := range m.PRs {
		if pr.State == "OPEN" {
			out = append(out, pr)
		}
	}
	return out, nil
}

// ReviewRequested holds PR numbers where the mock user is a requested reviewer.
func (m *MockGhClient) SetReviewRequested(numbers ...int) {
	m.reviewRequested = map[int]bool{}
	for _, n := range numbers {
		m.reviewRequested[n] = true
	}
}

func (m *MockGhClient) ListReviewRequestedPRs(_ context.Context) ([]PullRequest, error) {
	m.record("ListReviewRequestedPRs")
	out := make([]PullRequest, 0, len(m.PRs))
	for num, pr := range m.PRs {
		if pr.State == "OPEN" && m.reviewRequested[num] {
			out = append(out, pr)
		}
	}
	return out, nil
}

func (m *MockGhClient) SetChangesSinceReview(byPR map[int]int) {
	m.changesSinceReview = byPR
}

func (m *MockGhClient) ChangesSinceReview(_ context.Context) (map[int]int, error) {
	m.record("ChangesSinceReview")
	out := make(map[int]int, len(m.changesSinceReview))
	for k, v := range m.changesSinceReview {
		out[k] = v
	}
	return out, nil
}

func (m *MockGhClient) ListMergedPRsByHead(_ context.Context, heads []string) ([]PullRequest, error) {
	m.record(fmt.Sprintf("ListMergedPRsByHead(%v)", heads))
	headSet := make(map[string]bool, len(heads))
	for _, h := range heads {
		headSet[h] = true
	}
	type entry struct {
		num int
		pr  PullRequest
	}
	byHead := map[string]entry{}
	for num, pr := range m.PRs {
		if pr.State != "MERGED" {
			continue
		}
		if !headSet[pr.HeadRefName] {
			continue
		}
		if existing, ok := byHead[pr.HeadRefName]; !ok || num > existing.num {
			byHead[pr.HeadRefName] = entry{num: num, pr: pr}
		}
	}
	out := make([]PullRequest, 0, len(byHead))
	for _, e := range byHead {
		out = append(out, e.pr)
	}
	return out, nil
}

func (m *MockGhClient) GetPR(_ context.Context, number int) (PullRequest, error) {
	m.record(fmt.Sprintf("GetPR(%d)", number))
	pr, ok := m.PRs[number]
	if !ok {
		return PullRequest{}, fmt.Errorf("mock: PR #%d not found", number)
	}
	return pr, nil
}

func (m *MockGhClient) CreatePR(_ context.Context, p NewPR) (PullRequest, error) {
	m.record(fmt.Sprintf("CreatePR(head=%s,base=%s)", p.Head, p.Base))
	num := m.NextNum
	m.NextNum++
	pr := PullRequest{
		Number:      num,
		Title:       p.Title,
		Body:        p.Body,
		HeadRefName: p.Head,
		BaseRefName: p.Base,
		State:       "OPEN",
	}
	m.PRs[num] = pr
	return pr, nil
}

func (m *MockGhClient) UpdatePRBody(_ context.Context, number int, body string) error {
	m.record(fmt.Sprintf("UpdatePRBody(%d)", number))
	pr, ok := m.PRs[number]
	if !ok {
		return fmt.Errorf("mock: PR #%d not found", number)
	}
	pr.Body = body
	m.PRs[number] = pr
	return nil
}

// PRDiffs lets tests inject fixture diffs per PR number.
func (m *MockGhClient) SetPRDiff(number int, diff string) {
	if m.prDiffs == nil {
		m.prDiffs = map[int]string{}
	}
	m.prDiffs[number] = diff
}

func (m *MockGhClient) PRDiff(_ context.Context, number int) (string, error) {
	m.record(fmt.Sprintf("PRDiff(%d)", number))
	if d, ok := m.prDiffs[number]; ok {
		return d, nil
	}
	return "", fmt.Errorf("mock: no diff for PR #%d", number)
}

func (m *MockGhClient) UpdatePRTitle(_ context.Context, number int, newTitle string) error {
	m.record(fmt.Sprintf("UpdatePRTitle(%d,%s)", number, newTitle))
	pr, ok := m.PRs[number]
	if !ok {
		return fmt.Errorf("mock: PR #%d not found", number)
	}
	pr.Title = newTitle
	m.PRs[number] = pr
	return nil
}

func (m *MockGhClient) UpdatePRBase(_ context.Context, number int, newBase string) error {
	m.record(fmt.Sprintf("UpdatePRBase(%d,%s)", number, newBase))
	pr, ok := m.PRs[number]
	if !ok {
		return fmt.Errorf("mock: PR #%d not found", number)
	}
	pr.BaseRefName = newBase
	m.PRs[number] = pr
	return nil
}
