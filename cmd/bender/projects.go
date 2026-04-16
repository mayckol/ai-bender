package main

import (
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/mayckol/ai-bender/internal/workspace"
)

func newRegisterProjectCmd(_ *globalFlags) *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "register-project <path>",
		Short: "Register a project root in the global workspace registry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			registered, entry, err := workspace.Register(name, args[0])
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "registered: %s → %s\n", registered, entry.Path)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "explicit project name (kebab-case); defaults to a kebab-cased basename")
	return cmd
}

func newListProjectsCmd(_ *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "list-projects",
		Short: "List registered projects in the global workspace registry",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			listings, err := workspace.List(cwd)
			if err != nil {
				return err
			}
			if len(listings) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no projects registered (use `bender register-project <path>`)")
				return nil
			}
			return renderListings(cmd.OutOrStdout(), listings)
		},
	}
}

func renderListings(out io.Writer, listings []workspace.ProjectListing) error {
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tSTATUS\tPATH")
	for _, l := range listings {
		fmt.Fprintf(w, "%s\t%s\t%s\n", l.Name, l.Status, l.Entry.Path)
	}
	return w.Flush()
}
