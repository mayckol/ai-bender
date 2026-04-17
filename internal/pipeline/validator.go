package pipeline

import (
	"fmt"
	"regexp"
	"strings"
)

// Violation is one schema or reference error returned by Validate.
type Violation struct {
	NodeID  string
	Message string
}

func (v Violation) String() string {
	if v.NodeID == "" {
		return v.Message
	}
	return fmt.Sprintf("node %q: %s", v.NodeID, v.Message)
}

// Catalog is the minimal view of the skill + agent catalogs that the pipeline
// validator needs. Kept as an interface so doctor can wire in its own types
// without creating a circular import.
type Catalog interface {
	HasSkill(name string) bool
	HasAgent(name string) bool
}

// Validate walks the pipeline and reports every issue. An empty slice means
// the pipeline is contract-compliant.
//
// Checks:
//  1. ids unique
//  2. depends_on references existing ids
//  3. agent/skill references exist in the catalog (skipped if cat is nil)
//  4. every `when` reference names a declared variable
//  5. VariableDef shapes are consistent with their Kind
//  6. no cycles (Kahn's algorithm)
//  7. orchestrator nodes must not declare agent/skill
//  8. agent nodes must declare both agent and skill
func Validate(p *Pipeline, cat Catalog) []Violation {
	var out []Violation
	ids := make(map[string]bool, len(p.Nodes))
	for _, n := range p.Nodes {
		if n.ID == "" {
			out = append(out, Violation{Message: "node is missing id"})
			continue
		}
		if ids[n.ID] {
			out = append(out, Violation{NodeID: n.ID, Message: "duplicate id"})
			continue
		}
		ids[n.ID] = true
	}
	for _, n := range p.Nodes {
		out = append(out, validateShape(n, cat)...)
		out = append(out, validateDeps(n, ids)...)
		out = append(out, validateWhen(n, p.Variables)...)
	}
	for varName, def := range p.Variables {
		out = append(out, validateVariable(varName, def)...)
	}
	if cyc := detectCycle(p.Nodes); cyc != nil {
		out = append(out, Violation{Message: fmt.Sprintf("cycle detected: %s", strings.Join(cyc, " → "))})
	}
	return out
}

func validateShape(n Node, cat Catalog) []Violation {
	var out []Violation
	switch n.EffectiveType() {
	case NodeAgent:
		if n.Agent == "" {
			out = append(out, Violation{NodeID: n.ID, Message: "agent is required (type=agent)"})
		}
		if n.Skill == "" {
			out = append(out, Violation{NodeID: n.ID, Message: "skill is required (type=agent)"})
		}
		if cat != nil && n.Agent != "" && !cat.HasAgent(n.Agent) {
			out = append(out, Violation{NodeID: n.ID, Message: fmt.Sprintf("agent %q not in catalog", n.Agent)})
		}
		if cat != nil && n.Skill != "" && !cat.HasSkill(n.Skill) {
			out = append(out, Violation{NodeID: n.ID, Message: fmt.Sprintf("skill %q not in catalog", n.Skill)})
		}
	case NodeOrchestrator:
		if n.Agent != "" || n.Skill != "" {
			out = append(out, Violation{NodeID: n.ID, Message: "orchestrator nodes must not declare agent or skill"})
		}
	default:
		out = append(out, Violation{NodeID: n.ID, Message: fmt.Sprintf("unknown type %q (want agent|orchestrator)", n.Type)})
	}
	switch n.EffectiveDependsMode() {
	case DependsAll, DependsAny:
	default:
		out = append(out, Violation{NodeID: n.ID, Message: fmt.Sprintf("unknown depends_mode %q (want all-resolved|any-resolved)", n.DependsMode)})
	}
	return out
}

