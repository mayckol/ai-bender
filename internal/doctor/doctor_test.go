package doctor

import (
	"testing"
)

// TestRun_HealthyOnEmbeddedDefaultsOnly: T086.
// With the embedded defaults loaded and no user overrides, the catalog should be healthy enough
// that no error-severity issues are produced (warnings about missing optional tools are acceptable).
func TestRun_HealthyOnEmbeddedDefaultsOnly(t *testing.T) {
	r, err := Run("")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if r.HasErrors() {
		var msgs []string
		for _, i := range r.Issues {
			if i.Severity == SeverityError {
				msgs = append(msgs, i.Message)
			}
		}
		t.Fatalf("expected no errors on default catalog; got %v", msgs)
	}
	if r.SkillCount < 60 {
		t.Errorf("expected ≥60 skills loaded, got %d", r.SkillCount)
	}
	if r.AgentCount != 10 {
		t.Errorf("expected 10 agents loaded, got %d", r.AgentCount)
	}
	if r.GroupCount < 3 {
		t.Errorf("expected ≥3 groups loaded, got %d", r.GroupCount)
	}
}
