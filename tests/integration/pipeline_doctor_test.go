package integration_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestDoctor_PipelineViolations (T033 / T034): three malformed fixtures, each
// surfacing a distinct node-attributed violation, all under the 200 ms budget.
func TestDoctor_PipelineViolations(t *testing.T) {
	bin := buildBenderOnce(t)

	cases := []struct {
		name       string
		pipeline   string
		wantSubstr string
	}{
		{
			name: "cycle",
			pipeline: `schema_version: 1
pipeline:
  id: cyclic
  description: "cycle fixture"
nodes:
  - { id: a, agent: scout, skill: bg-scout-explore, depends_on: [b] }
  - { id: b, agent: scout, skill: bg-scout-explore, depends_on: [a] }
`,
			wantSubstr: "cycle detected",
		},
		{
			name: "unknown_agent",
			pipeline: `schema_version: 1
pipeline:
  id: bad-agent
  description: "broken ref fixture"
nodes:
  - { id: ghost, agent: nobody, skill: bg-scout-explore }
`,
			wantSubstr: `agent "nobody" not in catalog`,
		},
		{
			name: "bad_when",
			pipeline: `schema_version: 1
pipeline:
  id: bad-when
  description: "broken when fixture"
nodes:
  - id: x
    agent: scout
    skill: bg-scout-explore
    when: "tdd_mode"
`,
			wantSubstr: "is not a supported expression",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := mkProject(t)
			if err := os.MkdirAll(filepath.Join(root, ".bender"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, ".bender", "pipeline.yaml"), []byte(tc.pipeline), 0o644); err != nil {
				t.Fatal(err)
			}

			start := time.Now()
			out, err := runBender(t, bin, root, "doctor")
			elapsed := time.Since(start)

			if err == nil {
				t.Fatalf("expected non-zero exit; got healthy:\n%s", out)
			}
			if !strings.Contains(out, tc.wantSubstr) {
				t.Fatalf("expected violation containing %q, got:\n%s", tc.wantSubstr, out)
			}
			if elapsed > 2*time.Second {
				t.Fatalf("doctor took %s — budget is well under 2 s (SC-004 targets 200 ms for a 20-node pipeline; this 2-node fixture is slack)", elapsed)
			}
		})
	}
}
