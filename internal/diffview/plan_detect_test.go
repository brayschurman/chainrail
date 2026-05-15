package diffview

import "testing"

func TestPlanDetect_Empty(t *testing.T) {
	sig := DetectPlan("")
	if sig.Severity != PlanMissing {
		t.Errorf("got %v, want PlanMissing", sig.Severity)
	}
}

func TestPlanDetect_Tiny(t *testing.T) {
	sig := DetectPlan("fixes bug")
	if sig.Severity != PlanMissing {
		t.Errorf("got %v, want PlanMissing", sig.Severity)
	}
}

func TestPlanDetect_Thin(t *testing.T) {
	body := "Touches the auth middleware to support the new session token format introduced in #685."
	sig := DetectPlan(body)
	if sig.Severity != PlanThin {
		t.Errorf("got %v, want PlanThin (chars=%d)", sig.Severity, sig.Chars)
	}
}

func TestPlanDetect_PresentByLength(t *testing.T) {
	body := "Refactors the floorplan exporter to support the new shape API. Walks every existing call site, " +
		"replaces the legacy positional args with the keyword form, updates the snapshot tests, and fixes a " +
		"subtle race in the cache invalidation that was masked by the old API's synchronous nature."
	sig := DetectPlan(body)
	if sig.Severity != PlanPresent {
		t.Errorf("got %v, want PlanPresent (chars=%d)", sig.Severity, sig.Chars)
	}
}

func TestPlanDetect_PresentByHeading(t *testing.T) {
	body := "## What\nDelete the db-old shim package."
	sig := DetectPlan(body)
	if sig.Severity != PlanPresent {
		t.Errorf("got %v, want PlanPresent due to heading", sig.Severity)
	}
	if !sig.HasHeading {
		t.Error("HasHeading should be true")
	}
}

func TestPlanDetect_PresentByBullets(t *testing.T) {
	body := "Changes:\n- delete X\n- rename Y\n- update tests"
	sig := DetectPlan(body)
	if sig.Severity != PlanPresent {
		t.Errorf("got %v, want PlanPresent due to bullets", sig.Severity)
	}
	if !sig.HasBullets {
		t.Error("HasBullets should be true")
	}
}

func TestPlanDetect_CodeBlockDetected(t *testing.T) {
	body := "Reproduces with:\n```\nnpm test\n```\nFix is in commit ab12cd."
	sig := DetectPlan(body)
	if !sig.HasCode {
		t.Error("HasCode should be true")
	}
}
