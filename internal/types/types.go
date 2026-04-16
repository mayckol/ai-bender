// Package types holds enums and selector shapes shared by the skill, agent, group, and resolver packages.
// Centralizing them here prevents circular imports.
package types

type Context string

const (
	ContextFG Context = "fg"
	ContextBG Context = "bg"
)

type Stage string

const (
	StageBootstrap Stage = "bootstrap"
	StageCry       Stage = "cry"
	StagePlan      Stage = "plan"
	StageTDD       Stage = "tdd"
	StageGHU       Stage = "ghu"
	StageImplement Stage = "implement"
	StageDoctor    Stage = "doctor"
)

func KnownStages() []Stage {
	return []Stage{StageBootstrap, StageCry, StagePlan, StageTDD, StageGHU, StageImplement, StageDoctor}
}

type IssueType string

const (
	IssueAny           IssueType = "any"
	IssueBug           IssueType = "bug"
	IssueFeature       IssueType = "feature"
	IssuePerformance   IssueType = "performance"
	IssueArchitectural IssueType = "architectural"
)

func KnownIssueTypes() []IssueType {
	return []IssueType{IssueAny, IssueBug, IssueFeature, IssuePerformance, IssueArchitectural}
}

type Origin int

const (
	OriginEmbedded Origin = iota
	OriginUser
)

func (o Origin) String() string {
	if o == OriginUser {
		return "user"
	}
	return "embedded"
}

// TagSelector expresses set inclusion + exclusion over `provides` tags.
type TagSelector struct {
	AnyOf  []string `yaml:"any_of,omitempty"`
	NoneOf []string `yaml:"none_of,omitempty"`
}

// SkillSelector is the shape used by both agents and groups to bind skills.
type SkillSelector struct {
	Explicit []string    `yaml:"explicit,omitempty"`
	Patterns []string    `yaml:"patterns,omitempty"`
	Tags     TagSelector `yaml:"tags,omitempty"`
}

// IsEmpty returns true when the selector binds nothing.
func (s SkillSelector) IsEmpty() bool {
	return len(s.Explicit) == 0 && len(s.Patterns) == 0 && len(s.Tags.AnyOf) == 0
}

// ResolveContext is the runtime context passed to the selector resolver.
type ResolveContext struct {
	Stage         Stage
	IssueType     IssueType
	AgentContexts []Context
}
