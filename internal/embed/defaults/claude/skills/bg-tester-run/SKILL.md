---
name: bg-tester-run
context: bg
description: "Run the test suite using the resolved project test command. Reports pass/fail, coverage, and diff coverage."
provides: [test, run, coverage]
stages: [ghu, implement]
applies_to: [any]
---

# bg-tester-run

Runs the project's test suite. Invoked twice during a typical `/ghu` TDD cycle:

1. After `bg-tester-scaffold` has written commented-out stubs — at this point the suite either skips the stub tests (they have no assertions) or registers them as passing no-ops. That's the expected baseline.
2. After `bg-crafter-implement` activates the assertions and finishes implementing — at this point the suite MUST pass.

In plain mode `bg-tester-write-and-run` handles both authoring and running; this skill is specifically for TDD mode where authoring and running are split across steps.

## Resolving the test command

Order of precedence:

1. The project constitution's Tests section (`.bender/artifacts/constitution.md`) — if it declares a concrete command (e.g. `test_command: "go test -race -count=1 ./..."`), use it verbatim.

2. **Autodetect** from marker files, in this order:
   - `go.mod`                                → `go test -race -count=1 ./...`
   - `pyproject.toml` or `pytest.ini`         → `pytest -q`
   - `package.json` (+ `bun.lock`)            → `bun test`
   - `package.json`                           → `npm test --silent`
   - `Cargo.toml`                             → `cargo test`
   - `pom.xml` / `build.gradle(.kts)`         → `mvn -q test` or `gradle test`
   - `composer.json`                          → `vendor/bin/phpunit --no-coverage`

If nothing matches, emit `skill_failed` with category `"test_runner_missing"` and a suggestion to pin the test command in `.bender/artifacts/constitution.md` under a `Tests` section.

## Steps

1. **Resolve the command** per above.
2. **Execute** with `cwd` set to the project root (or the configured `cwd`). Enforce `timeout_seconds` if set.
3. **Parse the result**:
   - Exit code 0 → green. Emit `skill_completed` with `result: "green"` and counts (`tests_total`, `tests_passed`, `tests_skipped`).
   - Non-zero → red. Emit `finding_reported` per failing test (severity `high`, category `test_failure`) with the test name, file:line, and the first failure message. If `verbose_on_failure: true`, re-run the failing packages with the language's verbose flag and attach the tail of the output.
4. **Coverage (optional)**: if the runner emits coverage artifacts (e.g. `coverage.out` from Go, `coverage.xml` from pytest), parse them and include the total + per-package numbers in `skill_completed.payload.coverage`. Not required for TDD mode — crafter's feedback loop already uses pass/fail.
5. **Never modify source files.** This skill only runs tests and reports — its write scope is read-only.

## What you DO NOT do

- Author new tests. Authoring happens in `bg-tester-scaffold` (TDD mode) or `bg-tester-write-and-run` (plain mode).
- Modify production code.
- Install language toolchains. If the runner is missing from PATH, emit `skill_failed` and let the orchestrator surface it.
