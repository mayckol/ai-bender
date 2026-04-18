package catalog

import "sort"

// BreakReport identifies a selection change that would leave a dependent
// component without its required dependency. When Deselected is "benchmarker"
// and Dependents is ["mistakeinator"], the user tried to remove benchmarker
// while mistakeinator (still selected) declares it as a dep.
type BreakReport struct {
	Deselected string
	Dependents []string
}

// DetectBreaks walks every currently-selected component and checks whether
// any of its depends_on entries would be deselected in `target`. The result
// is keyed by the deselected component id so the caller can surface a
// per-component error.
func DetectBreaks(cat *Catalog, target map[string]bool) []BreakReport {
	reports := map[string][]string{}
	for id, comp := range cat.Components {
		if !target[id] {
			continue
		}
		for _, dep := range comp.DependsOn {
			if !target[dep] {
				reports[dep] = append(reports[dep], id)
			}
		}
	}
	out := make([]BreakReport, 0, len(reports))
	for dep, dependents := range reports {
		sort.Strings(dependents)
		out = append(out, BreakReport{Deselected: dep, Dependents: dependents})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Deselected < out[j].Deselected })
	return out
}

// CascadeDeselect walks the dependency graph from a seed of deselections and
// returns the smallest target selection map that is consistent: for every
// deselected component, every component that depends on it transitively is
// also deselected. Idempotent; order of `seed` does not matter.
func CascadeDeselect(cat *Catalog, current map[string]bool, seed []string) map[string]bool {
	out := make(map[string]bool, len(current))
	for k, v := range current {
		out[k] = v
	}
	for _, id := range seed {
		out[id] = false
	}
	// Repeatedly sweep: any component whose dep is false becomes false.
	for {
		changed := false
		for id, comp := range cat.Components {
			if !out[id] {
				continue
			}
			for _, dep := range comp.DependsOn {
				if !out[dep] {
					out[id] = false
					changed = true
					break
				}
			}
		}
		if !changed {
			break
		}
	}
	return out
}
