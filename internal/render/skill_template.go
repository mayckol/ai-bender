package render

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
	"text/template/parse"

	"github.com/mayckol/ai-bender/internal/catalog"
)

// TemplateSuffix identifies a file that must be rendered rather than copied
// verbatim. Shipped files under `defaults/claude/skills/**` are either
// `SKILL.md` (verbatim) or `SKILL.md.tmpl` (templated).
const TemplateSuffix = ".tmpl"

// Ctx is the root data passed to every skill template.
type Ctx struct {
	Components map[string]ComponentCtx
}

// ComponentCtx is the per-component view exposed to templates.
type ComponentCtx struct {
	ID          string
	Selected    bool
	Description string
}

// BuildCtx projects the resolved selection map into a template context.
func BuildCtx(cat *catalog.Catalog, sel map[string]bool) Ctx {
	out := Ctx{Components: make(map[string]ComponentCtx, len(cat.Components))}
	for id, comp := range cat.Components {
		out.Components[id] = ComponentCtx{
			ID:          id,
			Selected:    sel[id],
			Description: comp.Description,
		}
	}
	return out
}

// Skill parses the given template source, validates that every referenced
// component id exists in the catalog (AST pre-pass; fails before any file
// is written), and renders the template against `ctx`. Returns the rendered
// bytes and a slice of WARNINGS: empty on a clean render; non-empty when
// the template parsed but referenced identifiers that should be surfaced.
func Skill(src []byte, ctx Ctx, cat *catalog.Catalog) ([]byte, error) {
	tmpl, err := template.New("skill").Funcs(funcMap(ctx)).Parse(string(src))
	if err != nil {
		return nil, fmt.Errorf("render: parse: %w", err)
	}
	if err := validateIDs(tmpl, cat); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, ctx); err != nil {
		return nil, fmt.Errorf("render: execute: %w", err)
	}
	return buf.Bytes(), nil
}

// funcMap builds the small template vocabulary: `selected "id"` and
// `description "id"`. We keep the surface tiny so template authors cannot
// accidentally reach outside the catalog contract.
func funcMap(ctx Ctx) template.FuncMap {
	return template.FuncMap{
		"selected": func(id string) bool {
			c, ok := ctx.Components[id]
			return ok && c.Selected
		},
		"description": func(id string) string {
			return ctx.Components[id].Description
		},
	}
}

// validateIDs walks the parsed template AST and inspects every call to the
// `selected` or `description` helper; the sole argument must be a string
// literal naming a known component. Anything else fails here, before
// Execute ever runs.
func validateIDs(t *template.Template, cat *catalog.Catalog) error {
	var issues []string
	for _, tree := range t.Templates() {
		if tree == nil || tree.Tree == nil {
			continue
		}
		walk(tree.Tree.Root, cat.Components, &issues)
	}
	if len(issues) > 0 {
		return fmt.Errorf("render: template references unknown component id(s):\n  - %s", strings.Join(issues, "\n  - "))
	}
	return nil
}

func walk(node parse.Node, known map[string]catalog.Component, issues *[]string) {
	if node == nil {
		return
	}
	switch n := node.(type) {
	case *parse.ListNode:
		if n == nil {
			return
		}
		for _, sub := range n.Nodes {
			walk(sub, known, issues)
		}
	case *parse.ActionNode:
		walkPipe(n.Pipe, known, issues)
	case *parse.IfNode:
		walkPipe(n.Pipe, known, issues)
		walk(n.List, known, issues)
		if n.ElseList != nil {
			walk(n.ElseList, known, issues)
		}
	case *parse.RangeNode:
		walkPipe(n.Pipe, known, issues)
		walk(n.List, known, issues)
		if n.ElseList != nil {
			walk(n.ElseList, known, issues)
		}
	case *parse.WithNode:
		walkPipe(n.Pipe, known, issues)
		walk(n.List, known, issues)
		if n.ElseList != nil {
			walk(n.ElseList, known, issues)
		}
	}
}

func walkPipe(p *parse.PipeNode, known map[string]catalog.Component, issues *[]string) {
	if p == nil {
		return
	}
	for _, cmd := range p.Cmds {
		if cmd == nil || len(cmd.Args) == 0 {
			continue
		}
		ident, ok := cmd.Args[0].(*parse.IdentifierNode)
		if !ok {
			continue
		}
		if ident.Ident != "selected" && ident.Ident != "description" {
			continue
		}
		if len(cmd.Args) < 2 {
			*issues = append(*issues, fmt.Sprintf("%s helper without argument", ident.Ident))
			continue
		}
		lit, ok := cmd.Args[1].(*parse.StringNode)
		if !ok {
			*issues = append(*issues, fmt.Sprintf("%s helper needs a string literal id", ident.Ident))
			continue
		}
		if _, ok := known[lit.Text]; !ok {
			*issues = append(*issues, fmt.Sprintf("unknown component id %q", lit.Text))
		}
	}
}
