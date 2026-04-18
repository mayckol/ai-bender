package render

import (
	"bytes"
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/mayckol/ai-bender/internal/catalog"
)

// Pipeline re-renders pipeline.yaml against the resolved selection, dropping
// nodes whose agent belongs to a deselected component and pruning
// `depends_on` references to dropped nodes. Returns the marshalled bytes
// and the set of dropped node ids (for the init summary + conflict report).
func Pipeline(src []byte, cat *catalog.Catalog, sel map[string]bool) ([]byte, []string, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(src, &doc); err != nil {
		return nil, nil, fmt.Errorf("render: parse pipeline.yaml: %w", err)
	}

	dropped := droppedNodeIDs(cat, sel)

	if doc.Kind == yaml.DocumentNode && len(doc.Content) == 1 {
		pruneNodes(doc.Content[0], dropped)
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		return nil, nil, fmt.Errorf("render: marshal pipeline.yaml: %w", err)
	}
	_ = enc.Close()

	out := make([]string, 0, len(dropped))
	for id := range dropped {
		out = append(out, id)
	}
	return buf.Bytes(), out, nil
}

// droppedNodeIDs returns the set of pipeline node ids that belong to
// deselected components.
func droppedNodeIDs(cat *catalog.Catalog, sel map[string]bool) map[string]struct{} {
	out := map[string]struct{}{}
	for id, comp := range cat.Components {
		if sel[id] {
			continue
		}
		for _, nid := range comp.Paths.PipelineNodes {
			out[nid] = struct{}{}
		}
	}
	return out
}

// pruneNodes mutates a top-level mapping node in place, dropping any
// `nodes:` entry whose `id` is in `dropped` and stripping references to
// dropped ids out of every remaining node's `depends_on:`.
func pruneNodes(top *yaml.Node, dropped map[string]struct{}) {
	if top.Kind != yaml.MappingNode {
		return
	}
	var nodesList *yaml.Node
	for i := 0; i+1 < len(top.Content); i += 2 {
		if top.Content[i].Value == "nodes" {
			nodesList = top.Content[i+1]
			break
		}
	}
	if nodesList == nil || nodesList.Kind != yaml.SequenceNode {
		return
	}
	kept := nodesList.Content[:0]
	for _, node := range nodesList.Content {
		id := mapValue(node, "id")
		if _, drop := dropped[id]; drop {
			continue
		}
		if depsNode := mapChild(node, "depends_on"); depsNode != nil && depsNode.Kind == yaml.SequenceNode {
			pruned := depsNode.Content[:0]
			for _, depVal := range depsNode.Content {
				if _, drop := dropped[depVal.Value]; drop {
					continue
				}
				pruned = append(pruned, depVal)
			}
			depsNode.Content = pruned
		}
		kept = append(kept, node)
	}
	nodesList.Content = kept
}

// mapValue returns the string scalar associated with `key` in a mapping
// node, or "" if absent.
func mapValue(n *yaml.Node, key string) string {
	c := mapChild(n, key)
	if c == nil {
		return ""
	}
	return c.Value
}

// mapChild returns the *yaml.Node associated with `key` in a mapping node,
// or nil if absent or `n` is not a mapping.
func mapChild(n *yaml.Node, key string) *yaml.Node {
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return n.Content[i+1]
		}
	}
	return nil
}
