package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/mayckol/ai-bender/internal/constitution"
	"github.com/mayckol/ai-bender/internal/discovery"
	"github.com/mayckol/ai-bender/internal/workspace"
)

type initOpts struct {
	here  bool
	force bool
}

func newInitCmd(g *globalFlags) *cobra.Command {
	o := &initOpts{}
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Scaffold .claude/ and write artifacts/constitution.md (idempotent)",
		Long: `init materialises the embedded .claude/ defaults into the current project (or --here),
runs heuristic discovery (no AI calls), and writes artifacts/constitution.md. Existing user-modified
files are preserved unless --force is passed.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runInit(cmd.OutOrStdout(), g, o)
		},
	}
	cmd.Flags().BoolVar(&o.here, "here", false, "explicitly initialise in the current directory (default behaviour)")
	cmd.Flags().BoolVar(&o.force, "force", false, "overwrite user-modified files in .claude/ with embedded defaults")
	return cmd
}

func runInit(out io.Writer, _ *globalFlags, o *initOpts) error {
	root, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("init: get cwd: %w", err)
	}
	if o.here {
		// Explicit affirmation; no path change required.
	}

	scaffoldRes, err := workspace.Scaffold(workspace.ScaffoldOptions{
		ProjectRoot: root,
		Force:       o.force,
	})
	if err != nil {
		return fmt.Errorf("init: scaffold: %w", err)
	}

	res, err := discovery.Run(root)
	if err != nil {
		return fmt.Errorf("init: discovery: %w", err)
	}

	written, err := constitution.Write(root, res, time.Now())
	if err != nil {
		return fmt.Errorf("init: constitution: %w", err)
	}

	printInitSummary(out, root, scaffoldRes, written, res)
	return nil
}

func printInitSummary(out io.Writer, root string, sr *workspace.ScaffoldResult, constitutionPath string, res discovery.Result) {
	fmt.Fprintf(out, "bender: scaffolded %d files (%d preserved", len(sr.Created), len(sr.Preserved))
	if len(sr.Overwritten) > 0 {
		fmt.Fprintf(out, ", %d overwritten via --force", len(sr.Overwritten))
	}
	fmt.Fprintln(out, ")")
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

func relPathOrAbs(root, p string) string {
	if rel, err := filepath.Rel(root, p); err == nil {
		return rel
	}
	return p
}
