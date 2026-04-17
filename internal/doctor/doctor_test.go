package doctor

import (
	"os"
	"path/filepath"
	"strings"
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
	// Lean catalog: 7 slash commands + 20 worker skills (2 per agent × 10 agents).
	if r.SkillCount < 25 || r.SkillCount > 31 {
		t.Errorf("expected default skill count in [25, 31], got %d", r.SkillCount)
	}
	if r.AgentCount != 10 {
		t.Errorf("expected 10 agents loaded, got %d", r.AgentCount)
	}
	if r.GroupCount < 2 {
		t.Errorf("expected ≥2 groups loaded, got %d", r.GroupCount)
	}
}

// TestRun_PipelineValidation covers the doctor → pipeline validator wiring:
// a healthy user pipeline passes, a broken agent ref surfaces as CategoryPipeline,
// and a cycle produces an error that flips r.HasErrors to true.
func TestRun_PipelineValidation(t *testing.T) {
	cases := []struct {
		name        string
		yaml        string
		wantHealthy bool
		wantInMsg   string
	}{
		{
			name: "healthy",
			yaml: `schema_version: 1
pipeline:
  id: ok
  description: "healthy fixture"
nodes:
  - id: scout
    agent: scout
    skill: bg-scout-explore
    priority: 100
`,
			wantHealthy: true,
		},
		{
			name: "unknown agent",
			yaml: `schema_version: 1
pipeline:
  id: bad-agent
  description: "broken ref fixture"
nodes:
  - id: ghost
    agent: nobody
    skill: bg-scout-explore
    priority: 10
`,
			wantInMsg: `agent "nobody" not in catalog`,
		},
		{
			name: "cycle",
			yaml: `schema_version: 1
pipeline:
  id: cyclic
  description: "cycle fixture"
nodes:
  - id: a
    agent: scout
    skill: bg-scout-explore
    depends_on: [b]
    priority: 50
  - id: b
    agent: scout
    skill: bg-scout-explore
    depends_on: [a]
    priority: 50
`,
			wantInMsg: "cycle detected",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.MkdirAll(filepath.Join(root, ".bender"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, ".bender", "pipeline.yaml"), []byte(tc.yaml), 0o644); err != nil {
				t.Fatal(err)
			}
			r, err := Run(root)
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			sawPipelineErr := false
			var gotMsgs []string
			for _, i := range r.Issues {
				if i.Category == CategoryPipeline && i.Severity == SeverityError {
					sawPipelineErr = true
					gotMsgs = append(gotMsgs, i.Message)
				}
			}
			if tc.wantHealthy {
				if sawPipelineErr {
					t.Fatalf("expected no pipeline errors, got: %v", gotMsgs)
				}
				return
			}
			if !sawPipelineErr {
				t.Fatalf("expected a CategoryPipeline error, got none. Issues: %+v", r.Issues)
			}
			joined := strings.Join(gotMsgs, "\n")
			if !strings.Contains(joined, tc.wantInMsg) {
				t.Fatalf("expected pipeline error containing %q, got:\n%s", tc.wantInMsg, joined)
			}
			if !r.HasErrors() {
				t.Fatal("expected r.HasErrors() to be true")
			}
		})
	}
}
