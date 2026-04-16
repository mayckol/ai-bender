package workspace

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/mayckol/ai-bender/internal/artifact"
)

// Registry is the global per-user list of registered projects.
// Lives at ~/.bender/workspace.yaml by default, or $XDG_CONFIG_HOME/bender/workspace.yaml when set.
type Registry struct {
	DefaultProject string                  `yaml:"default_project,omitempty"`
	Projects       map[string]ProjectEntry `yaml:"projects,omitempty"`
}

// ProjectEntry is one registered project.
type ProjectEntry struct {
	Path         string `yaml:"path"`
	RegisteredAt string `yaml:"registered_at"`
}

// Status represents the runtime status of a registered project.
type Status string

const (
	StatusCurrent   Status = "current"
	StatusAvailable Status = "available"
	StatusMissing   Status = "missing"
)

// ProjectListing pairs a registered project with its computed runtime status.
type ProjectListing struct {
	Name   string
	Entry  ProjectEntry
	Status Status
}

var (
	nameRe     = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)
	kebabSafe  = regexp.MustCompile(`[^a-z0-9-]+`)
	dashRunsRe = regexp.MustCompile(`-+`)

	// ErrDuplicateName is returned by Register when the chosen name is already registered.
	ErrDuplicateName = errors.New("workspace: project name already registered")
	// ErrInvalidName is returned when a project name fails the kebab-case regex.
	ErrInvalidName = errors.New("workspace: project name must match [a-z0-9][a-z0-9-]{0,62}")
	// ErrPathNotDir is returned when the supplied path does not exist or is not a directory.
	ErrPathNotDir = errors.New("workspace: path is not a directory")
)

// RegistryPath returns the absolute path of the registry file, honoring $XDG_CONFIG_HOME.
func RegistryPath() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "bender", "workspace.yaml"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("workspace: resolve home dir: %w", err)
	}
	return filepath.Join(home, ".bender", "workspace.yaml"), nil
}

// LoadRegistry reads and parses the registry file. A missing file yields an empty Registry,
// not an error, so callers can treat absence the same as zero registered projects.
func LoadRegistry() (*Registry, string, error) {
	path, err := RegistryPath()
	if err != nil {
		return nil, "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &Registry{Projects: map[string]ProjectEntry{}}, path, nil
		}
		return nil, path, fmt.Errorf("workspace: read %s: %w", path, err)
	}
	var r Registry
	if err := yaml.Unmarshal(data, &r); err != nil {
		return nil, path, fmt.Errorf("workspace: parse %s: %w", path, err)
	}
	if r.Projects == nil {
		r.Projects = map[string]ProjectEntry{}
	}
	return &r, path, nil
}

