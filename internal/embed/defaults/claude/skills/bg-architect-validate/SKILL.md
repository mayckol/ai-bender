---
name: bg-architect-validate
context: bg
description: "Validate that the implementation respects the planned boundaries; detect drift between code and the planned data model / API contract."
provides: [architect, validate, boundaries, drift]
stages: [ghu, implement]
applies_to: [any]
---

# bg-architect-validate

Read-only validation pass during `/ghu`. Compares what `crafter` produced against what `architect` planned. Emits findings — never modifies code.

## Two checks

### 1. Boundary validation

Re-walk the dependency graph after `crafter` lands changes. For every new edge that crosses a module boundary the plan said to keep separate, emit a `high`-severity `finding_reported` citing both modules and the new import.

### 2. Drift detection

Compare the resolved types/endpoints in the source tree to the artifacts at `artifacts/plan/`:
- **Data model drift**: a struct/type/schema changed in code without updating `artifacts/plan/data-model-<ts>.md` → `medium`-severity finding.
- **API contract drift**: a new exposed endpoint / CLI flag / GraphQL field that doesn't appear in `artifacts/plan/api-contract-<ts>.md` → `high`-severity finding.

## Operating principles

- Read-only. Your write scope is empty for the source tree (only `artifacts/plan/**` is allowed, and you should not write there during `/ghu`).
- Findings cite both the plan reference and the divergent code location.
