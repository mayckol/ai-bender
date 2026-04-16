---
name: bg-benchmarker-analyze
context: bg
description: "Static performance analysis: hot paths, N+1 queries, allocation hot spots in the change set. No measurement; flag suspects for benchmark."
provides: [perf, hotpaths, n+1, alloc]
stages: [ghu, implement]
applies_to: [any]
---

# bg-benchmarker-analyze

Read the change set and flag patterns that *might* be performance hot spots. No measurement here — `bg-benchmarker-measure` does the actual benchmarks. This skill produces hypotheses worth measuring.

## Three patterns

### 1. Hot paths
Inspect loops, sync primitives, and allocation calls in the new code:
- Nested loops over user-supplied data → `info`-severity "needs measurement".
- Mutex/lock acquisition inside a loop → `info`-severity "needs measurement".
- New goroutine spawned per item in a loop → `medium`-severity "review concurrency model".

### 2. N+1 queries
Trace database/HTTP calls inside loops:
- `for _, x := range xs { db.QueryRow("...") }` → `medium` ("expected" but worth confirming).
- `for _, x := range xs { client.Get(...) }` → `medium`.
Suggest batching or eager loading in the finding description.

### 3. Allocation audit
Flag allocations inside hot loops in the change set:
- Slice/map literal allocated each iteration where reuse via pooling is possible → `info`.
- String concatenation inside a loop without `strings.Builder` (Go) / `StringBuilder` (Java/Kotlin) → `low`.
- Repeated boxing/unboxing of value types → `info`.

## Output

Findings with severity calibrated to "is this measurable now":
- Already measurable (the project has benchmarks for the function in question) → wait for `bg-benchmarker-measure` to confirm before raising severity.
- Not measurable today → `info`-severity with "needs measurement" tag and the suggested benchmark.

Cite the workload (input size, concurrency level) in every finding so future runs are comparable.
