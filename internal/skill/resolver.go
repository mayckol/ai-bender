package skill

import (
	"path"
	"sort"

	"github.com/mayckol/ai-bender/internal/types"
)

// Resolve returns the deterministic effective skill set for a selector + context, drawn from cat.
// The order of operations is the contract pinned by spec FR-024:
//
//  1. Add explicit names.
//  2. Add all skills whose name matches any glob pattern.
//  3. Add all skills whose `provides` ∩ `tags.any_of` is non-empty.
//  4. Remove skills whose `provides` ∩ `tags.none_of` is non-empty.
//  5. Filter by agent context (when ctx.AgentContexts is non-empty).
//  6. Filter by run issue type (`any` short-circuits).
//  7. Filter by current stage.
//
// The function is pure; given the same catalog and inputs it always returns the same slice.
func Resolve(cat *Catalog, sel types.SkillSelector, ctx types.ResolveContext) []*Skill {
	if cat == nil {
		return nil
	}
	picked := map[string]struct{}{}

	for _, name := range sel.Explicit {
		if cat.Get(name) != nil {
			picked[name] = struct{}{}
		}
	}
	for _, pattern := range sel.Patterns {
		for _, name := range cat.Names() {
			if matched, _ := path.Match(pattern, name); matched {
				picked[name] = struct{}{}
			}
		}
	}
	if len(sel.Tags.AnyOf) > 0 {
		for _, s := range cat.All() {
			if intersects(s.Provides, sel.Tags.AnyOf) {
				picked[s.Name] = struct{}{}
			}
		}
	}
	if len(sel.Tags.NoneOf) > 0 {
		for name := range picked {
			s := cat.Get(name)
			if s != nil && intersects(s.Provides, sel.Tags.NoneOf) {
				delete(picked, name)
			}
		}
	}

	out := make([]*Skill, 0, len(picked))
	for name := range picked {
		s := cat.Get(name)
		if s == nil {
			continue
		}
		if !contextMatches(s.Context, ctx.AgentContexts) {
			continue
		}
		if !issueTypeMatches(s.AppliesTo, ctx.IssueType) {
			continue
		}
		if !stageMatches(s.Stages, ctx.Stage) {
			continue
		}
		out = append(out, s)
	}

	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func intersects(a, b []string) bool {
	if len(a) == 0 || len(b) == 0 {
		return false
	}
	set := make(map[string]struct{}, len(a))
	for _, v := range a {
		set[v] = struct{}{}
	}
	for _, v := range b {
		if _, ok := set[v]; ok {
			return true
		}
	}
	return false
}

func contextMatches(skillCtx types.Context, agentCtx []types.Context) bool {
	if len(agentCtx) == 0 {
		return true
	}
	for _, c := range agentCtx {
		if c == skillCtx {
			return true
		}
	}
	return false
}

func issueTypeMatches(applies []types.IssueType, run types.IssueType) bool {
	if run == "" {
		return true
	}
	for _, it := range applies {
		if it == types.IssueAny || it == run {
			return true
		}
	}
	return false
}

func stageMatches(stages []types.Stage, stage types.Stage) bool {
	if stage == "" {
		return true
	}
	for _, s := range stages {
		if s == stage {
			return true
		}
	}
	return false
}
