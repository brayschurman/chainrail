package diffview

import (
	"regexp"
	"strings"
)

// PlanSeverity classifies how much structure / scope the PR description has.
// "Plan" here means: does the reviewer have enough context to scope a review,
// or is the PR a "just look at the diff" wall of agent output?
type PlanSeverity int

const (
	PlanPresent PlanSeverity = iota // structured body, multiple sections / bullets / >200 chars
	PlanThin                         // 50–200 chars, no structure — usable but minimal
	PlanMissing                      // empty or <50 chars
)

// PlanSignal captures the verdict on a PR body.
type PlanSignal struct {
	Severity   PlanSeverity
	Chars      int
	HasHeading bool
	HasBullets bool
	HasCode    bool
}

// DetectPlan inspects a PR body and returns a structured verdict.
func DetectPlan(body string) PlanSignal {
	body = strings.TrimSpace(body)
	sig := PlanSignal{Chars: len(body)}

	if hasHeadingRegex.MatchString(body) {
		sig.HasHeading = true
	}
	if hasBulletsRegex.MatchString(body) {
		sig.HasBullets = true
	}
	if strings.Contains(body, "```") {
		sig.HasCode = true
	}

	switch {
	case sig.Chars == 0:
		sig.Severity = PlanMissing
	case sig.HasHeading || sig.HasBullets:
		sig.Severity = PlanPresent
	case sig.Chars >= 200:
		sig.Severity = PlanPresent
	case sig.Chars < 50:
		sig.Severity = PlanMissing
	default:
		sig.Severity = PlanThin
	}
	return sig
}

// PlanNudgeMessage is the templated comment chainrail posts when the
// reviewer presses P on a PR with no plan. Copy-paste from the GitHub
// agent-PR review article.
const PlanNudgeMessage = `This PR is too large for me to review without a clearer implementation plan. ` +
	`Can you break it into smaller scoped units, or add a summary of what each part does and ` +
	`why it's structured this way? Happy to review after that.

(via [chainrail](https://github.com/brayschurman/chainrail))`

var (
	hasHeadingRegex = regexp.MustCompile(`(?m)^#{1,6}\s+\S`)
	hasBulletsRegex = regexp.MustCompile(`(?m)^(\s*)([-*]\s+\S|\d+\.\s+\S)`)
)
