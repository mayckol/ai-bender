---
name: bg-linter-run-and-fix
context: bg
description: "Detect lint config, run the linter against the change set, apply safe autofixes (formatting + import sorting), commit only those changes."
provides: [lint, autofix, format]
stages: [ghu, implement]
applies_to: [any]
---

# bg-linter-run-and-fix

Run the project's lint and formatter toolchain against the change set, applying ONLY safe autofixes. Semantic refactors are out of scope — those belong to `surgeon` or `crafter`.

## Steps

1. **Detect the lint config** from the constitution's Lint section:
   - `.golangci.yml` → `golangci-lint run`
   - `.eslintrc.*` / `eslint.config.*` → `eslint`
   - `.rubocop.yml` → `rubocop`
   - `pyproject.toml` (ruff/black/mypy) → `ruff check`, `black`, `mypy`
   - `Cargo.toml` → `cargo clippy`
   - Always run the language formatter: `gofmt`, `prettier --write`, `rustfmt`, `black`, etc.
2. **Run the linter** with default scope = changed files only. On `--all`: whole tree.
3. **Apply safe autofixes**: the linter's `--fix` flag plus the formatter. **Refuse semantic changes**: rule rewrites, dead-code removal, API renames.
4. **Run the formatter** unconditionally on the change set so style stays consistent.
5. **Commit only formatting and import-sorting changes**. If a "fix" would alter behavior, skip it and let `bg-linter-report` flag it instead.

## Stop conditions

- Lint config missing AND linter expected (per constitution) → emit `medium`-severity finding.
- Autofix would change observable behavior → skip and emit a finding describing what would have changed.
