package main

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mayckol/ai-bender/internal/event"
	"github.com/mayckol/ai-bender/internal/session"
)

func newEventCmd(g *globalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "event",
		Short: "Emit events into a bender session's events.jsonl",
		Long: `The event subcommand tree gives skills and sub-actors a single, atomic
entry point for writing events to .bender/sessions/<id>/events.jsonl. Using
this command (rather than ad-hoc printf >> events.jsonl) guarantees
O_APPEND|O_SYNC semantics and a single fsync per event so fsnotify-based
tails see exactly one write per emit.`,
	}
	cmd.AddCommand(newEventEmitCmd(g))
	return cmd
}

func newEventEmitCmd(g *globalFlags) *cobra.Command {
	var (
		sessionID    string
		typeStr      string
		actorKind    string
		actorName    string
		payloadStr   string
		sessionsRoot string
	)
	cmd := &cobra.Command{
		Use:   "emit",
		Short: "Append one event line to a session's events.jsonl",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if sessionID == "" {
				return usageErr(cmd, errors.New("--session is required"))
			}
			if typeStr == "" {
				return usageErr(cmd, errors.New("--type is required"))
			}
			if actorKind == "" {
				return usageErr(cmd, errors.New("--actor-kind is required"))
			}
			if actorName == "" {
				return usageErr(cmd, errors.New("--actor-name is required"))
			}
			if payloadStr == "" {
				return usageErr(cmd, errors.New("--payload is required"))
			}
			var payload map[string]any
			if err := json.Unmarshal([]byte(payloadStr), &payload); err != nil {
				return usageErr(cmd, fmt.Errorf("--payload must be a JSON object: %w", err))
			}

			root := sessionsRoot
			if root == "" {
				projectRoot, err := resolveProjectRoot(g)
				if err != nil {
					return err
				}
				root = session.SessionsRoot(projectRoot)
			}
			return event.Emit(event.EmitParams{
				SessionsRoot: root,
				SessionID:    sessionID,
				Type:         event.Type(typeStr),
				ActorKind:    event.ActorKind(actorKind),
				ActorName:    actorName,
				Payload:      payload,
			})
		},
	}
	cmd.Flags().StringVar(&sessionID, "session", "", "session id (required)")
	cmd.Flags().StringVar(&typeStr, "type", "", "event type, e.g. orchestrator_progress (required)")
	cmd.Flags().StringVar(&actorKind, "actor-kind", "", "actor kind: orchestrator|agent|stage|sink|user (required)")
	cmd.Flags().StringVar(&actorName, "actor-name", "", "actor name, e.g. ghu, scout (required)")
	cmd.Flags().StringVar(&payloadStr, "payload", "", "JSON object carrying the event payload (required)")
	cmd.Flags().StringVar(&sessionsRoot, "sessions-root", "", "absolute path to .bender/sessions (overrides project resolution; required when invoked from a worktree)")
	return cmd
}

// usageErr marks the error as argument-validation so the cobra wrapper can map
// to exit code 2 (per contracts/event-emit-cli.md).
func usageErr(cmd *cobra.Command, err error) error {
	cmd.SilenceUsage = false
	return err
}
