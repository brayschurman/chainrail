package diffview

import "testing"

func TestPromptInject_NonWorkflowIgnored(t *testing.T) {
	lines := []Line{{Kind: LineAdd, Text: "+pull_request.body anthropic"}}
	sig := DetectPromptInjection("src/foo.go", lines)
	if sig.Severity != PromptInjectNone {
		t.Errorf("non-workflow file: got %v", sig.Severity)
	}
}

func TestPromptInject_UntrustedAndLLMSameFile(t *testing.T) {
	lines := []Line{
		{Kind: LineContext, Text: " on: pull_request"},
		{Kind: LineAdd, Text: "+      - uses: anthropics/claude-action@v1"},
		{Kind: LineAdd, Text: "+        prompt: ${{ github.event.pull_request.body }}"},
	}
	sig := DetectPromptInjection(".github/workflows/triage.yml", lines)
	if sig.Severity != PromptInjectHigh {
		t.Errorf("expected High, got %v reasons=%v", sig.Severity, sig.Reasons)
	}
	if len(sig.Reasons) < 2 {
		t.Errorf("expected at least 2 reasons, got %v", sig.Reasons)
	}
}

func TestPromptInject_UntrustedLLMShellCritical(t *testing.T) {
	lines := []Line{
		{Kind: LineAdd, Text: `+        run: claude --prompt "${{ github.event.pull_request.body }}" | bash`},
	}
	sig := DetectPromptInjection(".github/workflows/danger.yml", lines)
	if sig.Severity != PromptInjectCritical {
		t.Errorf("expected Critical, got %v reasons=%v", sig.Severity, sig.Reasons)
	}
}

func TestPromptInject_WriteAllPermissions(t *testing.T) {
	lines := []Line{
		{Kind: LineAdd, Text: "+permissions: write-all"},
		{Kind: LineAdd, Text: "+        prompt: ${{ github.event.pull_request.body }}"},
		{Kind: LineAdd, Text: "+      - uses: anthropics/claude-action@v1"},
	}
	sig := DetectPromptInjection(".github/workflows/bot.yml", lines)
	if sig.Severity != PromptInjectCritical {
		t.Errorf("expected Critical with write-all, got %v", sig.Severity)
	}
}

func TestPromptInject_OnlyLLMSuspect(t *testing.T) {
	lines := []Line{
		{Kind: LineAdd, Text: "+      - uses: anthropics/claude-action@v1"},
		{Kind: LineAdd, Text: "+        prompt: build a release notes summary"},
	}
	sig := DetectPromptInjection(".github/workflows/release.yml", lines)
	if sig.Severity != PromptInjectSuspect {
		t.Errorf("expected Suspect, got %v", sig.Severity)
	}
}

func TestPromptInject_PlainCIIgnored(t *testing.T) {
	lines := []Line{
		{Kind: LineAdd, Text: "+      - run: pnpm test"},
		{Kind: LineAdd, Text: "+      - run: pnpm lint"},
	}
	sig := DetectPromptInjection(".github/workflows/ci.yml", lines)
	if sig.Severity != PromptInjectNone {
		t.Errorf("plain CI should be None, got %v", sig.Severity)
	}
}
