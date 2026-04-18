package catalog

// Catalog is the top-level shape of the embedded component catalog.
type Catalog struct {
	SchemaVersion int                  `yaml:"schema_version"`
	Components    map[string]Component `yaml:"components"`
}

// Component describes a single scaffoldable agent (and its owned skills +
// pipeline nodes). Zero-value Optional means mandatory; zero-value Default is
// interpreted as true by Normalize so a component newly-added to the catalog
// is installed by default on existing workspaces.
type Component struct {
	Optional    bool     `yaml:"optional"`
	Description string   `yaml:"description"`
	Paths       Paths    `yaml:"paths"`
	DependsOn   []string `yaml:"depends_on,omitempty"`
	Default     *bool    `yaml:"default,omitempty"`
}

// Paths groups the embedded filesystem entries a component owns plus the
// pipeline node ids it contributes to `bender/pipeline.yaml`.
type Paths struct {
	Agent         string   `yaml:"agent,omitempty"`
	Skills        []string `yaml:"skills,omitempty"`
	PipelineNodes []string `yaml:"pipeline_nodes,omitempty"`
}

// DefaultSelected returns the component's catalog-declared default selection
// state. Absent field ⇒ true (so catalog additions install by default).
func (c Component) DefaultSelected() bool {
	if c.Default == nil {
		return true
	}
	return *c.Default
}
