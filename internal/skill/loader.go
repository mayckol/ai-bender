package skill

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
	"text/template"

	"github.com/mayckol/ai-bender/internal/types"
)

// Catalog is the in-memory registry of skills resolved from the embedded defaults plus user overrides.
// It is shared between `bender doctor` and any future commands that need resolution; that ensures the
// diagnostic and the actual run see the same effective bindings.
type Catalog struct {
	skills map[string]*Skill
	order  []string
}

// LoadCatalog walks defaultsFS first, then userFS. Same-name files in userFS override embedded defaults
// entirely (no merge). Returns the Catalog and a slice of non-fatal warnings.
func LoadCatalog(defaultsFS, userFS fs.FS) (*Catalog, []string, error) {
	c := &Catalog{skills: map[string]*Skill{}}
	var warnings []string

	if defaultsFS != nil {
		defaultWarns, err := c.walk(defaultsFS, "claude/skills", types.OriginEmbedded)
		if err != nil {
			return nil, nil, fmt.Errorf("load embedded skills: %w", err)
		}
		warnings = append(warnings, defaultWarns...)
	}
	if userFS != nil {
		userWarns, err := c.walk(userFS, "skills", types.OriginUser)
		if err != nil {
			return nil, nil, fmt.Errorf("load user skills: %w", err)
		}
		warnings = append(warnings, userWarns...)
	}
	c.reindex()
	return c, warnings, nil
}

func (c *Catalog) walk(root fs.FS, base string, origin types.Origin) ([]string, error) {
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
		base := path.Base(p)
		isTmpl := base == "SKILL.md.tmpl"
		if base != "SKILL.md" && !isTmpl {
			return nil
		}
		// A SKILL.md and a SKILL.md.tmpl cannot coexist in the same
		// directory; prefer the .tmpl (it is the source of truth; the
		// verbatim variant would be a stale rendered artefact). The
		// catalog's map uses skill name (from frontmatter), so the last
		// write wins here — defensive tiebreak only.
		if isTmpl {
			sibling := strings.TrimSuffix(p, ".tmpl")
			if _, err := fs.Stat(root, sibling); err == nil {
				return nil // use the verbatim SKILL.md adjacent to this .tmpl
			}
		}
		data, readErr := fs.ReadFile(root, p)
		if readErr != nil {
			return fmt.Errorf("read %s: %w", p, readErr)
		}
		if isTmpl {
			rendered, renderErr := renderSkillForCatalog(data)
			if renderErr != nil {
				warnings = append(warnings, fmt.Sprintf("%s: template render for catalog: %v", p, renderErr))
				return nil
			}
			data = rendered
		}
		s, parseErr := ParseFrontmatter(data)
		if parseErr != nil {
			warnings = append(warnings, fmt.Sprintf("%s: %v", p, parseErr))
			return nil
		}
		s.Path = p
		s.Origin = origin
		// Folder-name prefix sanity warning when context disagrees with `bg-` / `fg-` folder prefix.
		dir := path.Base(path.Dir(p))
		if strings.HasPrefix(dir, "bg-") && s.Context != types.ContextBG {
			warnings = append(warnings, fmt.Sprintf("%s: folder prefix %q disagrees with declared context %q (context wins)", p, dir, s.Context))
		}
		if strings.HasPrefix(dir, "fg-") && s.Context != types.ContextFG {
			warnings = append(warnings, fmt.Sprintf("%s: folder prefix %q disagrees with declared context %q (context wins)", p, dir, s.Context))
		}
		c.skills[s.Name] = s
		return nil
	})
	if err != nil {
		return warnings, err
	}
	return warnings, nil
}

func (c *Catalog) reindex() {
	c.order = c.order[:0]
	for name := range c.skills {
		c.order = append(c.order, name)
	}
	sort.Strings(c.order)
}

// Get returns the skill registered under name, or nil when absent.
func (c *Catalog) Get(name string) *Skill {
	return c.skills[name]
}

// Names returns all skill names in deterministic (alphabetic) order.
func (c *Catalog) Names() []string {
	out := make([]string, len(c.order))
	copy(out, c.order)
	return out
}

// All returns every skill in deterministic order.
func (c *Catalog) All() []*Skill {
	out := make([]*Skill, 0, len(c.order))
	for _, n := range c.order {
		out = append(out, c.skills[n])
	}
	return out
}

// Len returns the number of skills currently in the catalog.
func (c *Catalog) Len() int { return len(c.skills) }

// renderSkillForCatalog renders a SKILL.md.tmpl with an "everything
// selected" view so ParseFrontmatter can read the embedded skill. We
// deliberately do not wire the live Component Catalog here — that would
// create an import cycle and, for diagnostic purposes, a fully-populated
// render is strictly more informative than a pruned one.
func renderSkillForCatalog(src []byte) ([]byte, error) {
	tmpl, err := template.New("skill").Funcs(template.FuncMap{
		"selected":    func(string) bool { return true },
		"description": func(string) string { return "" },
	}).Parse(string(src))
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, nil); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
