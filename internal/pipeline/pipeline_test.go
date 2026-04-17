package pipeline

import (
	"strings"
	"testing"
)

type stubCatalog struct {
	skills map[string]bool
	agents map[string]bool
}

func (s stubCatalog) HasSkill(n string) bool { return s.skills[n] }
func (s stubCatalog) HasAgent(n string) bool { return s.agents[n] }

func TestParse_MinimalPipeline(t *testing.T) {
	src := `
schema_version: 1
pipeline:
  id: demo
  description: "test"
  max_concurrent: 4
nodes:
  - id: scout
    agent: scout
    skill: bg-scout-explore
    priority: 100
  - id: arch
    agent: architect
    skill: bg-architect-validate
    depends_on: [scout]
    priority: 90
`
	p, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if p.Meta.ID != "demo" || p.EffectiveMaxConcurrent() != 4 {
		t.Fatalf("meta parse: %+v", p.Meta)
	}
	if len(p.Nodes) != 2 || p.Nodes[0].ID != "scout" || p.Nodes[1].DependsOn[0] != "scout" {
		t.Fatalf("nodes parse: %+v", p.Nodes)
	}
}

func TestParse_RejectsOldSchema(t *testing.T) {
	src := `
schema_version: 99
pipeline: { id: x, description: y }
nodes: [{ id: a, agent: x, skill: y }]
`
	if _, err := Parse([]byte(src)); err == nil {
		t.Fatal("expected schema_version mismatch")
	}
}

func TestValidate_FlagsBadReferences(t *testing.T) {
	p := &Pipeline{
		SchemaVersion: 1,
		Meta:          Meta{ID: "x", Description: "y"},
		Nodes: []Node{
			{ID: "a", Agent: "ghost", Skill: "bg-ghost-run", DependsOn: []string{"missing"}},
			{ID: "a", Agent: "scout", Skill: "bg-scout-explore"}, // duplicate id
		},
	}
	cat := stubCatalog{
		skills: map[string]bool{"bg-scout-explore": true},
		agents: map[string]bool{"scout": true},
	}
	vs := Validate(p, cat)
	joined := ""
	for _, v := range vs {
		joined += v.String() + "\n"
	}
	wants := []string{
		"duplicate id",
		`agent "ghost" not in catalog`,
		`skill "bg-ghost-run" not in catalog`,
		`depends_on "missing" is not a known node id`,
	}
	for _, w := range wants {
		if !strings.Contains(joined, w) {
			t.Errorf("expected violation containing %q, got:\n%s", w, joined)
		}
	}
}

func TestValidate_DetectsCycle(t *testing.T) {
	p := &Pipeline{
		SchemaVersion: 1,
		Meta:          Meta{ID: "x", Description: "y"},
		Nodes: []Node{
			{ID: "a", Agent: "scout", Skill: "bg-scout-explore", DependsOn: []string{"b"}},
			{ID: "b", Agent: "scout", Skill: "bg-scout-explore", DependsOn: []string{"a"}},
		},
	}
	cat := stubCatalog{
		skills: map[string]bool{"bg-scout-explore": true},
		agents: map[string]bool{"scout": true},
	}
	vs := Validate(p, cat)
	sawCycle := false
	for _, v := range vs {
		if strings.Contains(v.Message, "cycle detected") {
			sawCycle = true
		}
	}
	if !sawCycle {
		t.Fatalf("expected cycle detection, got %v", vs)
	}
}

