package pipeline

import "sort"

// Batch is one round of the walker: the set of node ids the orchestrator
// should dispatch in a single message. A batch with one entry means a solo
// dispatch; two or more means a parallel_dispatch.
type Batch struct {
	Wave  int
	Nodes []string
}

// DryRun simulates a full walk of the pipeline assuming every node succeeds.
// It returns the dispatch batches in order. `activeVars` maps variable names
// to concrete values; any node whose `when` evaluates false is silently
// skipped (resolved, but not dispatched).
//
// This is the algorithm the orchestrator SKILL is instructed to follow — Go
// implements it here so `bender pipeline dry-run` can preview the plan and
// tests can catch regressions.
func DryRun(p *Pipeline, activeVars map[string]any) ([]Batch, error) {
	nodeByID := make(map[string]*Node, len(p.Nodes))
	for i := range p.Nodes {
		nodeByID[p.Nodes[i].ID] = &p.Nodes[i]
	}

	status := make(map[string]string, len(p.Nodes)) // "pending" | "resolved" | "skipped"
	for _, n := range p.Nodes {
		if !whenSatisfied(n.When, activeVars) {
			status[n.ID] = "skipped"
			continue
		}
		status[n.ID] = "pending"
	}

	max := p.EffectiveMaxConcurrent()

	var batches []Batch
	wave := 0
	for {
		ready := readyNodes(p.Nodes, status)
		if len(ready) == 0 {
			break
		}
		sort.SliceStable(ready, func(i, j int) bool {
			a, b := nodeByID[ready[i]], nodeByID[ready[j]]
			if a.Priority != b.Priority {
				return a.Priority > b.Priority
			}
			return a.ID < b.ID
		})
		if len(ready) > max {
			ready = ready[:max]
		}
		batches = append(batches, Batch{Wave: wave, Nodes: ready})
		for _, id := range ready {
			status[id] = "resolved"
		}
		wave++
	}
	return batches, nil
}

func readyNodes(all []Node, status map[string]string) []string {
	var ready []string
	for i := range all {
		n := &all[i]
		if status[n.ID] != "pending" {
			continue
		}
		if depsSatisfied(n, status) {
			ready = append(ready, n.ID)
		}
	}
	return ready
}

func depsSatisfied(n *Node, status map[string]string) bool {
	if len(n.DependsOn) == 0 {
		return true
	}
	mode := n.EffectiveDependsMode()
	anyResolved := false
	for _, dep := range n.DependsOn {
		st := status[dep]
		switch mode {
		case DependsAll:
			// Skipped dependencies count as "resolved" — the branch was
			// never active, so downstream work shouldn't block on it.
			if st != "resolved" && st != "skipped" {
				return false
			}
		case DependsAny:
			if st == "resolved" {
				anyResolved = true
			}
		}
	}
	if mode == DependsAny {
		return anyResolved
	}
	return true
}

// whenSatisfied evaluates the (intentionally tiny) when-expression grammar.
// Unrecognised expressions resolve to true so a missing/complex `when`
// doesn't silently disable a node — the validator is what rejects those.
func whenSatisfied(expr string, vars map[string]any) bool {
	if expr == "" {
		return true
	}
	m := whenExpr.FindStringSubmatch(expr)
	if m == nil {
		return true
	}
	lhs := m[1]
	op := m[2]
	rhs := normaliseLiteral(m[3])
	actual, ok := vars[lhs]
	if !ok {
		return false
	}
	actualStr := normaliseValue(actual)
	switch op {
	case "==":
		return actualStr == rhs
	case "!=":
		return actualStr != rhs
	}
	return true
}

func normaliseLiteral(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

func normaliseValue(v any) string {
	switch x := v.(type) {
	case bool:
		if x {
			return "true"
		}
		return "false"
	case string:
		return x
	default:
		return ""
	}
}
