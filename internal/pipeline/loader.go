package pipeline

import (
	"errors"
	"fmt"
	"io/fs"

	"gopkg.in/yaml.v3"
)

// ErrNotFound is returned by LoadFromFS when pipeline.yaml is absent.
// Callers decide whether that's fatal or "fall back to the embedded default".
var ErrNotFound = errors.New("pipeline: pipeline.yaml not found")

// LoadFromFS reads and parses pipeline.yaml from the given filesystem.
// base is the path within root (e.g. "claude/pipeline.yaml" for the embed FS,
// or "pipeline.yaml" for a user's .claude FS).
func LoadFromFS(root fs.FS, base string) (*Pipeline, error) {
	if root == nil {
		return nil, ErrNotFound
	}
	data, err := fs.ReadFile(root, base)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("pipeline: read %s: %w", base, err)
	}
	p, err := Parse(data)
	if err != nil {
		return nil, err
	}
	p.SourcePath = base
	return p, nil
}

// Parse parses the bytes of a pipeline.yaml document. Schema-version and
// shape checks happen here; cross-reference validation is Validator's job.
func Parse(data []byte) (*Pipeline, error) {
	var p Pipeline
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("pipeline: parse: %w", err)
	}
	if p.SchemaVersion == 0 {
		return nil, fmt.Errorf("pipeline: schema_version is required")
	}
	if p.SchemaVersion != SchemaVersion {
		return nil, fmt.Errorf("pipeline: unsupported schema_version=%d (want %d)", p.SchemaVersion, SchemaVersion)
	}
	if p.Meta.ID == "" {
		return nil, fmt.Errorf("pipeline: pipeline.id is required")
	}
	if p.Meta.Description == "" {
		return nil, fmt.Errorf("pipeline: pipeline.description is required")
	}
	if len(p.Nodes) == 0 {
		return nil, fmt.Errorf("pipeline: nodes must declare at least one entry")
	}
	return &p, nil
}