func TestValidate_WhenVariableLookup(t *testing.T) {
	p := &Pipeline{
		SchemaVersion: 1,
		Meta:          Meta{ID: "x", Description: "y"},
		Variables: map[string]VariableDef{
			"tdd_mode": {Kind: VarLiteral, Value: true},
		},
		Nodes: []Node{
			{ID: "a", Agent: "scout", Skill: "bg-scout-explore", When: "tdd_mode == true"},
			{ID: "b", Agent: "scout", Skill: "bg-scout-explore", When: "missing_var == true"},
			{ID: "c", Agent: "scout", Skill: "bg-scout-explore", When: "tdd_mode"}, // bad shape
		},
	}
	cat := stubCatalog{
		skills: map[string]bool{"bg-scout-explore": true},
		agents: map[string]bool{"scout": true},
	}
	vs := Validate(p, cat)
	var sawMissing, sawShape bool
	for _, v := range vs {
		if strings.Contains(v.Message, "undeclared variable") {
			sawMissing = true
		}
		if strings.Contains(v.Message, "is not a supported expression") {
			sawShape = true
		}
	}
	if !sawMissing || !sawShape {
		t.Fatalf("expected both violations, got:\n%v", vs)
	}
}

func TestDryRun_EmergentParallelism(t *testing.T) {
	// scout → architect → {crafter ∥ tester} → linter
	// Exercises: sequential nodes, emergent fan-out, and priority-based ordering.
	p := &Pipeline{
		SchemaVersion: 1,
		Meta:          Meta{ID: "x", Description: "y", MaxConcurrent: 8},
		Nodes: []Node{
			{ID: "scout", Agent: "scout", Skill: "bg-scout-explore", Priority: 100},
			{ID: "arch", Agent: "architect", Skill: "bg-architect-validate", DependsOn: []string{"scout"}, Priority: 90},
			{ID: "crafter", Agent: "crafter", Skill: "bg-crafter-implement", DependsOn: []string{"arch"}, Priority: 80},
			{ID: "tester", Agent: "tester", Skill: "bg-tester-write-and-run", DependsOn: []string{"arch"}, Priority: 80},
			{ID: "linter", Agent: "linter", Skill: "bg-linter-run-and-fix", DependsOn: []string{"crafter", "tester"}, Priority: 70},
		},
	}
	batches, err := DryRun(p, nil)
	if err != nil {
		t.Fatalf("DryRun: %v", err)
	}
	if len(batches) != 4 {
		t.Fatalf("wanted 4 waves, got %d: %+v", len(batches), batches)
	}
	if len(batches[2].Nodes) != 2 {
		t.Fatalf("wave 2 should be parallel {crafter, tester}, got %v", batches[2].Nodes)
	}
	if batches[2].Nodes[0] != "crafter" || batches[2].Nodes[1] != "tester" {
		t.Fatalf("wave 2 order (priority-then-id): %v", batches[2].Nodes)
	}
}

func TestDryRun_RespectsMaxConcurrent(t *testing.T) {
	p := &Pipeline{
		SchemaVersion: 1,
		Meta:          Meta{ID: "x", Description: "y", MaxConcurrent: 2},
		Nodes: []Node{
			{ID: "a", Agent: "scout", Skill: "bg-scout-explore", Priority: 100},
			{ID: "b", Agent: "scout", Skill: "bg-scout-explore", Priority: 100},
			{ID: "c", Agent: "scout", Skill: "bg-scout-explore", Priority: 90},
		},
	}
	batches, err := DryRun(p, nil)
	if err != nil {
		t.Fatalf("DryRun: %v", err)
	}
	// Cap is 2 → first batch takes the two highest-priority; c falls to batch 2.
	if len(batches) != 2 || len(batches[0].Nodes) != 2 || batches[1].Nodes[0] != "c" {
		t.Fatalf("max_concurrent not enforced: %+v", batches)
	}
}

