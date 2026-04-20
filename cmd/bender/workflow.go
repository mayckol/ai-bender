package main

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mayckol/ai-bender/internal/workflow"
)

func newWorkflowCmd(g *globalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workflow",
		Short: "Link consecutive sessions into one workflow for the live UI",
		Long: `The workflow subcommand tree lets a skill resolve a workflow id before it
creates a new session. Workflow ids group /tdd → /ghu (and similar consecutive
sessions on the same feature branch) so the UI can render their events as one
continuous timeline.`,
	}
	cmd.AddCommand(newWorkflowResolveCmd(g))
	return cmd
}

func newWorkflowResolveCmd(g *globalFlags) *cobra.Command {
	var key string
	cmd := &cobra.Command{
		Use:   "resolve",
		Short: "Return the workflow id to use for the next session (JSON)",
		Long: `Print a JSON object with "workflow_id" and, when inheriting from a prior
session, "parent_session_id". The skill calls this before ` + "`bender worktree create`" + `
and passes the values through via --workflow-id / --workflow-parent.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if key == "" {
				return errors.New("workflow resolve: --key is required")
			}
			root, err := resolveProjectRoot(g)
			if err != nil {
				return err
			}
			lk, err := workflow.Resolve(workflow.ResolveParams{
				ProjectRoot: root,
				Key:         key,
			})
			if err != nil {
				return err
			}
			out := map[string]string{
				"workflow_id": lk.WorkflowID,
			}
			if lk.ParentSession != "" {
				out["parent_session_id"] = lk.ParentSession
			}
			if lk.WorkflowIsNew {
				out["new"] = "true"
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			if err := enc.Encode(out); err != nil {
				return fmt.Errorf("workflow resolve: encode: %w", err)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&key, "key", "", "grouping key (typically the current git branch) — required")
	return cmd
}
