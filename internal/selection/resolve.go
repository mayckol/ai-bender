package selection

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/mayckol/ai-bender/internal/catalog"
)

// Flags carries the CLI-side `--with` / `--without` values. Callers populate
// these directly from cobra's StringSlice flags.
type Flags struct {
	With    []string
	Without []string
}

// Resolve computes the final per-component selection map for an init run,
// applying precedence: catalog defaults → persisted manifest → CLI flags.
// Rejects mandatory deselection, unknown component ids, and contradictions.
func Resolve(cat *catalog.Catalog, m *Manifest, f Flags) (map[string]bool, error) {
	if cat == nil {
		return nil, errors.New("selection.Resolve: catalog is required")
	}

	// 1. Start from catalog defaults. Mandatory ⇒ true; optional ⇒ its
	//    `default` field (defaulting to true).
	result := make(map[string]bool, len(cat.Components))
	for id, comp := range cat.Components {
		if !comp.Optional {
			result[id] = true
			continue
		}
		result[id] = comp.DefaultSelected()
	}

	// 2. Apply persisted manifest (for optional components only).
	if m != nil {
		for id, entry := range m.Components {
			comp, ok := cat.Components[id]
			if !ok {
				return nil, fmt.Errorf("selection: manifest names unknown component %q", id)
			}
			if !comp.Optional {
				// Mandatory entries in the manifest are ignored; they are
				// always true.
				continue
			}
			result[id] = entry.Selected
		}
	}

	// 3. Reject contradictions in the flags themselves.
	if dup := overlap(f.With, f.Without); len(dup) > 0 {
		return nil, fmt.Errorf("selection: --with and --without name the same component(s): %v", dup)
	}

	// 4. Apply --with then --without.
	for _, id := range f.With {
		comp, ok := cat.Components[id]
		if !ok {
			return nil, unknownErr(cat, id)
		}
		if !comp.Optional {
			// Including a mandatory component is a no-op (it's already true)
			// — don't error; this keeps scripts simple.
			continue
		}
		result[id] = true
	}
	for _, id := range f.Without {
		comp, ok := cat.Components[id]
		if !ok {
			return nil, unknownErr(cat, id)
		}
		if !comp.Optional {
			return nil, fmt.Errorf("selection: %q is mandatory and cannot be deselected. Optional components: %v", id, cat.OptionalIDs())
		}
		result[id] = false
	}

	return result, nil
}

// overlap returns the ids that appear in both lists.
func overlap(a, b []string) []string {
	set := map[string]struct{}{}
	for _, x := range a {
		set[x] = struct{}{}
	}
	var out []string
	for _, x := range b {
		if _, ok := set[x]; ok {
			out = append(out, x)
		}
	}
	sort.Strings(out)
	return out
}

func unknownErr(cat *catalog.Catalog, id string) error {
	optional := cat.OptionalIDs()
	return fmt.Errorf("selection: unknown component %q. Optional components are: [%s]", id, strings.Join(optional, ", "))
}
