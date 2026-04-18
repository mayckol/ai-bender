// Package workspace materialises the embedded defaults tree into a project's
// `.claude/` (Claude Code–native artefacts: agents, skills) and `.bender/`
// (bender-owned config: pipeline.yaml, groups.yaml) plus, in registry.go,
// manages the global multi-project registry at `~/.bender/workspace.yaml`.
package workspace

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mayckol/ai-bender/internal/catalog"
	embedded "github.com/mayckol/ai-bender/internal/embed"
	"github.com/mayckol/ai-bender/internal/render"
)

// ScaffoldOptions controls a single materialisation pass.
type ScaffoldOptions struct {
	ProjectRoot string
	Force       bool
	// Catalog + Selection are optional; when both are nil the scaffolder
	// falls back to today's "install everything verbatim" behaviour. This
	// preserves backward compatibility for callers that haven't wired the
	// feature yet.
	Catalog   *catalog.Catalog
	Selection map[string]bool
}

// ScaffoldResult summarises what changed during a Scaffold call.
type ScaffoldResult struct {
	Created     []string
	Preserved   []string
	Overwritten []string
	Removed     []string
	Excluded    []string // component ids whose files were not written
}

// pipelineEmbedPath is the embedded path to the default pipeline.yaml.
const pipelineEmbedPath = "bender/pipeline.yaml"

// embeddedTrees lists every top-level prefix in the embedded `defaults/` FS
// that we mirror onto disk. Adding a new bender-owned tree = one line here.
var embeddedTrees = []string{"claude", "bender"}

// Scaffold walks every top-level tree in the embedded defaults and
// materialises files under the project root. When Catalog + Selection are
// provided, files owned by deselected components are skipped, `.tmpl`
// skills are rendered, and pipeline.yaml is re-rendered with pruned nodes.
func Scaffold(opts ScaffoldOptions) (*ScaffoldResult, error) {
	if opts.ProjectRoot == "" {
		return nil, errors.New("workspace: ProjectRoot is required")
	}
	if err := os.MkdirAll(opts.ProjectRoot, 0o755); err != nil {
		return nil, err
	}
	res := &ScaffoldResult{}
	root := embedded.FS()

	ownedBy, excluded := buildOwnership(opts.Catalog, opts.Selection)
	ctx := render.Ctx{}
	if opts.Catalog != nil {
		ctx = render.BuildCtx(opts.Catalog, opts.Selection)
	}
	res.Excluded = excluded

	for _, tree := range embeddedTrees {
		if _, err := fs.Stat(root, tree); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return nil, fmt.Errorf("stat embedded %s: %w", tree, err)
		}
		err := fs.WalkDir(root, tree, func(p string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				return nil
			}
			// Skip the catalog itself — it's consumed at runtime, not
			// shipped into the user's workspace today.
			if p == "bender/catalog.yaml" {
				return nil
			}
			// Skip pipeline.yaml here; rendered separately below so we can
			// prune nodes before writing.
			if p == pipelineEmbedPath && opts.Catalog != nil {
				return nil
			}
			if isOwnedByDeselected(p, ownedBy) {
				return nil
			}
			destPath := destinationFor(opts.ProjectRoot, p)
			if destPath == "" {
				return nil
			}
			data, err := fs.ReadFile(root, p)
			if err != nil {
				return fmt.Errorf("read embedded %s: %w", p, err)
			}
			// Render .tmpl skills against the selection.
			if strings.HasSuffix(destPath, render.TemplateSuffix) && opts.Catalog != nil {
				rendered, err := render.Skill(data, ctx, opts.Catalog)
				if err != nil {
					return fmt.Errorf("render %s: %w", p, err)
				}
				data = rendered
				destPath = strings.TrimSuffix(destPath, render.TemplateSuffix)
			}
			return writeFile(destPath, data, opts.Force, res)
		})
		if err != nil {
			return nil, err
		}
	}

	// Render + write pipeline.yaml (with pruning) when catalog is active.
	if opts.Catalog != nil {
		if err := writePipeline(root, opts, res); err != nil {
			return nil, err
		}
	}

	sort.Strings(res.Created)
	sort.Strings(res.Preserved)
	sort.Strings(res.Overwritten)
	sort.Strings(res.Excluded)
	return res, nil
}

