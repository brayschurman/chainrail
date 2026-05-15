package diffview

import "testing"

func TestIsCIRelated(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{".github/workflows/ci.yml", true},
		{".github/workflows/release.yaml", true},
		{"src/jest.config.ts", true},
		{"vitest.config.js", true},
		{"playwright.config.ts", true},
		{".coveragerc", true},
		{"codecov.yml", true},
		{".codecov.yml", true},
		{".eslintrc.json", true},
		{"eslint.config.mjs", true},
		{"tsconfig.json", true},
		{"tsconfig.build.json", true},
		{".pre-commit-config.yaml", true},
		{"src/index.ts", false},
		{"README.md", false},
		{"package.json", false},
	}
	for _, c := range cases {
		got := IsCIRelated(c.path)
		if got != c.want {
			t.Errorf("IsCIRelated(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

func TestDetectCIRisk_PathOnlyWithoutWeakening(t *testing.T) {
	sig := DetectCIRisk(".github/workflows/ci.yml", []Line{
		{Kind: LineAdd, Text: "+      run: pnpm test"},
		{Kind: LineDel, Text: "-      run: npm test"},
	})
	if sig.Risk != CIRiskConfig {
		t.Errorf("got %v, want CIRiskConfig", sig.Risk)
	}
}

func TestDetectCIRisk_DetectsSkipPattern(t *testing.T) {
	sig := DetectCIRisk("src/wall.test.ts", []Line{
		{Kind: LineAdd, Text: `+  test.skip("regression case", () => {})`},
	})
	// This file isn't CI-config — so we don't surface skip detection here.
	// Verify the path filter still kicks in correctly.
	if sig.Risk != CIRiskNone {
		t.Errorf("non-CI file should be CIRiskNone, got %v", sig.Risk)
	}

	sig = DetectCIRisk(".github/workflows/ci.yml", []Line{
		{Kind: LineAdd, Text: `+      - run: test.skip("regression")`},
	})
	if sig.Risk != CIRiskWeakening {
		t.Errorf("got %v, want CIRiskWeakening", sig.Risk)
	}
	if len(sig.Reasons) == 0 {
		t.Error("expected at least one reason")
	}
}

func TestDetectCIRisk_DetectsContinueOnError(t *testing.T) {
	sig := DetectCIRisk(".github/workflows/ci.yml", []Line{
		{Kind: LineAdd, Text: "+        continue-on-error: true"},
	})
	if sig.Risk != CIRiskWeakening {
		t.Errorf("got %v, want CIRiskWeakening", sig.Risk)
	}
}

func TestDetectCIRisk_DetectsOrTrue(t *testing.T) {
	sig := DetectCIRisk(".github/workflows/ci.yml", []Line{
		{Kind: LineAdd, Text: "+        run: npm test || true"},
	})
	if sig.Risk != CIRiskWeakening {
		t.Errorf("got %v, want CIRiskWeakening", sig.Risk)
	}
}

func TestDetectCIRisk_DetectsIfFalse(t *testing.T) {
	sig := DetectCIRisk(".github/workflows/ci.yml", []Line{
		{Kind: LineAdd, Text: "+      if: false"},
	})
	if sig.Risk != CIRiskWeakening {
		t.Errorf("got %v, want CIRiskWeakening", sig.Risk)
	}
}

func TestDetectCIRisk_IgnoresRemovedWeakeningSignals(t *testing.T) {
	// Removing a continue-on-error: true is a GOOD thing, not bad.
	sig := DetectCIRisk(".github/workflows/ci.yml", []Line{
		{Kind: LineDel, Text: "-        continue-on-error: true"},
	})
	if sig.Risk != CIRiskConfig {
		t.Errorf("removing weakening should still register as Config, got %v", sig.Risk)
	}
}
