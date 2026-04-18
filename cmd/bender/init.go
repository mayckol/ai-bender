package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mayckol/ai-bender/internal/catalog"
	"github.com/mayckol/ai-bender/internal/constitution"
	"github.com/mayckol/ai-bender/internal/discovery"
	"github.com/mayckol/ai-bender/internal/selection"
	"github.com/mayckol/ai-bender/internal/workspace"
)

type initOpts struct {
	here          bool
	force         bool
	with          []string
	without       []string
	noInteractive bool
}

// Exit code conventions (documented in contracts/cli.md):
//   1 = generic failure
//   2 = user error (unknown id, mandatory deselection, contradictory flags)
//   3 = dependency break; caller can retry with --force to cascade
//   4 = pipeline.yaml conflict with user edits
const (
	exitGeneric  = 1
	exitUserErr  = 2
	exitDepBreak = 3
	exitConflict = 4
)

// typedError carries the exit code the CLI should surface for a given failure.
// Non-typed errors fall back to exitGeneric.
type typedError struct {
	code int
	err  error
}

func (e *typedError) Error() string { return e.err.Error() }
func (e *typedError) Unwrap() error { return e.err }

func newInitCmd(g *globalFlags) *cobra.Command {
	o := &initOpts{}
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Scaffold .claude/ + .bender/ from the Component Catalog (idempotent)",
		Long: `init materialises the embedded .claude/ + .bender/ defaults into the current project,
classifying each agent and skill as mandatory or optional per the Component Catalog. Optional
components may be deselected via --without or added back via --with; the resulting selection
is persisted to .bender/selection.yaml. Existing user-modified files are preserved unless
--force is passed.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			err := runInit(cmd.OutOrStdout(), cmd.ErrOrStderr(), g, o)
			if err == nil {
				return nil
			}
			// Translate typed errors into cobra-silencing + os.Exit so the
			// exit code contract holds regardless of cobra's own error wrap.
			var te *typedError
			if errors.As(err, &te) {
				fmt.Fprintln(cmd.ErrOrStderr(), te.Error())
				cmd.SilenceUsage = true
				cmd.SilenceErrors = true
				os.Exit(te.code)
			}
			return err
		},
	}
	cmd.Flags().BoolVar(&o.here, "here", false, "explicitly initialise in the current directory (default behaviour)")
	cmd.Flags().BoolVar(&o.force, "force", false, "overwrite user-modified files in .claude/ and .bender/ with regenerated content")
	cmd.Flags().StringArrayVar(&o.with, "with", nil, "include an optional component (repeatable)")
	cmd.Flags().StringArrayVar(&o.without, "without", nil, "exclude an optional component (repeatable)")
	cmd.Flags().BoolVar(&o.noInteractive, "no-interactive", false, "force non-interactive mode even in a TTY")
	return cmd
}

func runInit(out, errw io.Writer, _ *globalFlags, o *initOpts) error {
	root, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("init: get cwd: %w", err)
	}
	_ = o.here

	cat, err := catalog.Load()
	if err != nil {
		return fmt.Errorf("init: catalog: %w", err)
	}

	manifest, err := selection.Load(root)
	if err != nil {
		return fmt.Errorf("init: selection: %w", err)
	}
	if manifest != nil {
		if err := manifest.Validate(cat); err != nil {
			return &typedError{code: exitUserErr, err: fmt.Errorf("init: %w", err)}
		}
	}

	sel, err := selection.Resolve(cat, manifest, selection.Flags{With: o.with, Without: o.without})
	if err != nil {
		return &typedError{code: exitUserErr, err: fmt.Errorf("init: %w", err)}
	}

	breaks := catalog.DetectBreaks(cat, sel)
	if len(breaks) > 0 && !o.force {
		msg := formatBreaks(breaks)
		return &typedError{code: exitDepBreak, err: fmt.Errorf("init: dependency break:\n%s\n  re-run with --force to cascade the deselection", msg)}
	}
	if len(breaks) > 0 && o.force {
		for _, b := range breaks {
			sel = catalog.CascadeDeselect(cat, sel, []string{b.Deselected})
		}
		fmt.Fprintf(errw, "bender: --force cascaded deselection through dependents: %s\n", formatBreaks(breaks))
	}

	scaffoldRes, err := workspace.Scaffold(workspace.ScaffoldOptions{
		ProjectRoot: root,
		Force:       o.force,
		Catalog:     cat,
		Selection:   sel,
	})
	if err != nil {
		return fmt.Errorf("init: scaffold: %w", err)
	}

	if err := selection.Save(root, sel); err != nil {
		return fmt.Errorf("init: persist selection: %w", err)
	}

	discoveryRes, err := discovery.Run(root)
	if err != nil {
		return fmt.Errorf("init: discovery: %w", err)
	}

	written, err := constitution.Write(root, discoveryRes, time.Now())
	if err != nil {
		return fmt.Errorf("init: constitution: %w", err)
	}

	printInitSummary(out, root, scaffoldRes, written, discoveryRes, sel, cat)
	return nil
}

func printInitSummary(out io.Writer, root string, sr *workspace.ScaffoldResult, constitutionPath string, res discovery.Result, sel map[string]bool, cat *catalog.Catalog) {
	fmt.Fprintf(out, "bender: scaffolded %d files (%d preserved", len(sr.Created), len(sr.Preserved))
	if len(sr.Overwritten) > 0 {
		fmt.Fprintf(out, ", %d overwritten via --force", len(sr.Overwritten))
	}
	if len(sr.Removed) > 0 {
		fmt.Fprintf(out, ", %d removed", len(sr.Removed))
	}
	fmt.Fprintln(out, ")")

	included, excluded := splitSelection(sel, cat)
	fmt.Fprintf(out, "bender: selection — included [%s]; excluded [%s]\n", strings.Join(included, ", "), strings.Join(excluded, ", "))
	fmt.Fprintf(out, "bender: selection written → %s\n", selection.ManifestFileName)
	fmt.Fprintf(out, "bender: constitution written → %s\n", relPathOrAbs(root, constitutionPath))
	if !res.Stack.IsZero() {
		fmt.Fprintf(out, "  detected stack: %s", res.Stack.Language)
		if res.Stack.PackageManager != "" {
			fmt.Fprintf(out, " (%s)", res.Stack.PackageManager)
		}
		fmt.Fprintln(out)
	}
	if len(res.Pending) > 0 {
		fmt.Fprintln(out, "bender: pending sections:")
		for _, p := range res.Pending {
			fmt.Fprintf(out, "  - %s\n", p)
		}
	}
	fmt.Fprintln(out, "next: open this project in Claude Code and try `/cry \"<your request>\"`")
}

func splitSelection(sel map[string]bool, cat *catalog.Catalog) (included, excluded []string) {
	for _, id := range cat.IDs() {
		if sel[id] {
			included = append(included, id)
		} else {
			excluded = append(excluded, id)
		}
	}
	sort.Strings(included)
	sort.Strings(excluded)
	return
}

func formatBreaks(breaks []catalog.BreakReport) string {
	var lines []string
	for _, b := range breaks {
		lines = append(lines, fmt.Sprintf("  - %q would leave %v without a dependency", b.Deselected, b.Dependents))
	}
	return strings.Join(lines, "\n")
}

func relPathOrAbs(root, p string) string {
	if rel, err := filepath.Rel(root, p); err == nil {
		return rel
	}
	return p
}
