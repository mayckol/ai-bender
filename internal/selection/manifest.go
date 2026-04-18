package selection

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/mayckol/ai-bender/internal/catalog"
)

// ManifestFileName is the on-disk location of the Selection Manifest,
// relative to the workspace root.
const ManifestFileName = ".bender/selection.yaml"

// Manifest is the on-disk shape of `.bender/selection.yaml` — a stable
// contract that `bender init`, diagnostic commands, and any future UI read
// as the authoritative current selection.
type Manifest struct {
	SchemaVersion int                      `yaml:"schema_version"`
	Components    map[string]ManifestEntry `yaml:"components"`
}

// ManifestEntry holds one component's persisted state. Extra fields may be
// added in a future schema_version bump without breaking older readers.
type ManifestEntry struct {
	Selected bool `yaml:"selected"`
}

// Load reads the manifest from workspace root. Absence is not an error — a
// nil manifest is returned so the caller can distinguish "fresh workspace"
// (fall back to catalog defaults) from "user previously chose to include
// everything".
func Load(workspaceRoot string) (*Manifest, error) {
	path := filepath.Join(workspaceRoot, ManifestFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("selection: read %s: %w", path, err)
	}
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("selection: parse %s: %w", path, err)
	}
	if m.SchemaVersion != 1 {
		return nil, fmt.Errorf("selection: unsupported schema_version %d (want 1)", m.SchemaVersion)
	}
	return &m, nil
}

// Validate checks the manifest against a catalog: every key must be a known
// component, and no mandatory component may be marked deselected.
func (m *Manifest) Validate(cat *catalog.Catalog) error {
	for id, entry := range m.Components {
		comp, ok := cat.Components[id]
		if !ok {
			return fmt.Errorf("selection: unknown component %q in manifest", id)
		}
		if !comp.Optional && !entry.Selected {
			return fmt.Errorf("selection: mandatory component %q cannot be marked selected: false", id)
		}
	}
	return nil
}

// Save writes the manifest to `.bender/selection.yaml` under workspaceRoot.
// Creates the `.bender/` directory if missing. Atomic-ish: writes to a
// sibling tempfile + rename.
func Save(workspaceRoot string, sel map[string]bool) error {
	m := Manifest{
		SchemaVersion: 1,
		Components:    make(map[string]ManifestEntry, len(sel)),
	}
	for id, v := range sel {
		m.Components[id] = ManifestEntry{Selected: v}
	}
	data, err := yaml.Marshal(m)
	if err != nil {
		return fmt.Errorf("selection: marshal: %w", err)
	}
	benderDir := filepath.Join(workspaceRoot, ".bender")
	if err := os.MkdirAll(benderDir, 0o755); err != nil {
		return fmt.Errorf("selection: mkdir %s: %w", benderDir, err)
	}
	path := filepath.Join(workspaceRoot, ManifestFileName)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("selection: write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("selection: rename %s -> %s: %w", tmp, path, err)
	}
	return nil
}
