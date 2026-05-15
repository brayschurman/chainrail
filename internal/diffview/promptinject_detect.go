package diffview

import (
	"regexp"
	"strings"
)

// PromptInjectSeverity classifies how dangerous a workflow file's diff is.
type PromptInjectSeverity int

const (
	PromptInjectNone PromptInjectSeverity = iota
	PromptInjectSuspect                        // untrusted input *or* LLM, not both
	PromptInjectHigh                           // untrusted input AND LLM in same file
	PromptInjectCritical                       // pull_request.body → LLM, plus shell exec or write-token
)

// PromptInjectSignal captures one workflow file's prompt-injection risk.
type PromptInjectSignal struct {
	Severity PromptInjectSeverity
	Reasons  []string
}

// DetectPromptInjection inspects workflow YAML for the prompt-injection
// pattern: untrusted user input interpolated into a step that calls an LLM
// or executes the model's output. Returns PromptInjectNone for non-workflow
// files and workflows without the dangerous patterns.
//
// We deliberately work on the *whole file body* — not just the diff — so a
// workflow that *already* had the dangerous pattern but is now being
// modified for any reason still surfaces. Reviewing the addition triggers
// reviewing the file's security posture.
func DetectPromptInjection(path string, lines []Line) PromptInjectSignal {
	if !isWorkflowPath(path) {
		return PromptInjectSignal{}
	}

	// Reconstruct the file's post-diff body — context + adds, skipping dels.
	var sb strings.Builder
	for _, l := range lines {
		switch l.Kind {
		case LineAdd, LineContext:
			sb.WriteString(stripMarker(l.Text))
			sb.WriteByte('\n')
		}
	}
	body := sb.String()

	hasUntrusted := untrustedInputRegex.MatchString(body)
	hasLLM := llmMentionRegex.MatchString(body)
	hasShellExec := shellExecRegex.MatchString(body)
	hasWriteAll := strings.Contains(body, "permissions: write-all") ||
		strings.Contains(body, "permissions: 'write-all'")

	var reasons []string
	if hasUntrusted {
		reasons = append(reasons, "untrusted input interpolated (pull_request body / issue body / commit message)")
	}
	if hasLLM {
		reasons = append(reasons, "step likely calls an LLM (anthropic / openai / claude / gpt)")
	}
	if hasShellExec {
		reasons = append(reasons, "model output executed in shell (eval / pipe to bash)")
	}
	if hasWriteAll {
		reasons = append(reasons, "permissions: write-all granted")
	}

	switch {
	case hasUntrusted && hasLLM && (hasShellExec || hasWriteAll):
		return PromptInjectSignal{Severity: PromptInjectCritical, Reasons: reasons}
	case hasUntrusted && hasLLM:
		return PromptInjectSignal{Severity: PromptInjectHigh, Reasons: reasons}
	case hasUntrusted || hasLLM:
		return PromptInjectSignal{Severity: PromptInjectSuspect, Reasons: reasons}
	}
	return PromptInjectSignal{}
}

var (
	workflowPathRegex = regexp.MustCompile(`(?i)^\.github/workflows/.*\.ya?ml$`)
	untrustedInputRegex = regexp.MustCompile(
		`\$\{\{\s*github\.event\.(pull_request|issue|comment)\.(body|title)` +
			`|\$\{\{\s*github\.event\.head_commit\.message` +
			`|\$\{\{\s*github\.event\.commits\[\*\]\.message`,
	)
	llmMentionRegex = regexp.MustCompile(
		`(?i)\b(anthropic|openai|claude|gpt-?[0-9]?|chatgpt|gemini|llm)\b`,
	)
	shellExecRegex = regexp.MustCompile(
		`(?i)(eval\s+["$]|\| ?(bash|sh)\b|\$\([^)]*(anthropic|openai|claude|gpt))`,
	)
)

func isWorkflowPath(path string) bool {
	return workflowPathRegex.MatchString(path)
}
