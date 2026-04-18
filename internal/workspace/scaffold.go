// Package workspace materialises the embedded defaults tree into a project's
// `.claude/` (Claude Code–native artefacts: agents, skills) and `.bender/`
// (bender-owned config: pipeline.yaml, groups.yaml) plus, in registry.go,
// manages the global multi-project registry at `~/.bender/workspace.yaml`.
package workspace

import (
	"bytes"
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

	// Remove on-disk files that belong to newly-deselected components.
	// Safe by default: a removal candidate whose on-disk bytes differ
	// from what init would have rendered is treated as user-edited and
	// skipped unless --force.
	if opts.Catalog != nil {
		if err := removeDeselectedFiles(root, opts, ownedBy, res); err != nil {
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

// PipelineConflictError signals FR-015: the on-disk pipeline.yaml has been
// user-edited AND the newly-rendered content differs, so init refuses to
// overwrite silently. The caller writes `pipeline.yaml.proposed` alongside
// the user's file and exits non-zero.
type PipelineConflictError struct {
	UserPath     string
	ProposedPath string
	DroppedNodes []string
}

func (e *PipelineConflictError) Error() string {
	msg := fmt.Sprintf("pipeline.yaml has been user-edited; new content written to %s (re-run with --force to overwrite)", e.ProposedPath)
	if len(e.DroppedNodes) > 0 {
		msg += fmt.Sprintf("\n  would drop nodes: %v", e.DroppedNodes)
	}
	return msg
}

// writePipeline renders pipeline.yaml against the current selection and
// writes it, recording the fingerprint so subsequent runs can detect
// drift. On drift without --force, writes pipeline.yaml.proposed and
// returns a PipelineConflictError.
func writePipeline(root fs.FS, opts ScaffoldOptions, res *ScaffoldResult) error {
	src, err := fs.ReadFile(root, pipelineEmbedPath)
	if err != nil {
		return fmt.Errorf("read embedded pipeline.yaml: %w", err)
	}
	rendered, dropped, err := render.Pipeline(src, opts.Catalog, opts.Selection)
	if err != nil {
		return err
	}
	dest := filepath.Join(opts.ProjectRoot, ".bender", "pipeline.yaml")

	// Fresh write: no existing file → write + fingerprint.
	if _, statErr := os.Stat(dest); errors.Is(statErr, os.ErrNotExist) {
		if err := writeFile(dest, rendered, false, res); err != nil {
			return err
		}
		return render.WriteFingerprint(opts.ProjectRoot, render.Fingerprint(rendered))
	} else if statErr != nil {
		return statErr
	}

	// File exists. Check for drift.
	drifted, err := render.IsDrifted(opts.ProjectRoot, dest)
	if err != nil {
		return err
	}
	if drifted && !opts.Force {
		proposed := dest + ".proposed"
		if err := os.WriteFile(proposed, rendered, 0o644); err != nil {
			return err
		}
		return &PipelineConflictError{
			UserPath:     dest,
			ProposedPath: proposed,
			DroppedNodes: dropped,
		}
	}

	// Either not drifted, or --force: overwrite and refresh fingerprint.
	// Also, when regenerated content is byte-identical to what's on disk,
	// skip the write and add the file to Preserved for summary fidelity.
	onDisk, err := os.ReadFile(dest)
	if err != nil {
		return err
	}
	if bytes.Equal(onDisk, rendered) {
		res.Preserved = append(res.Preserved, dest)
		// Idempotent path: ensure the sidecar is in sync.
		return render.WriteFingerprint(opts.ProjectRoot, render.Fingerprint(rendered))
	}
	if err := os.WriteFile(dest, rendered, 0o644); err != nil {
		return err
	}
	res.Overwritten = append(res.Overwritten, dest)
	// If --force was used to overwrite a user-edited file, drop any stale
	// .proposed sidecar so the workspace is clean.
	_ = os.Remove(dest + ".proposed")
	return render.WriteFingerprint(opts.ProjectRoot, render.Fingerprint(rendered))
}

// removeDeselectedFiles walks the on-disk `.claude/` tree and deletes any
// file attributable to a deselected component. A file whose bytes differ
// from the embedded-defaults source (i.e. the user edited it) is NOT
// removed unless --force, so user edits are never discarded silently.
func removeDeselectedFiles(root fs.FS, opts ScaffoldOptions, owned map[string]string, res *ScaffoldResult) error {
	if owned == nil {
		return nil
	}
	for embedPath := range owned {
		dest := destinationFor(opts.ProjectRoot, embedPath)
		if dest == "" {
			continue
		}
		info, err := os.Stat(dest)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return err
		}
		if info.IsDir() {
			if err := removeTreeSafely(root, opts, embedPath, dest, res); err != nil {
				return err
			}
			continue
		}
		if err := removeFileSafely(root, opts, embedPath, dest, res); err != nil {
			return err
		}
	}
	return nil
}

func removeTreeSafely(root fs.FS, opts ScaffoldOptions, embedPath, destDir string, res *ScaffoldResult) error {
	// Walk the embedded side to enumerate files this directory should
	// contain; then remove their on-disk counterparts, guarding against
	// user edits.
	err := fs.WalkDir(root, embedPath, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		dest := destinationFor(opts.ProjectRoot, p)
		if dest == "" {
			return nil
		}
		return removeFileSafely(root, opts, p, dest, res)
	})
	if err != nil {
		return err
	}
	// Clean up the destination directory if empty.
	_ = os.Remove(destDir)
	return nil
}

func removeFileSafely(root fs.FS, opts ScaffoldOptions, embedPath, dest string, res *ScaffoldResult) error {
	onDisk, err := os.ReadFile(dest)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	src, err := fs.ReadFile(root, embedPath)
	if err != nil {
		return err
	}
	if !bytes.Equal(onDisk, src) && !opts.Force {
		// User-edited. Skip with no effect; the file stays on disk.
		return nil
	}
	if err := os.Remove(dest); err != nil {
		return err
	}
	res.Removed = append(res.Removed, dest)
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
