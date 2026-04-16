package skill

import (
	"reflect"
	"testing"

	"github.com/mayckol/ai-bender/internal/types"
)

func mkSkill(name string, ctx types.Context, provides []string, stages []types.Stage, applies []types.IssueType) *Skill {
	return &Skill{
		Frontmatter: Frontmatter{
			Name:      name,
			Context:   ctx,
			Provides:  provides,
			Stages:    stages,
			AppliesTo: applies,
		},
	}
}

func mkCatalog(skills ...*Skill) *Catalog {
	c := &Catalog{skills: map[string]*Skill{}}
	for _, s := range skills {
		c.skills[s.Name] = s
	}
	c.reindex()
	return c
}

func names(out []*Skill) []string {
	n := make([]string, len(out))
	for i, s := range out {
		n[i] = s.Name
	}
	return n
}

func TestResolve_ExplicitOnly(t *testing.T) {
	cat := mkCatalog(
		mkSkill("a", types.ContextBG, []string{"x"}, []types.Stage{types.StageGHU}, []types.IssueType{types.IssueAny}),
		mkSkill("b", types.ContextBG, []string{"x"}, []types.Stage{types.StageGHU}, []types.IssueType{types.IssueAny}),
	)
	got := Resolve(cat, types.SkillSelector{Explicit: []string{"a"}}, types.ResolveContext{Stage: types.StageGHU})
	if !reflect.DeepEqual(names(got), []string{"a"}) {
		t.Fatalf("got %v want [a]", names(got))
	}
}

func TestResolve_PatternsThenTagsAnyOf(t *testing.T) {
	cat := mkCatalog(
		mkSkill("check-data-model", types.ContextBG, []string{"check"}, []types.Stage{types.StageGHU}, []types.IssueType{types.IssueAny}),
		mkSkill("check-api", types.ContextBG, []string{"check"}, []types.Stage{types.StageGHU}, []types.IssueType{types.IssueAny}),
		mkSkill("apply-patch", types.ContextBG, []string{"code-generation"}, []types.Stage{types.StageGHU}, []types.IssueType{types.IssueAny}),
	)
	sel := types.SkillSelector{
		Patterns: []string{"check-*"},
		Tags:     types.TagSelector{AnyOf: []string{"code-generation"}},
	}
	got := names(Resolve(cat, sel, types.ResolveContext{Stage: types.StageGHU}))
	want := []string{"apply-patch", "check-api", "check-data-model"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestResolve_NoneOfRemoves(t *testing.T) {
	cat := mkCatalog(
		mkSkill("destroy-everything", types.ContextBG, []string{"code-generation", "destructive"}, []types.Stage{types.StageGHU}, []types.IssueType{types.IssueAny}),
		mkSkill("apply-patch", types.ContextBG, []string{"code-generation"}, []types.Stage{types.StageGHU}, []types.IssueType{types.IssueAny}),
	)
	sel := types.SkillSelector{
		Tags: types.TagSelector{AnyOf: []string{"code-generation"}, NoneOf: []string{"destructive"}},
	}
	got := names(Resolve(cat, sel, types.ResolveContext{Stage: types.StageGHU}))
	if !reflect.DeepEqual(got, []string{"apply-patch"}) {
		t.Fatalf("got %v want [apply-patch]", got)
	}
}

func TestResolve_FilterByContext(t *testing.T) {
	cat := mkCatalog(
		mkSkill("fg-thing", types.ContextFG, []string{"x"}, []types.Stage{types.StageGHU}, []types.IssueType{types.IssueAny}),
		mkSkill("bg-thing", types.ContextBG, []string{"x"}, []types.Stage{types.StageGHU}, []types.IssueType{types.IssueAny}),
	)
	sel := types.SkillSelector{Tags: types.TagSelector{AnyOf: []string{"x"}}}
	got := names(Resolve(cat, sel, types.ResolveContext{Stage: types.StageGHU, AgentContexts: []types.Context{types.ContextBG}}))
	if !reflect.DeepEqual(got, []string{"bg-thing"}) {
		t.Fatalf("got %v want [bg-thing]", got)
	}
}

func TestResolve_FilterByIssueType(t *testing.T) {
	cat := mkCatalog(
		mkSkill("only-feature", types.ContextBG, []string{"x"}, []types.Stage{types.StageGHU}, []types.IssueType{types.IssueFeature}),
		mkSkill("anything", types.ContextBG, []string{"x"}, []types.Stage{types.StageGHU}, []types.IssueType{types.IssueAny}),
	)
	sel := types.SkillSelector{Tags: types.TagSelector{AnyOf: []string{"x"}}}
	got := names(Resolve(cat, sel, types.ResolveContext{Stage: types.StageGHU, IssueType: types.IssueBug}))
	if !reflect.DeepEqual(got, []string{"anything"}) {
		t.Fatalf("got %v want [anything]", got)
	}
}

func TestResolve_FilterByStage(t *testing.T) {
	cat := mkCatalog(
		mkSkill("plan-only", types.ContextBG, []string{"x"}, []types.Stage{types.StagePlan}, []types.IssueType{types.IssueAny}),
		mkSkill("ghu-only", types.ContextBG, []string{"x"}, []types.Stage{types.StageGHU}, []types.IssueType{types.IssueAny}),
	)
	sel := types.SkillSelector{Tags: types.TagSelector{AnyOf: []string{"x"}}}
	got := names(Resolve(cat, sel, types.ResolveContext{Stage: types.StageGHU}))
	if !reflect.DeepEqual(got, []string{"ghu-only"}) {
		t.Fatalf("got %v want [ghu-only]", got)
	}
}

func TestResolve_Determinism(t *testing.T) {
	cat := mkCatalog(
		mkSkill("c", types.ContextBG, []string{"x"}, []types.Stage{types.StageGHU}, []types.IssueType{types.IssueAny}),
		mkSkill("a", types.ContextBG, []string{"x"}, []types.Stage{types.StageGHU}, []types.IssueType{types.IssueAny}),
		mkSkill("b", types.ContextBG, []string{"x"}, []types.Stage{types.StageGHU}, []types.IssueType{types.IssueAny}),
	)
	sel := types.SkillSelector{Tags: types.TagSelector{AnyOf: []string{"x"}}}
	first := names(Resolve(cat, sel, types.ResolveContext{Stage: types.StageGHU}))
	for i := range 100 {
		got := names(Resolve(cat, sel, types.ResolveContext{Stage: types.StageGHU}))
		if !reflect.DeepEqual(got, first) {
			t.Fatalf("non-deterministic: iter %d got %v vs first %v", i, got, first)
		}
	}
	if !reflect.DeepEqual(first, []string{"a", "b", "c"}) {
		t.Fatalf("not sorted: %v", first)
	}
}
