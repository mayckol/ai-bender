// Package pipeline parses and validates the declarative execution graph that
// `/ghu` and `/implement` walk. The graph lives at `.claude/pipeline.yaml`
// (with an embedded default). Two sibling nodes whose dependencies are all
// resolved run concurrently — parallelism is emergent from the DAG, not a
// per-group flag. Priority only breaks ties when more nodes are ready than
// `max_concurrent` allows.
package pipeline

// SchemaVersion is the current pipeline.yaml schema the loader accepts.
const SchemaVersion = 1

// DefaultMaxConcurrent caps parallel Agent dispatches when `pipeline.max_concurrent`
// is unset or zero. High enough that typical 3–4-wide fan-outs aren't capped,
// low enough that a pathological DAG can't flood the host.
const DefaultMaxConcurrent = 8

// NodeType distinguishes agent-dispatched work from orchestrator-owned work
// (e.g. the final report, which isn't delegated).
type NodeType string

const (
	// NodeAgent is a node that dispatches to a specialist subagent via the
	// Agent tool. The common case.
	NodeAgent NodeType = "agent"
	// NodeOrchestrator is a node the orchestrator itself executes — no Agent
	// tool call, no subagent. Used for the final report step.
	NodeOrchestrator NodeType = "orchestrator"
)

// DependsMode controls how a node's depends_on list is evaluated.
//
//   - DependsAll (default): every listed dependency must have resolved
//     successfully (completed or skipped-via-when).
//   - DependsAny: any one resolved dependency is sufficient — used where a
//     graph branches on `when` and the downstream node only needs one of the
//     branches to have landed.
type DependsMode string

const (
	DependsAll DependsMode = "all-resolved"
	DependsAny DependsMode = "any-resolved"
)

// Pipeline is the top-level document.
type Pipeline struct {
	SchemaVersion int                    `yaml:"schema_version"`
	Meta          Meta                   `yaml:"pipeline"`
	Variables     map[string]VariableDef `yaml:"variables,omitempty"`
	Nodes         []Node                 `yaml:"nodes"`

	// SourcePath is set by LoadFromFS for error messages. Not serialized.
	SourcePath string `yaml:"-"`
}

// Meta is the pipeline's header.
type Meta struct {
	ID            string `yaml:"id"`
	Description   string `yaml:"description"`
	MaxConcurrent int    `yaml:"max_concurrent,omitempty"`
	HaltOnFailure bool   `yaml:"halt_on_failure,omitempty"`
}

// Node is one entry in the DAG.
type Node struct {
	ID            string      `yaml:"id"`
	Type          NodeType    `yaml:"type,omitempty"`
	Agent         string      `yaml:"agent,omitempty"`
	Skill         string      `yaml:"skill,omitempty"`
	DependsOn     []string    `yaml:"depends_on,omitempty"`
	DependsMode   DependsMode `yaml:"depends_mode,omitempty"`
	Priority      int         `yaml:"priority,omitempty"`
	When          string      `yaml:"when,omitempty"`
	Required      bool        `yaml:"required,omitempty"`
	HaltOnFailure bool        `yaml:"halt_on_failure,omitempty"`
}

// VariableKind is the discriminator for VariableDef. Only a small, closed set
// of kinds is supported in v1 — we intentionally avoid a full expression
// engine until we see a real need.
type VariableKind string

const (
	// VarGlobApproved is true iff the glob matches at least one file and every
	// matched file's YAML frontmatter has `status: <require_status>`.
	VarGlobApproved VariableKind = "glob_nonempty_with_status"
	// VarPlanFlag reads a boolean from the latest approved plan's frontmatter.
	VarPlanFlag VariableKind = "plan_flag"
	// VarLiteral hardcodes a value — useful for per-project overrides.
	VarLiteral VariableKind = "literal"
)

// VariableDef describes how one `when`-referenced variable is evaluated at
// orchestrator start. Kept intentionally small.
type VariableDef struct {
	Kind          VariableKind `yaml:"kind"`
	Pattern       string       `yaml:"pattern,omitempty"`
	RequireStatus string       `yaml:"require_status,omitempty"`
	Flag          string       `yaml:"flag,omitempty"`
	Value         any          `yaml:"value,omitempty"`
}

// EffectiveMaxConcurrent returns the cap to use when Meta.MaxConcurrent is
// zero or negative. Resolution order:
//  1. explicit pipeline.max_concurrent from YAML,
//  2. BENDER_MAX_CONCURRENT env var,
//  3. host-memory-aware auto cap,
//  4. DefaultMaxConcurrent.
func (p *Pipeline) EffectiveMaxConcurrent() int {
	if p.Meta.MaxConcurrent > 0 {
		return p.Meta.MaxConcurrent
	}
	if n := envMaxConcurrent(); n > 0 {
		return n
	}
	if n := autoCapFromHost(); n > 0 {
		return n
	}
	return DefaultMaxConcurrent
}

// EffectiveDependsMode normalises DependsMode. Empty defaults to DependsAll.
func (n *Node) EffectiveDependsMode() DependsMode {
	if n.DependsMode == "" {
		return DependsAll
	}
	return n.DependsMode
}

// EffectiveType normalises NodeType. Empty defaults to NodeAgent.
func (n *Node) EffectiveType() NodeType {
	if n.Type == "" {
		return NodeAgent
	}
	return n.Type
}
