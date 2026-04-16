---
name: bg-reviewer-critique
context: bg
description: "Critique the change set against spec, constitution, language idioms, and test quality. Cite, never fix."
provides: [review, spec, constitution, idioms, test-quality]
stages: [ghu, implement]
applies_to: [any]
---

# bg-reviewer-critique

Read the change set produced by `crafter` and `tester`, compare it to the approved spec, the project constitution, the language/framework idioms, and the quality of the new tests. Emit findings — **never fix** what you flag.

## Four critique dimensions

### 1. vs spec
For each section of the approved spec at `.bender/artifacts/specs/<slug>-<ts>.md`, verify the implementation honors it. Cite spec section + file + line range when the implementation drifts.

### 2. vs constitution
Read `.bender/artifacts/constitution.md`. For each rule in Conventions (naming, error handling, DI, architecture pattern), verify the change respects it. Constitution rules win over personal style preferences.

### 3. Language / framework idioms
Use the constitution's Stack section to know which idioms apply:
- **Go**: error wrapping with `%w`, table-driven tests, no panic in library code, context as first param.
- **TypeScript / React**: hooks, no class components in new code, prefer `unknown` over `any`.
- **Python**: type hints on public APIs, prefer `pathlib`, context managers for resources.
- **Rust**: idiomatic ownership, prefer `?` over manual match, no unnecessary `.unwrap()` in library code.

### 4. Test quality
Critique the new tests authored this run:
- Coverage of acceptance criteria.
- Edge cases (nil/empty/boundary/concurrent).
- Test isolation (no test depends on another).
- Test naming clarity.

## Output

Write findings to `.bender/artifacts/ghu/reviews/<run-timestamp>/critique-<task>.md`. Each finding includes:

```yaml
- id: <stable id>
  severity: info | low | medium | high | critical
  category: spec | constitution | idioms | test-quality
  title: <one line>
  description: <full body>
  location:
    path: <repo-relative>
    line_start: <int>
    line_end: <int>
  rule_violated: <spec section or constitution rule>
```

Distinguish structural issues (severity ≥ medium) from style nits (info). Do not bury one in the other.

## What you DO NOT do

- Modify the files you critique. If you can write the fix, you can also write the finding. The fix belongs to `crafter` or `surgeon`.
