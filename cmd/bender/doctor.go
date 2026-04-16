package main

import (
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/mayckol/ai-bender/internal/doctor"
	"github.com/mayckol/ai-bender/internal/workspace"
)

// Doctor exit codes per `contracts/cli.md`.
const (
	ExitDoctorWarning  = 40
	ExitDoctorError    = 41
)

func newDoctorCmd(g *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Validate the catalog: empty selectors, broken patterns, missing tools, duplicate names",
		RunE: func(cmd *cobra.Command, _ []string) error {
			projectRoot, err := resolveProjectRoot(g)
			if err != nil {
				return err
			}
			r, err := doctor.Run(projectRoot)
			if err != nil {
				return err
			}
			renderReport(cmd.OutOrStdout(), r)
			if r.HasErrors() {
				os.Exit(ExitDoctorError)
			}
			if r.HasWarnings() {
				os.Exit(ExitDoctorWarning)
			}
			return nil
		},
	}
}

func resolveProjectRoot(g *globalFlags) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	_, path, err := workspace.Resolve(g.project, cwd)
	if err != nil {
		return "", err
	}
	if path == "" {
		// No registered project; operate against cwd.
		return cwd, nil
	}
	return path, nil
}

func renderReport(out io.Writer, r *doctor.Report) {
	fmt.Fprintf(out, "bender doctor: %d skills, %d agents, %d groups loaded\n", r.SkillCount, r.AgentCount, r.GroupCount)
	if len(r.Issues) > 0 {
		w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "SEVERITY\tCATEGORY\tSUBJECT\tMESSAGE")
		for _, i := range r.Issues {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", i.Severity, i.Category, i.Subject, i.Message)
		}
		_ = w.Flush()
	}
	switch {
	case r.HasErrors():
		fmt.Fprintln(out, "status: errors present (slash commands may fail)")
	case r.HasWarnings():
		fmt.Fprintln(out, "status: warnings present")
	default:
		fmt.Fprintln(out, "status: healthy")
	}
}
