---
name: fg-tester-scaffold-structure
context: fg
description: "Mirror the source tree under artifacts/plan/tests/ with one scaffold per source file needing coverage."
provides: [tdd, scaffold]
stages: [tdd]
applies_to: [any]
---

# fg-tester-scaffold-structure

Inspect the plan's affected_files. For each, create artifacts/plan/tests/<source-path>-<ts>.md.
