// Package agent loads agent definitions from markdown files with YAML frontmatter.
// Agent definitions describe persona, write scope, and skill selectors; they are read by Claude Code
// (or any executor) and validated by `bender doctor`.
package agent

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"sort"

	"gopkg.in/yaml.v3"

	"github.com/mayckol/ai-bender/internal/types"
)

// WriteScope partitions the project tree for an agent. `Deny` wins on conflict.
type WriteScope struct {
	Allow []string `yaml:"allow"`
	Deny  []string `yaml:"deny"`
}

// Frontmatter is the YAML metadata at the top of every agent file.
type Frontmatter struct {
	Name        string              `yaml:"name"`
	Purpose     string              `yaml:"purpose"`
	PersonaHint string              `yaml:"persona_hint"`
	WriteScope  WriteScope          `yaml:"write_scope"`
	Skills      types.SkillSelector `yaml:"skills"`
	Context     []types.Context     `yaml:"context,omitempty"`
	InvokedBy   []types.Stage       `yaml:"invoked_by"`
}

// Agent is a parsed agent definition: frontmatter + markdown body + provenance.
type Agent struct {
	Frontmatter
	Body   string
	Path   string
	Origin types.Origin
}

var (
	nameRe           = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}$`)
	frontmatterDelim = []byte("---")
)

// ErrMissingFrontmatter is returned when an agent file does not begin with `---`.
var ErrMissingFrontmatter = errors.New("agent: missing yaml frontmatter")

// ParseAgent parses a single agent markdown into an Agent value.
func ParseAgent(data []byte) (*Agent, error) {
	fm, body, err := splitFrontmatter(data)
	if err != nil {
		return nil, err
	}
	var f Frontmatter
	if err := yaml.Unmarshal(fm, &f); err != nil {
		return nil, fmt.Errorf("agent: parse frontmatter: %w", err)
	}
	a := &Agent{Frontmatter: f, Body: string(body)}
	if err := a.validate(); err != nil {
		return nil, err
	}
	return a, nil
}

func (a *Agent) validate() error {
	if !nameRe.MatchString(a.Name) {
		return fmt.Errorf("agent: name %q does not match %s", a.Name, nameRe)
	}
	if a.Purpose == "" {
		return fmt.Errorf("agent %q: purpose is required", a.Name)
	}
	if a.PersonaHint == "" {
		return fmt.Errorf("agent %q: persona_hint is required", a.Name)
	}
	if len(a.InvokedBy) == 0 {
		return fmt.Errorf("agent %q: invoked_by must be non-empty", a.Name)
	}
	for _, s := range a.InvokedBy {
		if !isKnownStage(s) {
			return fmt.Errorf("agent %q: unknown invoked_by stage %q", a.Name, s)
		}
	}
	for _, c := range a.Context {
		if c != types.ContextFG && c != types.ContextBG {
			return fmt.Errorf("agent %q: context must be fg or bg, got %q", a.Name, c)
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

func splitFrontmatter(data []byte) (frontmatter, body []byte, err error) {
	if !bytes.HasPrefix(data, frontmatterDelim) {
		return nil, nil, ErrMissingFrontmatter
	}
	rest := data[len(frontmatterDelim):]
	rest = bytes.TrimLeft(rest, "\r\n")
	end := bytes.Index(rest, append([]byte("\n"), frontmatterDelim...))
	if end < 0 {
		return nil, nil, fmt.Errorf("agent: frontmatter not terminated")
	}
	frontmatter = rest[:end]
	body = bytes.TrimLeft(rest[end+1+len(frontmatterDelim):], "\r\n")
	return frontmatter, body, nil
}

// Registry is the deterministic registry of agents loaded from the embedded defaults plus user overrides.
type Registry struct {
	agents map[string]*Agent
	order  []string
}

// LoadRegistry walks defaultsFS first then userFS; same-name files in userFS fully replace embedded defaults.
func LoadRegistry(defaultsFS, userFS fs.FS) (*Registry, []string, error) {
	r := &Registry{agents: map[string]*Agent{}}
	var warnings []string
	if defaultsFS != nil {
		w, err := r.walk(defaultsFS, "claude/agents", types.OriginEmbedded)
		if err != nil {
			return nil, nil, fmt.Errorf("load embedded agents: %w", err)
		}
		warnings = append(warnings, w...)
	}
	if userFS != nil {
		w, err := r.walk(userFS, "agents", types.OriginUser)
		if err != nil {
			return nil, nil, fmt.Errorf("load user agents: %w", err)
		}
		warnings = append(warnings, w...)
	}
	r.reindex()
	return r, warnings, nil
}

func (r *Registry) walk(root fs.FS, base string, origin types.Origin) ([]string, error) {
	var warnings []string
	err := fs.WalkDir(root, base, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		if d.IsDir() {
			return nil
		}
		if path.Ext(p) != ".md" {
			return nil
		}
		data, readErr := fs.ReadFile(root, p)
		if readErr != nil {
			return fmt.Errorf("read %s: %w", p, readErr)
		}
		a, parseErr := ParseAgent(data)
		if parseErr != nil {
			warnings = append(warnings, fmt.Sprintf("%s: %v", p, parseErr))
			return nil
		}
		a.Path = p
		a.Origin = origin
		r.agents[a.Name] = a
		return nil
	})
	if err != nil {
		return warnings, err
	}
	return warnings, nil
}

func (r *Registry) reindex() {
	r.order = r.order[:0]
	for name := range r.agents {
		r.order = append(r.order, name)
	}
	sort.Strings(r.order)
}

// Get returns the agent registered under name, or nil when absent.
func (r *Registry) Get(name string) *Agent { return r.agents[name] }

// Names returns the registered agent names in deterministic (alphabetic) order.
func (r *Registry) Names() []string {
	out := make([]string, len(r.order))
	copy(out, r.order)
	return out
}

// All returns every agent in deterministic order.
func (r *Registry) All() []*Agent {
	out := make([]*Agent, 0, len(r.order))
	for _, n := range r.order {
		out = append(out, r.agents[n])
	}
	return out
}

// Len returns the number of registered agents.
func (r *Registry) Len() int { return len(r.agents) }
