---
name: bg-tester-write-and-run
context: bg
description: "Author executable tests + fixtures for the change set, run the suite, report total coverage and diff coverage."
provides: [test, code-generation, coverage]
stages: [ghu, implement]
applies_to: [any]
---

# bg-tester-write-and-run

Author executable tests for the new code in the change set, run the test suite, and report coverage. Combines what `bg-tester-write-{unit,integration,fixture}` and `bg-tester-run-{suite,coverage,diff-coverage}` used to do separately.

## Steps

1. **Identify the change set**: every source file modified or created by `bg-crafter-implement` in this run.
2. **Author tests** following the language's convention (resolved from the constitution):
   - **Unit**: one test file per production file. Cover acceptance criteria + edge cases. Avoid mocks at the boundary itself; use them for collaborators.
   - **Integration**: real adapters where the boundary lives (DB, HTTP, filesystem). One file per integration surface.
   - **Fixtures**: small, human-readable, under `tests/fixtures/` or `testdata/`.
3. **Run the suite** using the test command from the constitution (`go test ./...`, `npm test`, `pytest`, `cargo test`, etc.).
4. **Compute coverage**:
   - Total + per-package using the project's coverage tool.
   - **Diff coverage**: percentage of changed lines covered by new tests.
5. **Report**:
   - On any test failure: emit `finding_reported` with severity `high`, the failing test name, and a one-line diagnosis.
   - On uncovered new code: emit `finding_reported` with severity `medium`, the file/range, and the suggested test case.

## Stop conditions

- Tests are red on entry (not by your changes) → emit `high`-severity finding and stop. Do not author new tests on top of broken ones.
- The project doesn't have a test command in the constitution → emit `medium`-severity finding asking the user to fill in the Tests section.

## What you DO NOT do

- Modify production code. Your write scope denies it.
- Write prose-only TDD scaffolds — that's `fg-tester-scaffold` during `/tdd`.
