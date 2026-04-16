---
name: bender-doctor
context: fg
description: "Validate the catalog using the bender binary; surface empty skill sets, broken selectors, missing tools, and override conflicts."
provides: [stage, doctor, validation]
stages: [doctor]
applies_to: [any]
inputs: []
outputs: []
requires_tools: []
---

# `/bender-doctor` — Validate the Catalog

Wrapper around `bender doctor`. Runs the binary's catalog walker, then summarises the output for the user.

## Workflow

1. Use the **Bash tool** to run:

   ```bash
   bender doctor
   ```

2. Capture exit code:
   - `0` → healthy.
   - `40` → warnings (missing tools, ambiguous overrides). Continue but flag.
   - `41` → blocking errors (parse failures, empty skill sets that would break a slash command).

3. Parse and summarise the output:
   - Per-agent effective skill set: list each agent's bound skill count.
   - Conflicts: list each conflict with file path and line.
   - Missing tools: list each `requires_tools` entry not on PATH, grouped by skill.

4. Print a recommendation:
   - Healthy: "catalog is healthy; you can run `/cry`, `/plan`, `/ghu` safely."
   - Warnings: "warnings present; review and fix before relying on affected skills."
   - Blocking: "blocking errors present; **do not** run `/ghu` until resolved."

## Notes

- This skill does no AI work itself — it only invokes the binary and renders the output.
- If the `bender` binary is not on PATH, fall back to `.specify/scripts/bash/bender-doctor.sh` (or PowerShell on Windows). The skill's `requires_tools` is intentionally empty so `bender doctor` does not warn when invoked from environments without the binary on PATH (the fallback covers them).
