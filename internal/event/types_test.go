package event

import "testing"

// TestClarificationsConstants pins the literal type values added for feature
// 006-plan-clarifications. Renaming any of them is a wire-protocol break.
func TestClarificationsConstants(t *testing.T) {
	cases := []struct {
		got  Type
		want string
	}{
		{TypeClarificationsRequested, "clarifications_requested"},
		{TypeClarificationsResolved, "clarifications_resolved"},
		{TypeClarificationsPending, "clarifications_pending"},
	}
	for _, c := range cases {
		if string(c.got) != c.want {
			t.Errorf("type literal: got %q want %q", string(c.got), c.want)
		}
	}
}

func TestKnownTypes_IncludesClarifications(t *testing.T) {
	known := map[Type]bool{}
	for _, t := range KnownTypes() {
		known[t] = true
	}
	for _, want := range []Type{
		TypeClarificationsRequested,
		TypeClarificationsResolved,
		TypeClarificationsPending,
	} {
		if !known[want] {
			t.Errorf("KnownTypes missing %q", want)
		}
	}
}

func TestRequiredPayloadFields_Clarifications(t *testing.T) {
	want := []string{"artifact_path", "question_count", "resolved_count", "pending_count", "skipped_count", "deferred_count"}
	for _, kind := range []Type{TypeClarificationsRequested, TypeClarificationsResolved, TypeClarificationsPending} {
		got := RequiredPayloadFields(kind)
		if len(got) != len(want) {
			t.Fatalf("%s: required fields count: got %d want %d", kind, len(got), len(want))
		}
		set := map[string]bool{}
		for _, k := range got {
			set[k] = true
		}
		for _, k := range want {
			if !set[k] {
				t.Errorf("%s: missing required payload field %q", kind, k)
			}
		}
	}
}
