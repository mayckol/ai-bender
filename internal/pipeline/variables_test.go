package pipeline

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEvaluateVariables_Literal(t *testing.T) {
	p := &Pipeline{
		Variables: map[string]VariableDef{
			"tdd_mode":      {Kind: VarLiteral, Value: true},
			"plan_refactor": {Kind: VarLiteral, Value: false},
			"mode":          {Kind: VarLiteral, Value: "strict"},
		},
	}
	got, err := EvaluateVariables(p, t.TempDir())
	if err != nil {
		t.Fatalf("EvaluateVariables: %v", err)
	}
	if got["tdd_mode"] != true || got["plan_refactor"] != false || got["mode"] != "strict" {
		t.Fatalf("literal values: %+v", got)
	}
}

func TestEvaluateVariables_GlobApproved(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".bender", "artifacts", "plan", "tests")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Two approved scaffolds.
	for _, name := range []string{"spec-a.md", "spec-b.md"} {
		body := "---\nstatus: approved\n---\nbody\n"
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	p := &Pipeline{
		Variables: map[string]VariableDef{
			"tdd_mode": {Kind: VarGlobApproved, Pattern: ".bender/artifacts/plan/tests/**/*.md", RequireStatus: "approved"},
		},
	}
	got, err := EvaluateVariables(p, root)
	if err != nil {
		t.Fatalf("EvaluateVariables: %v", err)
	}
	if got["tdd_mode"] != true {
		t.Fatalf("expected tdd_mode=true with approved scaffolds, got %+v", got)
	}

	// Swap one scaffold to draft — should flip to false.
	if err := os.WriteFile(filepath.Join(dir, "spec-a.md"), []byte("---\nstatus: draft\n---\nbody\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err = EvaluateVariables(p, root)
	if err != nil {
		t.Fatalf("EvaluateVariables: %v", err)
	}
	if got["tdd_mode"] != false {
		t.Fatalf("expected tdd_mode=false with one draft scaffold, got %+v", got)
	}
}

func TestEvaluateVariables_GlobEmpty(t *testing.T) {
	root := t.TempDir()
	p := &Pipeline{
		Variables: map[string]VariableDef{
			"tdd_mode": {Kind: VarGlobApproved, Pattern: ".bender/artifacts/plan/tests/**/*.md", RequireStatus: "approved"},
		},
	}
	got, err := EvaluateVariables(p, root)
	if err != nil {
		t.Fatalf("EvaluateVariables: %v", err)
	}
	if got["tdd_mode"] != false {
		t.Fatalf("expected tdd_mode=false with no scaffolds, got %+v", got)
	}
}

func TestEvaluateVariables_PlanFlag(t *testing.T) {
	root := t.TempDir()
	planDir := filepath.Join(root, ".bender", "artifacts", "plan")
	if err := os.MkdirAll(planDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\nstatus: approved\nrequires_refactor: true\n---\n"
	if err := os.WriteFile(filepath.Join(planDir, "plan-2026-01-01.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	p := &Pipeline{
		Variables: map[string]VariableDef{
			"plan_refactor": {Kind: VarPlanFlag, Flag: "requires_refactor"},
		},
	}
	got, err := EvaluateVariables(p, root)
	if err != nil {
		t.Fatalf("EvaluateVariables: %v", err)
	}
	if got["plan_refactor"] != true {
		t.Fatalf("expected plan_refactor=true, got %+v", got)
	}
}

func TestEvaluateVariables_PlanFlagMissing(t *testing.T) {
	root := t.TempDir()
	p := &Pipeline{
		Variables: map[string]VariableDef{
			"plan_refactor": {Kind: VarPlanFlag, Flag: "requires_refactor"},
		},
	}
	got, err := EvaluateVariables(p, root)
	if err != nil {
		t.Fatalf("EvaluateVariables: %v", err)
	}
	if got["plan_refactor"] != false {
		t.Fatalf("expected plan_refactor=false when plan missing, got %+v", got)
	}
}

// TestDryRun_DefaultPipeline_PlainBranch drives the shipped default pipeline
// through DryRun with tdd_mode=false, plan_refactor=false and asserts the
// expected plain-branch wave sequence.
func TestDryRun_DefaultPipeline_PlainBranch(t *testing.T) {
	p := defaultPipelineFixture(t)
	vars := map[string]any{"tdd_mode": false, "plan_refactor": false}
	batches, err := DryRun(p, vars)
	if err != nil {
		t.Fatalf("DryRun: %v", err)
	}
	waves := waveLabels(batches)
	want := [][]string{{"scout"}, {"architect"}, {"plain-impl", "plain-tests"}, {"linter"}, {"benchmarker", "reviewer", "sentinel"}, {"scribe"}, {"report"}}
	if !equalWaves(waves, want) {
		t.Fatalf("plain-branch waves:\nwant %v\ngot  %v", want, waves)
	}
}

func TestDryRun_DefaultPipeline_TDDBranch(t *testing.T) {
	p := defaultPipelineFixture(t)
	vars := map[string]any{"tdd_mode": true, "plan_refactor": false}
	batches, err := DryRun(p, vars)
	if err != nil {
		t.Fatalf("DryRun: %v", err)
	}
	waves := waveLabels(batches)
	want := [][]string{{"scout"}, {"architect"}, {"tdd-scaffold"}, {"tdd-implement"}, {"tdd-verify"}, {"linter"}, {"benchmarker", "reviewer", "sentinel"}, {"scribe"}, {"report"}}
	if !equalWaves(waves, want) {
		t.Fatalf("TDD-branch waves:\nwant %v\ngot  %v", want, waves)
	}
}

// defaultPipelineFixture builds a Pipeline mirroring the shipped embed default
// so the DryRun tests above don't depend on the embedded FS at test time.
func defaultPipelineFixture(t *testing.T) *Pipeline {
	t.Helper()
	return &Pipeline{
		SchemaVersion: 1,
		Meta:          Meta{ID: "ghu-default", Description: "fixture", MaxConcurrent: 8},
		Variables: map[string]VariableDef{
			"tdd_mode":      {Kind: VarLiteral, Value: false},
			"plan_refactor": {Kind: VarLiteral, Value: false},
		},
		Nodes: []Node{
			{ID: "scout", Agent: "scout", Skill: "bg-scout-explore", Priority: 100},
			{ID: "architect", Agent: "architect", Skill: "bg-architect-validate", DependsOn: []string{"scout"}, Priority: 100},
			{ID: "surgeon", Agent: "surgeon", Skill: "bg-surgeon-refactor", DependsOn: []string{"architect"}, Priority: 90, When: "plan_refactor == true"},
			{ID: "tdd-scaffold", Agent: "tester", Skill: "bg-tester-scaffold", DependsOn: []string{"architect", "surgeon"}, Priority: 80, When: "tdd_mode == true", HaltOnFailure: true},
			{ID: "tdd-implement", Agent: "crafter", Skill: "bg-crafter-implement", DependsOn: []string{"tdd-scaffold"}, Priority: 80, When: "tdd_mode == true", HaltOnFailure: true},
			{ID: "tdd-verify", Agent: "tester", Skill: "bg-tester-run", DependsOn: []string{"tdd-implement"}, Priority: 80, When: "tdd_mode == true", HaltOnFailure: true},
			{ID: "plain-impl", Agent: "crafter", Skill: "bg-crafter-implement", DependsOn: []string{"architect", "surgeon"}, Priority: 80, When: "tdd_mode == false"},
			{ID: "plain-tests", Agent: "tester", Skill: "bg-tester-write-and-run", DependsOn: []string{"architect", "surgeon"}, Priority: 80, When: "tdd_mode == false"},
			{ID: "linter", Agent: "linter", Skill: "bg-linter-run-and-fix", DependsOn: []string{"tdd-verify", "plain-impl", "plain-tests"}, DependsMode: DependsAny, Priority: 70},
			{ID: "reviewer", Agent: "reviewer", Skill: "bg-reviewer-critique", DependsOn: []string{"linter"}, Priority: 60},
			{ID: "sentinel", Agent: "sentinel", Skill: "bg-sentinel-static-scan", DependsOn: []string{"linter"}, Priority: 60},
			{ID: "benchmarker", Agent: "benchmarker", Skill: "bg-benchmarker-analyze", DependsOn: []string{"linter"}, Priority: 60},
			{ID: "scribe", Agent: "scribe", Skill: "bg-scribe-update-docs", DependsOn: []string{"reviewer", "sentinel", "benchmarker"}, Priority: 50},
			{ID: "report", Type: NodeOrchestrator, DependsOn: []string{"scribe"}, Priority: 10},
		},
	}
}

func waveLabels(batches []Batch) [][]string {
	out := make([][]string, len(batches))
	for i, b := range batches {
		out[i] = append([]string(nil), b.Nodes...)
	}
	return out
}

func equalWaves(a, b [][]string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if len(a[i]) != len(b[i]) {
			return false
		}
		for j := range a[i] {
			if a[i][j] != b[i][j] {
				return false
			}
		}
	}
	return true
}