func validateDeps(n Node, ids map[string]bool) []Violation {
	var out []Violation
	seen := make(map[string]bool, len(n.DependsOn))
	for _, dep := range n.DependsOn {
		if dep == n.ID {
			out = append(out, Violation{NodeID: n.ID, Message: "depends on itself"})
			continue
		}
		if seen[dep] {
			out = append(out, Violation{NodeID: n.ID, Message: fmt.Sprintf("duplicate dependency %q", dep)})
			continue
		}
		seen[dep] = true
		if !ids[dep] {
			out = append(out, Violation{NodeID: n.ID, Message: fmt.Sprintf("depends_on %q is not a known node id", dep)})
		}
	}
	return out
}

// whenExpr matches `<ident> == <literal>` / `<ident> != <literal>` with any
// whitespace. Literals are: true | false | quoted string | bare identifier.
var whenExpr = regexp.MustCompile(`^\s*([A-Za-z_][A-Za-z0-9_.]*)\s*(==|!=)\s*(true|false|"[^"]*"|[A-Za-z0-9_.-]+)\s*$`)

func validateWhen(n Node, vars map[string]VariableDef) []Violation {
	if n.When == "" {
		return nil
	}
	m := whenExpr.FindStringSubmatch(n.When)
	if m == nil {
		return []Violation{{NodeID: n.ID, Message: fmt.Sprintf("when %q is not a supported expression (want `<var> == <literal>` or `<var> != <literal>`)", n.When)}}
	}
	varName := m[1]
	if _, ok := vars[varName]; !ok {
		return []Violation{{NodeID: n.ID, Message: fmt.Sprintf("when references undeclared variable %q", varName)}}
	}
	return nil
}

func validateVariable(name string, def VariableDef) []Violation {
	switch def.Kind {
	case VarGlobApproved:
		if def.Pattern == "" {
			return []Violation{{Message: fmt.Sprintf("variable %q: kind=glob_nonempty_with_status requires pattern", name)}}
		}
		if def.RequireStatus == "" {
			return []Violation{{Message: fmt.Sprintf("variable %q: kind=glob_nonempty_with_status requires require_status", name)}}
		}
	case VarPlanFlag:
		if def.Flag == "" {
			return []Violation{{Message: fmt.Sprintf("variable %q: kind=plan_flag requires flag", name)}}
		}
	case VarLiteral:
		if def.Value == nil {
			return []Violation{{Message: fmt.Sprintf("variable %q: kind=literal requires value", name)}}
		}
	default:
		return []Violation{{Message: fmt.Sprintf("variable %q: unknown kind %q", name, def.Kind)}}
	}
	return nil
}

// detectCycle runs Kahn's algorithm over the DAG and returns a human-readable
// cycle if one exists, or nil.
func detectCycle(nodes []Node) []string {
	incoming := make(map[string]int, len(nodes))
	byID := make(map[string]*Node, len(nodes))
	children := make(map[string][]string, len(nodes))
	for i := range nodes {
		n := &nodes[i]
		byID[n.ID] = n
		if _, ok := incoming[n.ID]; !ok {
			incoming[n.ID] = 0
		}
	}
	for i := range nodes {
		n := &nodes[i]
		for _, dep := range n.DependsOn {
			if _, ok := byID[dep]; !ok {
				// Missing dep surfaces separately via validateDeps; skip here.
				continue
			}
			children[dep] = append(children[dep], n.ID)
			incoming[n.ID]++
		}
	}
	q := make([]string, 0, len(nodes))
	for id, in := range incoming {
		if in == 0 {
			q = append(q, id)
		}
	}
	resolved := 0
	for len(q) > 0 {
		head := q[0]
		q = q[1:]
		resolved++
		for _, c := range children[head] {
			incoming[c]--
			if incoming[c] == 0 {
				q = append(q, c)
			}
		}
	}
	if resolved == len(nodes) {
		return nil
	}
	// Find one remaining node to expose as the cycle entry point. We don't
	// reconstruct the full cycle path in v1 — the entry point is usually
	// enough to locate the problem.
	for id, in := range incoming {
		if in > 0 {
			return []string{id, "…"}
		}
	}
	return []string{"unknown"}
}
