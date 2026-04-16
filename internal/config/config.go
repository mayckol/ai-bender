// Package config loads project-level .bender/config.yaml and applies the precedence rules from spec FR-029.
package config

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// AgentOverride encodes the per-agent additive composers usable in .bender/config.yaml.
// These are applied AFTER the resolver runs.
type AgentOverride struct {
	Skills struct {
		Add    []string `yaml:"add,omitempty"`
		Remove []string `yaml:"remove,omitempty"`
	} `yaml:"skills,omitempty"`
	WriteScope struct {
		AllowAdd    []string `yaml:"allow_add,omitempty"`
		AllowRemove []string `yaml:"allow_remove,omitempty"`
		DenyAdd     []string `yaml:"deny_add,omitempty"`
		DenyRemove  []string `yaml:"deny_remove,omitempty"`
	} `yaml:"write_scope,omitempty"`
}

// Project is the schema of a per-project .bender/config.yaml.
type Project struct {
	Agents map[string]AgentOverride `yaml:"agents,omitempty"`
}

// Load reads and parses path. A missing file yields an empty Project (not an error) so callers can
// treat absence the same as an empty config.
func Load(path string) (*Project, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &Project{}, nil
		}
		return nil, fmt.Errorf("config: read %s: %w", path, err)
	}
	var p Project
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("config: parse %s: %w", path, err)
	}
	return &p, nil
}
