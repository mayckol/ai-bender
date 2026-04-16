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
	// Lean catalog: 6 slash commands + 20 worker skills (2 per agent × 10 agents).
	if r.SkillCount < 24 || r.SkillCount > 30 {
		t.Errorf("expected default skill count in [24, 30], got %d", r.SkillCount)
	}
	if r.AgentCount != 10 {
		t.Errorf("expected 10 agents loaded, got %d", r.AgentCount)
	}
	if r.GroupCount < 2 {
		t.Errorf("expected ≥2 groups loaded, got %d", r.GroupCount)
	}
}