func TestValidate_ShapeRules(t *testing.T) {
	cases := []struct {
		name      string
		node      Node
		wantInMsg string
	}{
		{"self-ref", Node{ID: "x", Agent: "scout", Skill: "bg-scout-explore", DependsOn: []string{"x"}}, "depends on itself"},
		{"duplicate-dep", Node{ID: "x", Agent: "scout", Skill: "bg-scout-explore", DependsOn: []string{"a", "a"}}, "duplicate dependency"},
		{"bad-depends-mode", Node{ID: "x", Agent: "scout", Skill: "bg-scout-explore", DependsMode: "both"}, "unknown depends_mode"},
		{"orchestrator-with-agent", Node{ID: "x", Type: NodeOrchestrator, Agent: "scout"}, "orchestrator nodes must not declare agent"},
		{"agent-missing-skill", Node{ID: "x", Agent: "scout"}, "skill is required"},
	}
	cat := stubCatalog{
		skills: map[string]bool{"bg-scout-explore": true},
		agents: map[string]bool{"scout": true, "a": true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			nodes := []Node{{ID: "a", Agent: "a", Skill: "bg-scout-explore"}, tc.node}
			p := &Pipeline{SchemaVersion: 1, Meta: Meta{ID: "x", Description: "y"}, Nodes: nodes}
			vs := Validate(p, cat)
			joined := ""
			for _, v := range vs {
				joined += v.String() + "\n"
			}
			if !strings.Contains(joined, tc.wantInMsg) {
				t.Fatalf("expected violation containing %q, got:\n%s", tc.wantInMsg, joined)
			}
		})
	}
}

func TestValidate_VariableKindFields(t *testing.T) {
	cases := []struct {
		name string
		def  VariableDef
		want string
	}{
		{"glob-no-pattern", VariableDef{Kind: VarGlobApproved, RequireStatus: "approved"}, "requires pattern"},
		{"glob-no-status", VariableDef{Kind: VarGlobApproved, Pattern: "x/**"}, "requires require_status"},
		{"planflag-no-flag", VariableDef{Kind: VarPlanFlag}, "requires flag"},
		{"literal-no-value", VariableDef{Kind: VarLiteral}, "requires value"},
		{"unknown-kind", VariableDef{Kind: "wat"}, `unknown kind "wat"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := &Pipeline{
				SchemaVersion: 1,
				Meta:          Meta{ID: "x", Description: "y"},
				Variables:     map[string]VariableDef{"v": tc.def},
				Nodes:         []Node{{ID: "a", Agent: "scout", Skill: "bg-scout-explore"}},
			}
			vs := Validate(p, stubCatalog{
				skills: map[string]bool{"bg-scout-explore": true},
				agents: map[string]bool{"scout": true},
			})
			joined := ""
			for _, v := range vs {
				joined += v.String() + "\n"
			}
			if !strings.Contains(joined, tc.want) {
				t.Fatalf("expected violation containing %q, got:\n%s", tc.want, joined)
			}
		})
	}
}

func TestDryRun_WhenSkipsNodes(t *testing.T) {
	// arch depends on scout and optional surgeon; surgeon skipped via when.
	// Downstream should still resolve because skipped deps count as resolved.
	p := &Pipeline{
		SchemaVersion: 1,
		Meta:          Meta{ID: "x", Description: "y"},
		Variables: map[string]VariableDef{
			"refactor": {Kind: VarLiteral, Value: false},
		},
		Nodes: []Node{
			{ID: "scout", Agent: "scout", Skill: "bg-scout-explore", Priority: 100},
			{ID: "surgeon", Agent: "surgeon", Skill: "bg-surgeon-refactor", DependsOn: []string{"scout"}, Priority: 90, When: "refactor == true"},
			{ID: "arch", Agent: "architect", Skill: "bg-architect-validate", DependsOn: []string{"scout", "surgeon"}, Priority: 85},
		},
	}
	batches, err := DryRun(p, map[string]any{"refactor": false})
	if err != nil {
		t.Fatalf("DryRun: %v", err)
	}
	// Expected: wave 0 = [scout], wave 1 = [arch] (surgeon skipped).
	ids := func(b Batch) []string { return b.Nodes }
	if len(batches) != 2 || ids(batches[0])[0] != "scout" || ids(batches[1])[0] != "arch" {
		t.Fatalf("when-skip failed: %+v", batches)
	}
}
