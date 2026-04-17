// Package group loads named skill selectors from groups.yaml.
package group

import (
	"errors"
	"fmt"
	"io/fs"
	"sort"

	"gopkg.in/yaml.v3"

	"github.com/mayckol/ai-bender/internal/types"
)

// Selector is a named selector usable by slash commands and orchestrator code.
type Selector struct {
	types.SkillSelector `yaml:",inline"`
	Context             []types.Context `yaml:"context,omitempty"`
}

// Group is one entry in groups.yaml.
type Group struct {
	Name           string   `yaml:"-"`
	Description    string   `yaml:"description"`
	Select         Selector `yaml:"select"`
	Ordered        bool     `yaml:"ordered,omitempty"`
	HaltOnFailure  bool     `yaml:"halt_on_failure,omitempty"`
}

// File is the on-disk shape of groups.yaml.
type File struct {
	Groups map[string]Group `yaml:"groups"`
}

// ErrEmptySelector is returned when a group declares no selectors at all.
var ErrEmptySelector = errors.New("group: select must declare at least one of explicit, patterns, tags")

// Parse parses the bytes of a groups.yaml file.
func Parse(data []byte) (map[string]*Group, error) {
	var f File
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("group: parse: %w", err)
	}
	out := map[string]*Group{}
	for name, g := range f.Groups {
		if name == "" {
			return nil, fmt.Errorf("group: empty group name")
		}
		if g.Description == "" {
			return nil, fmt.Errorf("group %q: description is required", name)
		}
		if g.Select.IsEmpty() {
			return nil, fmt.Errorf("group %q: %w", name, ErrEmptySelector)
		}
		gp := g
		gp.Name = name
		out[name] = &gp
	}
	return out, nil
}

// LoadFromFS reads `groups.yaml` at the given `base` path inside `root` and parses it.
// Callers pass `"bender/groups.yaml"` for the embedded FS and `"groups.yaml"` for a
// user FS rooted at `<project>/.bender/`. A missing file yields an empty map and no
// error so the caller can layer defaults and user files.
func LoadFromFS(root fs.FS, base string) (map[string]*Group, error) {
	data, err := fs.ReadFile(root, base)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return map[string]*Group{}, nil
		}
		return nil, fmt.Errorf("group: read %s: %w", base, err)
	}
	return Parse(data)
}

// Names returns the group names in deterministic order.
func Names(groups map[string]*Group) []string {
	out := make([]string, 0, len(groups))
	for name := range groups {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
