package stack

import (
	"fmt"

	"github.com/brayschurman/chainrail/internal/github"
)

type Layer struct {
	Branch string
	PR     *github.PullRequest
	Parent string
}

// Walk returns the chain of branches from the bottom of the stack (closest to
// trunk) up to and including currentBranch. The first element's Parent is
// empty — meaning its base is the trunk.
//
// v0.1 walks the GitHub PR graph: every branch in the chain must have a PR for
// it to appear in the result. If currentBranch has no PR, or any intermediate
// parent has no PR before reaching trunk, the function returns an empty slice
// (not an error) — callers should treat that as "not currently in a stack".
//
// localBranches is reserved for v0.2 richer-validation use (e.g. rejecting
// chains rooted on a foreign user's branch) and is unused by the v0.1 algorithm.
func Walk(currentBranch, trunk string, prs []github.PullRequest, localBranches map[string]bool) ([]Layer, error) {
	_ = localBranches
	if currentBranch == trunk {
		return nil, nil
	}

	byHead := make(map[string]github.PullRequest, len(prs))
	for _, pr := range prs {
		byHead[pr.HeadRefName] = pr
	}

	var chain []Layer
	branch := currentBranch
	seen := map[string]bool{}
	for {
		if seen[branch] {
			return nil, fmt.Errorf("cycle detected at %q while walking stack", branch)
		}
		seen[branch] = true

		pr, ok := byHead[branch]
		if !ok {
			return nil, nil
		}
		prCopy := pr
		parent := ""
		if pr.BaseRefName != trunk {
			parent = pr.BaseRefName
		}
		chain = append(chain, Layer{Branch: branch, PR: &prCopy, Parent: parent})
		if pr.BaseRefName == trunk {
			break
		}
		branch = pr.BaseRefName
	}

	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}
	return chain, nil
}
