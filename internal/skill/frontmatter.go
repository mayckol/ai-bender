// Package skill loads, validates, and indexes skill markdown files declared with YAML frontmatter.
//
// A skill file lives at <root>/skills/<name>/SKILL.md (root is either the embedded defaults FS or a
// project's `.claude/`). The file format is:
//
//	---
//	<yaml frontmatter>
//	---
//	<markdown body>
package skill

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"

	"gopkg.in/yaml.v3"

	"github.com/mayckol/ai-bender/internal/types"
)

// Frontmatter is the YAML metadata at the top of every SKILL.md.
type Frontmatter struct {
	Name          string            `yaml:"name"`
	Context       types.Context     `yaml:"context"`
	Description   string            `yaml:"description"`
	Provides      []string          `yaml:"provides"`
	Stages        []types.Stage     `yaml:"stages"`
	AppliesTo     []types.IssueType `yaml:"applies_to"`
	Inputs        []string          `yaml:"inputs,omitempty"`
	Outputs       []string          `yaml:"outputs,omitempty"`
	RequiresTools []string          `yaml:"requires_tools,omitempty"`
}

// Skill is a parsed skill definition: frontmatter + markdown body + provenance.
type Skill struct {
	Frontmatter
	Body   string
	Path   string
	Origin types.Origin
}

// nameRe is the validation pattern for `name`. Matches `[a-z][a-z0-9-]{0,80}`.
var nameRe = regexp.MustCompile(`^[a-z][a-z0-9-]{0,80}$`)

// frontmatterDelim splits a markdown file with `---\n...\n---\n<body>` framing.
var frontmatterDelim = []byte("---")

// ParseFrontmatter parses a SKILL.md byte slice into a Skill. It performs structural validation;
// catalog-level checks (uniqueness, override semantics) live in the loader.
func ParseFrontmatter(data []byte) (*Skill, error) {
	fm, body, err := splitFrontmatter(data)
	if err != nil {
		return nil, err
	}
	var f Frontmatter
	if err := yaml.Unmarshal(fm, &f); err != nil {
		return nil, fmt.Errorf("skill: parse frontmatter: %w", err)
	}
	s := &Skill{Frontmatter: f, Body: string(body)}
	if err := s.validate(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Skill) validate() error {
	if !nameRe.MatchString(s.Name) {
		return fmt.Errorf("skill: name %q does not match %s", s.Name, nameRe)
	}
	if s.Context != types.ContextFG && s.Context != types.ContextBG {
		return fmt.Errorf("skill %q: context must be fg or bg, got %q", s.Name, s.Context)
	}
	if s.Description == "" {
		return fmt.Errorf("skill %q: description is required", s.Name)
	}
	if len(s.Provides) == 0 {
		return fmt.Errorf("skill %q: provides must be non-empty", s.Name)
	}
	if len(s.Stages) == 0 {
		return fmt.Errorf("skill %q: stages must be non-empty", s.Name)
	}
	for _, st := range s.Stages {
		if !isKnownStage(st) {
			return fmt.Errorf("skill %q: unknown stage %q", s.Name, st)
		}
	}
	if len(s.AppliesTo) == 0 {
		return fmt.Errorf("skill %q: applies_to must be non-empty", s.Name)
	}
	for _, it := range s.AppliesTo {
		if !isKnownIssueType(it) {
			return fmt.Errorf("skill %q: unknown applies_to %q", s.Name, it)
		}
	}
	return nil
}

func isKnownStage(s types.Stage) bool {
	for _, candidate := range types.KnownStages() {
		if s == candidate {
			return true
		}
	}
	return false
}

func isKnownIssueType(it types.IssueType) bool {
	for _, candidate := range types.KnownIssueTypes() {
		if it == candidate {
			return true
		}
	}
	return false
}

// splitFrontmatter extracts the YAML frontmatter and Markdown body from a SKILL.md file.
// Returns ErrMissingFrontmatter when the file does not start with `---`.
func splitFrontmatter(data []byte) (frontmatter, body []byte, err error) {
	if !bytes.HasPrefix(data, frontmatterDelim) {
		return nil, nil, ErrMissingFrontmatter
	}
	rest := data[len(frontmatterDelim):]
	rest = bytes.TrimLeft(rest, "\r\n")
	end := bytes.Index(rest, append([]byte("\n"), frontmatterDelim...))
	if end < 0 {
		return nil, nil, fmt.Errorf("skill: frontmatter not terminated")
	}
	frontmatter = rest[:end]
	body = bytes.TrimLeft(rest[end+1+len(frontmatterDelim):], "\r\n")
	return frontmatter, body, nil
}

// ErrMissingFrontmatter is returned when a markdown file does not begin with `---`.
var ErrMissingFrontmatter = errors.New("skill: missing yaml frontmatter")
