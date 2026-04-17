// Package main is the entrypoint for the bender CLI.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/mayckol/ai-bender/internal/version"
)

// globalFlags holds the values bound to the global persistent flags.
type globalFlags struct {
	project string
	noColor bool
	quiet   bool
	verbose bool
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		// Cobra already prints the error; just exit non-zero.
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	g := &globalFlags{}
	root := &cobra.Command{
		Use:   "bender",
		Short: "Spec-driven scaffold for Claude Code",
		Long: `bender scaffolds a .claude/ workspace into a target project, manages a multi-project
registry, validates the catalog, and inspects on-disk session artifacts. The slash commands
(/cry, /plan, /tdd, /ghu, /implement) are markdown files Claude Code reads and executes;
this binary itself does not invoke any LLM.`,
		Version:       version.Resolve(),
		SilenceUsage:  true,
		SilenceErrors: false,
	}
	root.PersistentFlags().StringVar(&g.project, "project", "", "registered project name to operate on")
	root.PersistentFlags().BoolVar(&g.noColor, "no-color", false, "disable color output")
	root.PersistentFlags().BoolVar(&g.quiet, "quiet", false, "suppress informational logs")
	root.PersistentFlags().BoolVar(&g.verbose, "verbose", false, "print loader and resolver decisions to stderr")

	root.AddCommand(newVersionCmd())
	root.AddCommand(newInitCmd(g))
	root.AddCommand(newRegisterProjectCmd(g))
	root.AddCommand(newListProjectsCmd(g))
	root.AddCommand(newDoctorCmd(g))
	root.AddCommand(newSessionsCmd(g))
	root.AddCommand(newServerCmd(g))
	root.AddCommand(newSyncDefaultsCmd(g))
	root.AddCommand(newUpdateCmd(g))
	return root
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print bender version and exit",
		RunE: func(_ *cobra.Command, _ []string) error {
			_, err := fmt.Println(version.Resolve())
			return err
		},
	}
}