// writeFile applies the create/preserve/overwrite contract to a single file.
func writeFile(dest string, data []byte, force bool, res *ScaffoldResult) error {
	exists := false
	if _, err := os.Stat(dest); err == nil {
		exists = true
	}
	if exists && !force {
		res.Preserved = append(res.Preserved, dest)
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(dest, data, 0o644); err != nil {
		return err
	}
	if exists {
		res.Overwritten = append(res.Overwritten, dest)
	} else {
		res.Created = append(res.Created, dest)
	}
	return nil
}

// writePipeline renders pipeline.yaml against the current selection and
// writes it, recording the fingerprint so subsequent runs can detect
// drift.
func writePipeline(root fs.FS, opts ScaffoldOptions, res *ScaffoldResult) error {
	src, err := fs.ReadFile(root, pipelineEmbedPath)
	if err != nil {
		return fmt.Errorf("read embedded pipeline.yaml: %w", err)
	}
	rendered, _, err := render.Pipeline(src, opts.Catalog, opts.Selection)
	if err != nil {
		return err
	}
	dest := filepath.Join(opts.ProjectRoot, ".bender", "pipeline.yaml")
	if err := writeFile(dest, rendered, opts.Force, res); err != nil {
		return err
	}
	// Only refresh the fingerprint if we actually wrote the file (create
	// or overwrite). Preserved user edits keep the old sidecar.
	wrote := false
	for _, p := range res.Created {
		if p == dest {
			wrote = true
			break
		}
	}
	if !wrote {
		for _, p := range res.Overwritten {
			if p == dest {
				wrote = true
				break
			}
		}
	}
	if wrote {
		if err := render.WriteFingerprint(opts.ProjectRoot, render.Fingerprint(rendered)); err != nil {
			return err
		}
	}
	return nil
}

// buildOwnership builds a path-prefix → component-id map so the walker can
// skip files owned by deselected components. Returns the map and the sorted
// list of excluded component ids (for the init summary).
func buildOwnership(cat *catalog.Catalog, sel map[string]bool) (map[string]string, []string) {
	if cat == nil {
		return nil, nil
	}
	owned := map[string]string{}
	var excluded []string
	for id, comp := range cat.Components {
		// A catalog entry only gates scaffold when its selection is false
		// (mandatory entries are always true; optional ones may be false).
		if sel[id] {
			continue
		}
		excluded = append(excluded, id)
		if comp.Paths.Agent != "" {
			owned[comp.Paths.Agent] = id
		}
		for _, sk := range comp.Paths.Skills {
			owned[sk] = id
		}
	}
	sort.Strings(excluded)
	return owned, excluded
}

// isOwnedByDeselected returns true when `p` is inside a skill directory or
// matches an agent file that belongs to a deselected component.
func isOwnedByDeselected(p string, owned map[string]string) bool {
	if owned == nil {
		return false
	}
	if _, ok := owned[p]; ok {
		return true
	}
	for prefix := range owned {
		if strings.HasPrefix(p, prefix+"/") {
			return true
		}
	}
	return false
}

// destinationFor maps an embedded path to its on-disk destination.
// `claude/<rel>` → `<root>/.claude/<rel>`; `bender/<rel>` → `<root>/.bender/<rel>`.
// Unknown prefixes return "" so the walker skips them.
func destinationFor(projectRoot, embeddedPath string) string {
	for _, tree := range embeddedTrees {
		if rel, ok := strings.CutPrefix(embeddedPath, tree+"/"); ok {
			return filepath.Join(projectRoot, "."+tree, filepath.FromSlash(rel))
		}
	}
	return ""
}
