package agent

import (
	"github.com/mayckol/ai-bender/internal/config"
)

// ApplyOverride mutates the agent's Frontmatter (in place) to reflect the
// additive composers declared in .bender/config.yaml. Idempotent: invoking
// it twice with the same override yields the same Frontmatter.
//
// Semantics:
//   - skills.add:       each name is appended to SkillSelector.Explicit
//                       (dedup against existing explicit entries).
//   - skills.remove:    each name is stripped from Explicit AND Patterns AND
//                       Tags.AnyOf, and appended to Tags.NoneOf so the
//                       resolver's set-difference pass excludes tag-matched
//                       skills of that name too.
//   - write_scope.*_add / *_remove: the allow and deny lists compose
//                       additively; *_remove wins on conflict inside one
//                       override (same as the multi-layer merge rule in
//                       internal/config/precedence.go).
func ApplyOverride(a *Agent, ov config.AgentOverride) {
	if a == nil {
		return
	}

	if len(ov.Skills.Add) > 0 {
		a.Skills.Explicit = appendUniqueStrings(a.Skills.Explicit, ov.Skills.Add)
	}
	if len(ov.Skills.Remove) > 0 {
		a.Skills.Explicit = removeAllStrings(a.Skills.Explicit, ov.Skills.Remove)
		a.Skills.Patterns = removeAllStrings(a.Skills.Patterns, ov.Skills.Remove)
		a.Skills.Tags.AnyOf = removeAllStrings(a.Skills.Tags.AnyOf, ov.Skills.Remove)
		a.Skills.Tags.NoneOf = appendUniqueStrings(a.Skills.Tags.NoneOf, ov.Skills.Remove)
	}

	if len(ov.WriteScope.AllowAdd) > 0 {
		a.WriteScope.Allow = appendUniqueStrings(a.WriteScope.Allow, ov.WriteScope.AllowAdd)
	}
	if len(ov.WriteScope.AllowRemove) > 0 {
		a.WriteScope.Allow = removeAllStrings(a.WriteScope.Allow, ov.WriteScope.AllowRemove)
	}
	if len(ov.WriteScope.DenyAdd) > 0 {
		a.WriteScope.Deny = appendUniqueStrings(a.WriteScope.Deny, ov.WriteScope.DenyAdd)
	}
	if len(ov.WriteScope.DenyRemove) > 0 {
		a.WriteScope.Deny = removeAllStrings(a.WriteScope.Deny, ov.WriteScope.DenyRemove)
	}
}

func appendUniqueStrings(dst, items []string) []string {
	seen := make(map[string]struct{}, len(dst)+len(items))
	for _, v := range dst {
		seen[v] = struct{}{}
	}
	for _, v := range items {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		dst = append(dst, v)
	}
	return dst
}

func removeAllStrings(dst, items []string) []string {
	if len(dst) == 0 || len(items) == 0 {
		return dst
	}
	banned := make(map[string]struct{}, len(items))
	for _, v := range items {
		banned[v] = struct{}{}
	}
	out := dst[:0]
	for _, v := range dst {
		if _, ok := banned[v]; ok {
			continue
		}
		out = append(out, v)
	}
	return out
}
