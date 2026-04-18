package catalog

import (
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	embedded "github.com/mayckol/ai-bender/internal/embed"
)

// catalogEmbedPath is where the catalog lives inside the embedded defaults
// FS — callers never need to hardcode this.
const catalogEmbedPath = "bender/catalog.yaml"

// Load reads and validates the embedded catalog. Returns an error when the
// catalog references paths or pipeline node ids that would not resolve at
// scaffold time — better to fail here than to ship an install-time surprise.
func Load() (*Catalog, error) {
	return LoadFS(embedded.FS())
}

// LoadFS is the same as Load but lets tests inject a custom fs.FS rooted at
// the embedded-defaults top level (i.e. the directory that holds `bender/`
// and `claude/` sub-trees).
func LoadFS(root fs.FS) (*Catalog, error) {
	data, err := fs.ReadFile(root, catalogEmbedPath)
	if err != nil {
		return nil, fmt.Errorf("catalog: read %s: %w", catalogEmbedPath, err)
	}
	var cat Catalog
	if err := yaml.Unmarshal(data, &cat); err != nil {
		return nil, fmt.Errorf("catalog: parse %s: %w", catalogEmbedPath, err)
	}
	if err := cat.Validate(root); err != nil {
		return nil, err
	}
	return &cat, nil
}

// Validate checks referential integrity: every declared path resolves in the
// embedded FS, every pipeline_nodes id appears in the embedded pipeline, and
// depends_on targets exist in the same catalog.
func (c *Catalog) Validate(root fs.FS) error {
	if c.SchemaVersion != 1 {
		return fmt.Errorf("catalog: unsupported schema_version %d (want 1)", c.SchemaVersion)
	}
	if len(c.Components) == 0 {
		return errors.New("catalog: no components declared")
	}

	nodeIDs, err := pipelineNodeIDs(root)
	if err != nil {
		return fmt.Errorf("catalog: read pipeline node ids: %w", err)
	}

	var issues []string
	for id, comp := range c.Components {
		if comp.Description == "" {
			issues = append(issues, fmt.Sprintf("%s: description is required", id))
		}
		if comp.Paths.Agent != "" {
			if _, err := fs.Stat(root, comp.Paths.Agent); err != nil {
				issues = append(issues, fmt.Sprintf("%s: agent path %q not in embedded FS", id, comp.Paths.Agent))
			}
		}
		for _, sk := range comp.Paths.Skills {
			if _, err := fs.Stat(root, sk); err != nil {
				issues = append(issues, fmt.Sprintf("%s: skill path %q not in embedded FS", id, sk))
			}
		}
		for _, nid := range comp.Paths.PipelineNodes {
			if _, ok := nodeIDs[nid]; !ok {
				issues = append(issues, fmt.Sprintf("%s: pipeline_nodes id %q not in embedded pipeline.yaml", id, nid))
			}
		}
		for _, dep := range comp.DependsOn {
			if _, ok := c.Components[dep]; !ok {
				issues = append(issues, fmt.Sprintf("%s: depends_on %q is not a known component", id, dep))
			}
		}
	}

	if len(issues) > 0 {
		sort.Strings(issues)
		return fmt.Errorf("catalog invalid:\n  - %s", strings.Join(issues, "\n  - "))
	}
	return nil
}

// OptionalIDs returns the catalog's optional component ids, sorted.
func (c *Catalog) OptionalIDs() []string {
	out := make([]string, 0, len(c.Components))
	for id, comp := range c.Components {
		if comp.Optional {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}

// MandatoryIDs returns the catalog's mandatory component ids, sorted.
func (c *Catalog) MandatoryIDs() []string {
	out := make([]string, 0, len(c.Components))
	for id, comp := range c.Components {
		if !comp.Optional {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}

// IDs returns every component id, sorted.
func (c *Catalog) IDs() []string {
	out := make([]string, 0, len(c.Components))
	for id := range c.Components {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// pipelineNodeIDs extracts the set of node ids from the embedded pipeline.yaml
// so Validate can check catalog.pipeline_nodes references against them.
func pipelineNodeIDs(root fs.FS) (map[string]struct{}, error) {
	data, err := fs.ReadFile(root, "bender/pipeline.yaml")
	if err != nil {
		return nil, err
	}
	var raw struct {
		Nodes []struct {
			ID string `yaml:"id"`
		} `yaml:"nodes"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	out := make(map[string]struct{}, len(raw.Nodes))
	for _, n := range raw.Nodes {
		if n.ID != "" {
			out[n.ID] = struct{}{}
		}
	}
	return out, nil
}
