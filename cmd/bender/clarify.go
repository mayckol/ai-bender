package main

import (
	"errors"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/mayckol/ai-bender/internal/clarification"
	"github.com/mayckol/ai-bender/internal/version"
)

type clarifyFlags struct {
	nonInteractive bool
	strict         bool
	fromSpec       string
	fromCapture    string
	timestamp      string
	out            string
	sessionDir     string
	in             string
}

func newClarifyCmd(_ *globalFlags) *cobra.Command {
	f := &clarifyFlags{}
	cmd := &cobra.Command{
		Use:           "clarify",
		Short:         "Run the plan-stage clarification round (interactive or non-interactive)",
		Long:          "Reads a Batch JSON describing detected ambiguities, prompts the practitioner (or emits pending entries in non-interactive mode), persists the clarifications artifact, applies resolutions back into the spec draft, and emits the new clarifications_* events.",
		SilenceUsage:  true,
		SilenceErrors: false,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !f.nonInteractive {
				if v := os.Getenv("BENDER_NONINTERACTIVE"); v == "1" || v == "true" {
					f.nonInteractive = true
				}
				if !f.nonInteractive && !term.IsTerminal(int(os.Stdin.Fd())) {
					f.nonInteractive = true
				}
			}
			if !f.strict {
				if v := os.Getenv("BENDER_CLARIFICATIONS_STRICT"); v == "1" || v == "true" {
					f.strict = true
				}
			}
			opts := clarification.Options{
				NonInteractive: f.nonInteractive,
				Strict:         f.strict,
				FromSpec:       f.fromSpec,
				FromCapture:    f.fromCapture,
				Timestamp:      f.timestamp,
				OutPath:        f.out,
				SessionDir:     f.sessionDir,
				InputPath:      f.in,
				ToolVersion:    version.Resolve(),
				Stdout:         cmd.OutOrStdout(),
				Stderr:         cmd.ErrOrStderr(),
				Stdin:          cmd.InOrStdin(),
			}
			err := clarification.Run(cmd.Context(), opts)
			if errors.Is(err, clarification.ErrPending) {
				os.Exit(2)
			}
			return err
		},
	}

	cmd.Flags().BoolVar(&f.nonInteractive, "non-interactive", false, "skip the interactive prompt; emit pending entries instead")
	cmd.Flags().BoolVar(&f.strict, "strict", false, "in non-interactive mode, exit before continuing the plan flow when pending entries remain")
	cmd.Flags().StringVar(&f.fromSpec, "from-spec", "", "path to the spec draft to apply resolutions into")
	cmd.Flags().StringVar(&f.fromCapture, "from-capture", "", "path to the source capture artifact (used as the reuse key)")
	cmd.Flags().StringVar(&f.timestamp, "timestamp", "", "shared plan-set timestamp; controls the artifact filename")
	cmd.Flags().StringVar(&f.out, "out", "", "output path for the clarifications artifact (defaults to .bender/artifacts/plan/clarifications-<timestamp>.md)")
	cmd.Flags().StringVar(&f.sessionDir, "session-dir", "", "session directory used for state.json updates and event emission")
	cmd.Flags().StringVar(&f.in, "input", "", "path to the Batch JSON input (defaults to stdin)")
	return cmd
}
