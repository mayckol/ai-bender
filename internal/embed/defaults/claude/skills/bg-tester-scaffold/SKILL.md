---
name: bg-tester-scaffold
context: bg
description: "Turn prose TDD scaffolds into commented-out test stubs at the real test paths. Crafter fills them in during implementation."
provides: [test, scaffold, code-generation]
stages: [ghu, implement]
applies_to: [any]
---

# bg-tester-scaffold

Runs during `/ghu` / `/implement` in **TDD mode** as the first step after `surgeon` (if any). Reads the prose scaffolds at `.bender/artifacts/plan/tests/**/*.md` (produced by `/tdd` + `/tdd confirm`) and writes **commented-out test stubs** at the real test paths. The stubs are NOT runnable tests — they are a blueprint for `bg-crafter-implement` to uncomment and activate as it builds the production code.

## Why commented-out instead of red tests?

Classic TDD requires red runnable tests that force compilation errors. In practice that fights against modern type-checked languages: the test can't reference a type that doesn't exist yet, so authoring a "failing test" against missing code is noisy. Commented-out stubs are less friction:

- the structure of the test is authoritative (name, mock setup, expected assertions);
- `crafter` sees exactly what to build against;
- `crafter` uncomments lines as it builds the matching production code;
- `bg-tester-run` verifies the final green state once implementation is done.

## Steps

1. **Locate the prose scaffolds** approved under `.bender/artifacts/plan/tests/**/*.md` (every file must have `status: approved`). Group them by the source file they `mirrors:`.
2. **Pick the target path** for each scaffold using the project's convention:
   - Go: sibling `_test.go` file in the same package.
   - TypeScript / JavaScript: sibling `.test.ts` / `.spec.ts` or colocated `__tests__/` file.
   - Python: `tests/` mirror with `test_<name>.py`.
   - Rust: `#[cfg(test)]` module at the bottom of the source file, or `tests/` for integration.
3. **Emit one test function per prose test case** in the scaffold. For every test:
   - Write the function signature with the language's testing harness (`func TestX(t *testing.T)`, `test("...", () => {})`, `def test_x():`, etc.).
   - Write a one-line narrative comment describing the case (copy from the scaffold's prose).
   - **Comment out** the mock setup, the subject-under-test construction, and every assertion. Use the language's single-line comment syntax.
   - Leave a `// TODO - implement the Code` marker so `crafter` can grep for it.
4. **Respect write scope**: only touch paths matching the tester agent's `write_scope.allow` (tests / specs / fixtures / testdata). Refuse and emit `skill_failed` otherwise.
5. **Emit one `file_changed` event** per stub file written. Include `lines_added` and `agent: "tester"`.
6. **Do not run the suite here.** Running is `bg-tester-run`'s job, after `bg-crafter-implement` has finished.

## Example output (Go)

Prose scaffold input (`.bender/artifacts/plan/tests/pkg/infra/handler-<ts>.md`):

```md
### TestNewInfraHandler
- Preconditions: feature-flag client mock
- Steps: construct NewInfraHandler(mockClient)
- Expected: returned handler is non-nil and ConfigcatClient field equals the mock
```

Generated stub (`pkg/infra/handler_test.go`):

```go
func TestNewInfraHandler(t *testing.T) {
	t.Parallel()
	// mockClient := &featureflag.FeatureFlagMock{}

	// TODO - implement the Code
	// handler := NewInfraHandler(mockClient)

	// assert.NotNil(t, handler)
	// assert.Equal(t, mockClient, handler.ConfigcatClient)
}
```

## What you DO NOT do

- Write runnable tests. That happens only after `crafter` implements; at that point `crafter` activates the assertions and `bg-tester-run` verifies.
- Modify production code. Your write scope denies it.
- Re-run the test suite — that's `bg-tester-run` at the end of the RED-GREEN cycle.
- Touch prose scaffolds under `.bender/artifacts/plan/tests/`. They are read-only input here; the output lives at the real test paths.
