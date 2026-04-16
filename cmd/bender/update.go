package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/mayckol/ai-bender/internal/version"
)

const (
	ExitUpdateNetwork    = 20
	ExitUpdateIntegrity  = 21
	ExitUpdatePermission = 22
)

func newUpdateCmd(_ *globalFlags) *cobra.Command {
	var checkOnly bool
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Replace the current bender binary with the latest release (use --check to inspect only)",
		Long: `update fetches the latest released bender binary, verifies its checksum, and atomically replaces
the running binary. With --check it only prints the available version without modifying anything.

v1 ships without a release server. Until one is published, this command always reports the current
version as up-to-date. Use the project's release notes for upgrade guidance in the meantime.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if checkOnly {
				fmt.Fprintf(cmd.OutOrStdout(), "current: %s\nlatest: %s (no release channel configured in v1)\n", version.Version, version.Version)
				return nil
			}
			fmt.Fprintln(cmd.ErrOrStderr(), "bender update: no release channel configured in v1; install via go install / Homebrew / curl|sh until a release server is published")
			os.Exit(ExitUpdateNetwork)
			return nil
		},
	}
	cmd.Flags().BoolVar(&checkOnly, "check", false, "print the available version without modifying anything")
	return cmd
}