// SaveRegistry writes the registry to disk atomically (temp + rename).
func SaveRegistry(r *Registry) error {
	path, err := RegistryPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(r)
	if err != nil {
		return fmt.Errorf("workspace: marshal: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".workspace.*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// Register adds a project to the registry. Returns the registered ProjectEntry.
// A blank `name` is auto-derived from the basename of `projectPath`, kebab-case-normalised.
func Register(name, projectPath string) (string, *ProjectEntry, error) {
	abs, err := filepath.Abs(projectPath)
	if err != nil {
		return "", nil, fmt.Errorf("workspace: resolve path: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil || !info.IsDir() {
		return "", nil, fmt.Errorf("%w: %s", ErrPathNotDir, abs)
	}
	if name == "" {
		name = autoName(filepath.Base(abs))
	}
	if !nameRe.MatchString(name) {
		return "", nil, fmt.Errorf("%w: %q", ErrInvalidName, name)
	}
	r, _, err := LoadRegistry()
	if err != nil {
		return "", nil, err
	}
	if _, exists := r.Projects[name]; exists {
		return "", nil, fmt.Errorf("%w: %q", ErrDuplicateName, name)
	}
	entry := ProjectEntry{Path: abs, RegisteredAt: artifact.Now()}
	r.Projects[name] = entry
	if r.DefaultProject == "" {
		r.DefaultProject = name
	}
	if err := SaveRegistry(r); err != nil {
		return "", nil, err
	}
	return name, &entry, nil
}

// Unregister removes a project. Returns ok=false when the name was not present.
func Unregister(name string) (bool, error) {
	r, _, err := LoadRegistry()
	if err != nil {
		return false, err
	}
	if _, ok := r.Projects[name]; !ok {
		return false, nil
	}
	delete(r.Projects, name)
	if r.DefaultProject == name {
		r.DefaultProject = ""
	}
	if err := SaveRegistry(r); err != nil {
		return false, err
	}
	return true, nil
}

// List returns every registered project paired with its runtime status (current vs available vs missing).
// Listings are sorted by name for deterministic output.
func List(cwd string) ([]ProjectListing, error) {
	r, _, err := LoadRegistry()
	if err != nil {
		return nil, err
	}
	cwdAbs, _ := filepath.Abs(cwd)
	out := make([]ProjectListing, 0, len(r.Projects))
	for name, entry := range r.Projects {
		out = append(out, ProjectListing{
			Name:   name,
			Entry:  entry,
			Status: classify(entry, cwdAbs),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// Resolve returns the project to operate on. If `flag` is non-empty, it is honored. Otherwise,
// the project containing cwd wins; failing that, the registry's default_project; failing that, the
// first-registered. Returns ("", "") when nothing is registered.
func Resolve(flag, cwd string) (string, string, error) {
	r, _, err := LoadRegistry()
	if err != nil {
		return "", "", err
	}
	if flag != "" {
		entry, ok := r.Projects[flag]
		if !ok {
			return "", "", fmt.Errorf("workspace: --project=%q not registered", flag)
		}
		return flag, entry.Path, nil
	}
	cwdAbs, _ := filepath.Abs(cwd)
	for name, entry := range r.Projects {
		if pathContains(entry.Path, cwdAbs) {
			return name, entry.Path, nil
		}
	}
	if r.DefaultProject != "" {
		if entry, ok := r.Projects[r.DefaultProject]; ok {
			return r.DefaultProject, entry.Path, nil
		}
	}
	return "", "", nil
}

func classify(entry ProjectEntry, cwd string) Status {
	if _, err := os.Stat(entry.Path); err != nil {
		return StatusMissing
	}
	if pathContains(entry.Path, cwd) {
		return StatusCurrent
	}
	return StatusAvailable
}

// pathContains reports whether `cwd` is `root` or a subdirectory of `root`. Both paths are
// passed through filepath.EvalSymlinks first so platform-specific symlink quirks (notably
// macOS's /var → /private/var) don't make a containment check spuriously fail.
func pathContains(root, cwd string) bool {
	if root == "" || cwd == "" {
		return false
	}
	root = resolveSymlinks(root)
	cwd = resolveSymlinks(cwd)
	rel, err := filepath.Rel(root, cwd)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel))
}

func resolveSymlinks(p string) string {
	if resolved, err := filepath.EvalSymlinks(p); err == nil {
		return resolved
	}
	return p
}

func autoName(basename string) string {
	low := strings.ToLower(basename)
	low = kebabSafe.ReplaceAllString(low, "-")
	low = dashRunsRe.ReplaceAllString(low, "-")
	low = strings.Trim(low, "-")
	if low == "" {
		return "project"
	}
	if !nameRe.MatchString(low) {
		// Trim to 63 chars and ensure leading char is alphanumeric.
		if len(low) > 63 {
			low = low[:63]
			low = strings.TrimRight(low, "-")
		}
		// Try once more; if still bad, fall back to a timestamped name.
		if !nameRe.MatchString(low) {
			return "project-" + time.Now().UTC().Format("20060102")
		}
	}
	return low
}
