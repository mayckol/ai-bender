package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mayckol/ai-bender/internal/workspace"
)

func newSyncDefaultsCmd(g *globalFlags) *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "sync-defaults",
		Short: "Re-materialize embedded defaults; preserves user-modified files unless --force",
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := resolveProjectRoot(g)
			if err != nil {
				return err
			}
			res, err := workspace.Scaffold(workspace.ScaffoldOptions{
				ProjectRoot: root,
				Force:       force,
			})
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "sync-defaults: %d added, %d preserved", len(res.Created), len(res.Preserved))
			if len(res.Overwritten) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), ", %d overwritten via --force", len(res.Overwritten))
			}
			fmt.Fprintln(cmd.OutOrStdout())
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "overwrite user-modified files with embedded defaults")
	return cmd
}
