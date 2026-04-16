package config

import (
	"reflect"
	"testing"
)

func TestMergeAgentOverrides_AppendsAndDedupes(t *testing.T) {
	out := MergeAgentOverrides(
		layerWithSkills("crafter", []string{"x", "y"}, nil),
		layerWithSkills("crafter", []string{"y", "z"}, []string{"old"}),
	)
	got := out["crafter"]
	if !reflect.DeepEqual(got.Skills.Add, []string{"x", "y", "z"}) {
		t.Fatalf("Add: got %v want [x y z]", got.Skills.Add)
	}
	if !reflect.DeepEqual(got.Skills.Remove, []string{"old"}) {
		t.Fatalf("Remove: got %v want [old]", got.Skills.Remove)
	}
}

func TestMergeAgentOverrides_MergesWriteScope(t *testing.T) {
	a := AgentOverride{}
	a.WriteScope.AllowAdd = []string{"**/*.go"}
	a.WriteScope.DenyAdd = []string{"docs/**"}
	b := AgentOverride{}
	b.WriteScope.DenyAdd = []string{"internal/legacy/**"}
	out := MergeAgentOverrides(map[string]AgentOverride{"crafter": a}, map[string]AgentOverride{"crafter": b})
	got := out["crafter"]
	if !reflect.DeepEqual(got.WriteScope.AllowAdd, []string{"**/*.go"}) {
		t.Fatalf("AllowAdd: got %v", got.WriteScope.AllowAdd)
	}
	if !reflect.DeepEqual(got.WriteScope.DenyAdd, []string{"docs/**", "internal/legacy/**"}) {
		t.Fatalf("DenyAdd: got %v", got.WriteScope.DenyAdd)
	}
}

func layerWithSkills(name string, add, remove []string) map[string]AgentOverride {
	ov := AgentOverride{}
	ov.Skills.Add = add
	ov.Skills.Remove = remove
	return map[string]AgentOverride{name: ov}
}
