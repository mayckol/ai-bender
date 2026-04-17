package agent

import (
	"reflect"
	"testing"

	"github.com/mayckol/ai-bender/internal/config"
	"github.com/mayckol/ai-bender/internal/types"
)

func mkAgent() *Agent {
	return &Agent{
		Frontmatter: Frontmatter{
			Name:        "crafter",
			Purpose:     "x",
			PersonaHint: "y",
			WriteScope: WriteScope{
				Allow: []string{"pkg/**", "cmd/**"},
				Deny:  []string{"internal/legacy/**"},
			},
			Skills: types.SkillSelector{
				Explicit: []string{"bg-crafter-implement"},
				Patterns: []string{"bg-crafter-*"},
				Tags:     types.TagSelector{AnyOf: []string{"code-generation"}},
			},
			InvokedBy: []types.Stage{types.StageGHU},
		},
	}
}

func TestApplyOverride_SkillsAddAppendsToExplicit(t *testing.T) {
	a := mkAgent()
	ov := config.AgentOverride{}
	ov.Skills.Add = []string{"my-extra-skill"}
	ApplyOverride(a, ov)
	if !reflect.DeepEqual(a.Skills.Explicit, []string{"bg-crafter-implement", "my-extra-skill"}) {
		t.Fatalf("got %v", a.Skills.Explicit)
	}
}

func TestApplyOverride_SkillsAddDeduplicates(t *testing.T) {
	a := mkAgent()
	ov := config.AgentOverride{}
	ov.Skills.Add = []string{"bg-crafter-implement", "my-extra-skill"}
	ApplyOverride(a, ov)
	if !reflect.DeepEqual(a.Skills.Explicit, []string{"bg-crafter-implement", "my-extra-skill"}) {
		t.Fatalf("got %v", a.Skills.Explicit)
	}
}

func TestApplyOverride_SkillsRemoveStripsAndAddsToNoneOf(t *testing.T) {
	a := mkAgent()
	a.Skills.Tags.AnyOf = append(a.Skills.Tags.AnyOf, "check-data-model")
	ov := config.AgentOverride{}
	ov.Skills.Remove = []string{"check-data-model"}
	ApplyOverride(a, ov)
	if contains(a.Skills.Tags.AnyOf, "check-data-model") {
		t.Fatal("expected check-data-model removed from any_of")
	}
	if !contains(a.Skills.Tags.NoneOf, "check-data-model") {
		t.Fatal("expected check-data-model pushed to none_of")
	}
}

func TestApplyOverride_WriteScopeDenyAddAppends(t *testing.T) {
	a := mkAgent()
	ov := config.AgentOverride{}
	ov.WriteScope.DenyAdd = []string{"internal/legacy/**", "vendor/**"}
	ApplyOverride(a, ov)
	if !reflect.DeepEqual(a.WriteScope.Deny, []string{"internal/legacy/**", "vendor/**"}) {
		t.Fatalf("got %v", a.WriteScope.Deny)
	}
}

func TestApplyOverride_WriteScopeAllowRemoveStrips(t *testing.T) {
	a := mkAgent()
	ov := config.AgentOverride{}
	ov.WriteScope.AllowRemove = []string{"cmd/**"}
	ApplyOverride(a, ov)
	if !reflect.DeepEqual(a.WriteScope.Allow, []string{"pkg/**"}) {
		t.Fatalf("got %v", a.WriteScope.Allow)
	}
}

func TestApplyOverride_Idempotent(t *testing.T) {
	a := mkAgent()
	ov := config.AgentOverride{}
	ov.Skills.Add = []string{"my-extra-skill"}
	ov.WriteScope.DenyAdd = []string{"internal/legacy/**"}
	ApplyOverride(a, ov)
	first := *a
	ApplyOverride(a, ov)
	if !reflect.DeepEqual(first.Skills, a.Skills) {
		t.Fatalf("skills drifted on second apply: before=%v after=%v", first.Skills, a.Skills)
	}
	if !reflect.DeepEqual(first.WriteScope, a.WriteScope) {
		t.Fatalf("write_scope drifted on second apply: before=%v after=%v", first.WriteScope, a.WriteScope)
	}
}

func TestApplyOverride_NilAgentIsNoop(t *testing.T) {
	ApplyOverride(nil, config.AgentOverride{})
}

func contains(xs []string, target string) bool {
	for _, x := range xs {
		if x == target {
			return true
		}
	}
	return false
}
