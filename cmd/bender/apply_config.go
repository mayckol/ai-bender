package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/spf13/cobra"

	"github.com/mayckol/ai-bender/internal/agent"
	"github.com/mayckol/ai-bender/internal/config"
)

const (
	ExitConfigMissingAgent = 70
	ExitConfigParseFailed  = 71
)

func newApplyConfigCmd(g *globalFlags) *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "apply-config",
		Short: "Bake .bender/config.yaml agent overrides into .claude/agents/*.md",
		Long: `Reads .bender/config.yaml and rewrites the matching files under .claude/agents/
so Claude Code's effective agent frontmatter reflects the declared overrides.

Idempotent: re-running with the same config produces no changes. Use --dry-run
to preview which files would be touched without writing.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := resolveProjectRoot(g)
			if err != nil {
				return err
			}
			return runApplyConfig(cmd, root, dryRun)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what would change without rewriting any file")
	return cmd
}

func runApplyConfig(cmd *cobra.Command, root string, dryRun bool) error {
	cfgPath := filepath.Join(root, ".bender", "config.yaml")
	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "apply-config: %v\n", err)
		os.Exit(ExitConfigParseFailed)
	}

	out := cmd.OutOrStdout()
	if len(cfg.Agents) == 0 {
		fmt.Fprintln(out, "apply-config: no agent overrides declared in .bender/config.yaml")
		return nil
	}

	names := make([]string, 0, len(cfg.Agents))
	for name := range cfg.Agents {
		names = append(names, name)
	}
	sort.Strings(names)

	var touched, unchanged int
	var missing []string

	for _, name := range names {
		ov := cfg.Agents[name]
		agentPath := filepath.Join(root, ".claude", "agents", name+".md")
		if _, err := os.Stat(agentPath); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				missing = append(missing, name)
				continue
			}
			return err
		}

		if dryRun {
			original, err := os.ReadFile(agentPath)
			if err != nil {
				return err
			}
			parsed, err := agent.ParseAgent(original)
			if err != nil {
				return fmt.Errorf("apply-config: %s: %w", agentPath, err)
			}
			before := snapshotBindings(parsed)
			agent.ApplyOverride(parsed, ov)
			after := snapshotBindings(parsed)
			if before == after {
				fmt.Fprintf(out, "  (unchanged) %s\n", relOrAbs(root, agentPath))
				unchanged++
				continue
			}
			fmt.Fprintf(out, "  (would update) %s\n    before: %s\n    after:  %s\n",
				relOrAbs(root, agentPath), before, after)
			touched++
			continue
		}

		changed, err := agent.RewriteFile(agentPath, ov)
		if err != nil {
			return err
		}
		if changed {
			fmt.Fprintf(out, "  updated   %s\n", relOrAbs(root, agentPath))
			touched++
		} else {
			fmt.Fprintf(out, "  unchanged %s\n", relOrAbs(root, agentPath))
			unchanged++
		}
	}

	fmt.Fprintf(out, "\napply-config: %d updated, %d unchanged", touched, unchanged)
	if len(missing) > 0 {
		sort.Strings(missing)
		fmt.Fprintf(out, ", %d missing: %v", len(missing), missing)
	}
	fmt.Fprintln(out)

	if len(missing) > 0 {
		fmt.Fprintf(cmd.ErrOrStderr(),
			"\napply-config: %d agent(s) referenced in config.yaml have no file under .claude/agents/.\n",
			len(missing))
		fmt.Fprintln(cmd.ErrOrStderr(),
			"Run `bender sync-defaults` to materialize the embedded defaults, or create the agent files manually, then re-run.")
		os.Exit(ExitConfigMissingAgent)
	}
	return nil
}

func snapshotBindings(a *agent.Agent) string {
	return fmt.Sprintf("skills(explicit=%v patterns=%v tags.any=%v tags.none=%v) scope(allow=%v deny=%v)",
		a.Skills.Explicit, a.Skills.Patterns, a.Skills.Tags.AnyOf, a.Skills.Tags.NoneOf,
		a.WriteScope.Allow, a.WriteScope.Deny,
	)
}

func relOrAbs(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil || len(rel) == 0 || rel[0] == '.' && len(rel) >= 2 && rel[1] == '.' {
		return path
	}
	return rel
}
