package diffview

import (
	"regexp"
	"strings"
)

// CIRisk classifies how a file affects CI confidence. Used to pin risky
// files to the top of the sidebar and to gate 100% progress.
type CIRisk int

const (
	CIRiskNone CIRisk = iota
	CIRiskConfig          // CI/test/coverage config touched but no obvious weakening
	CIRiskWeakening       // explicit weakening signal in the diff
)

// CISignal is a per-file detection result.
type CISignal struct {
	Risk    CIRisk
	Reasons []string // human-readable summary, e.g. "test marked .skip" or "continue-on-error added"
}

// IsCIRelated reports whether a path is one of the file kinds whose changes
// affect CI behavior (workflows, test/build configs, coverage).
func IsCIRelated(path string) bool {
	return ciPathRegex.MatchString(path)
}

// DetectCIRisk inspects a file's path and diff body for CI-related changes
// and returns a CISignal. Returns CIRiskNone for files that don't match any
// CI-relevant path pattern.
//
// Content-level weakening signals are only inspected on workflow files (where
// they're unambiguous). For other config files we conservatively report
// CIRiskConfig — the reviewer's judgment is the gate.
func DetectCIRisk(path string, lines []Line) CISignal {
	if !IsCIRelated(path) {
		return CISignal{Risk: CIRiskNone}
	}
	signal := CISignal{Risk: CIRiskConfig}

	// Walk additions only — a weakening signal that appears in a removed
	// line means the weakening was *removed*, which is the opposite.
	for _, l := range lines {
		if l.Kind != LineAdd {
			continue
		}
		text := stripMarker(l.Text)
		trimmed := strings.TrimSpace(text)
		switch {
		case skipPatternRegex.MatchString(trimmed):
			signal.Risk = CIRiskWeakening
			signal.Reasons = appendUnique(signal.Reasons, "test skipped (.skip / xit / xdescribe)")
		case strings.Contains(trimmed, "continue-on-error: true"):
			signal.Risk = CIRiskWeakening
			signal.Reasons = appendUnique(signal.Reasons, "continue-on-error: true added")
		case strings.Contains(trimmed, "|| true"):
			signal.Risk = CIRiskWeakening
			signal.Reasons = appendUnique(signal.Reasons, "command suppressed with || true")
		case strings.HasPrefix(trimmed, "if: false") || strings.Contains(trimmed, "if: false"):
			signal.Risk = CIRiskWeakening
			signal.Reasons = appendUnique(signal.Reasons, "step gated with if: false")
		case strings.Contains(trimmed, "coverage") && strings.Contains(trimmed, "threshold") && reducedNumberRegex.MatchString(trimmed):
			signal.Risk = CIRiskWeakening
			signal.Reasons = appendUnique(signal.Reasons, "coverage threshold reduced")
		}
	}

	if signal.Risk == CIRiskConfig && len(signal.Reasons) == 0 {
		signal.Reasons = []string{"CI / test / coverage config touched"}
	}
	return signal
}

var (
	ciPathRegex = regexp.MustCompile(`(?i)` + strings.Join([]string{
		`^\.github/workflows/.*\.ya?ml$`,
		`(^|/)(jest|vitest|playwright|karma)\.config\.(js|ts|cjs|mjs|json)$`,
		`(^|/)\.coveragerc$`,
		`(^|/)codecov\.ya?ml$`,
		`(^|/)\.codecov\.ya?ml$`,
		`(^|/)\.eslintrc(\.[a-z]+)?$`,
		`(^|/)eslint\.config\.(js|cjs|mjs|ts)$`,
		`(^|/)tsconfig.*\.json$`,
		`(^|/)\.pre-commit-config\.ya?ml$`,
	}, "|"))

	skipPatternRegex   = regexp.MustCompile(`\b(test\.skip|describe\.skip|it\.skip|xit|xdescribe|@pytest\.mark\.skip|@Skip)\b`)
	reducedNumberRegex = regexp.MustCompile(`\b\d+\b`)
)

func appendUnique(s []string, v string) []string {
	for _, x := range s {
		if x == v {
			return s
		}
	}
	return append(s, v)
}
