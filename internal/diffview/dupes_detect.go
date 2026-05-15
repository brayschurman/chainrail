package diffview

import (
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// DupeSignal records possible-duplicate findings for one file: each
// newly-added top-level symbol and where else in the repo a similar name
// appears outside this PR.
type DupeSignal struct {
	// Symbols added by this file's diff that have prior-art candidates.
	Findings []DupeFinding
}

// DupeFinding is one newly-added symbol and the repo locations where a
// similar identifier already exists.
type DupeFinding struct {
	Symbol     string   // newly-added identifier
	Candidates []string // up to 5 "path:line" matches outside this file
}

// Detector for new top-level identifiers per language, regex-only (no AST).
var (
	dupeGoRegex   = regexp.MustCompile(`^(?:func|type|var|const)\s+([A-Z][A-Za-z0-9_]*)`)
	dupeTSRegex   = regexp.MustCompile(`^export\s+(?:default\s+)?(?:async\s+)?(?:function|const|class|interface|type)\s+([A-Za-z_][A-Za-z0-9_]*)`)
	dupePyDefRgx  = regexp.MustCompile(`^def\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*\(`)
	dupePyClsRgx  = regexp.MustCompile(`^class\s+([A-Z][A-Za-z0-9_]*)`)
)

// DetectNewSymbols extracts identifiers introduced in this file's added
// lines. Only looks at LineAdd entries. Returns deduplicated names in
// declaration order.
func DetectNewSymbols(path string, lines []Line) []string {
	var rgxs []*regexp.Regexp
	switch ext := strings.ToLower(filepath.Ext(path)); ext {
	case ".go":
		rgxs = []*regexp.Regexp{dupeGoRegex}
	case ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs":
		rgxs = []*regexp.Regexp{dupeTSRegex}
	case ".py":
		rgxs = []*regexp.Regexp{dupePyDefRgx, dupePyClsRgx}
	default:
		return nil
	}

	seen := map[string]bool{}
	var out []string
	for _, l := range lines {
		if l.Kind != LineAdd {
			continue
		}
		body := strings.TrimSpace(stripMarker(l.Text))
		for _, rgx := range rgxs {
			if m := rgx.FindStringSubmatch(body); m != nil {
				name := m[1]
				if !seen[name] && len(name) >= 3 {
					seen[name] = true
					out = append(out, name)
				}
			}
		}
	}
	return out
}

// DetectDupes runs a repo-wide grep for each newly-added symbol from the
// file's diff, excluding paths inside the PR itself. Returns nil if no
// candidates are found or the grep fails.
//
// repoRoot is the working directory for `git grep`. excludedPaths are file
// paths in the PR that should be filtered out of grep results (so a symbol
// declaration doesn't match itself).
func DetectDupes(path string, lines []Line, repoRoot string, excludedPaths map[string]bool) DupeSignal {
	if repoRoot == "" {
		return DupeSignal{}
	}
	symbols := DetectNewSymbols(path, lines)
	if len(symbols) == 0 {
		return DupeSignal{}
	}
	var findings []DupeFinding
	for _, sym := range symbols {
		matches := grepSymbol(sym, repoRoot, excludedPaths, path)
		if len(matches) > 0 {
			findings = append(findings, DupeFinding{
				Symbol:     sym,
				Candidates: matches,
			})
		}
	}
	if len(findings) == 0 {
		return DupeSignal{}
	}
	return DupeSignal{Findings: findings}
}

// grepSymbol shells out to `git grep -nF` for symbol matches outside the
// excluded set. Capped at 5 matches per symbol so noisy identifiers don't
// blow up the UI.
func grepSymbol(sym, repoRoot string, excluded map[string]bool, ownPath string) []string {
	cmd := exec.Command("git", "grep", "-n", "-w", "-F", sym, "--",
		// Restrict to known source extensions to keep grep fast and the
		// output meaningful.
		"*.go", "*.ts", "*.tsx", "*.js", "*.jsx", "*.mjs", "*.cjs", "*.py")
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	var matches []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// path:line:content
		parts := strings.SplitN(line, ":", 3)
		if len(parts) < 3 {
			continue
		}
		path := parts[0]
		if path == ownPath || excluded[path] {
			continue
		}
		matches = append(matches, parts[0]+":"+parts[1])
		if len(matches) >= 5 {
			break
		}
	}
	return matches
}
