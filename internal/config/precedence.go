package config

// MergeAgentOverrides applies per-agent overrides from later layers on top of earlier ones,
// honouring the spec FR-029 rule that arrays compose via `add`/`remove` rather than replacing.
func MergeAgentOverrides(layers ...map[string]AgentOverride) map[string]AgentOverride {
	out := map[string]AgentOverride{}
	for _, layer := range layers {
		for name, ov := range layer {
			merged := out[name]
			merged.Skills.Add = appendUnique(merged.Skills.Add, ov.Skills.Add...)
			merged.Skills.Remove = appendUnique(merged.Skills.Remove, ov.Skills.Remove...)
			merged.WriteScope.AllowAdd = appendUnique(merged.WriteScope.AllowAdd, ov.WriteScope.AllowAdd...)
			merged.WriteScope.AllowRemove = appendUnique(merged.WriteScope.AllowRemove, ov.WriteScope.AllowRemove...)
			merged.WriteScope.DenyAdd = appendUnique(merged.WriteScope.DenyAdd, ov.WriteScope.DenyAdd...)
			merged.WriteScope.DenyRemove = appendUnique(merged.WriteScope.DenyRemove, ov.WriteScope.DenyRemove...)
			out[name] = merged
		}
	}
	return out
}

func appendUnique(dst []string, items ...string) []string {
	seen := make(map[string]struct{}, len(dst))
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
