---
name: bg-benchmarker-measure
context: bg
description: "Run the project's benchmarks and compare results against a baseline. Regressions are high-severity findings."
provides: [perf, benchmark, baseline]
stages: [ghu, implement]
applies_to: [any]
---

# bg-benchmarker-measure

Run the project's benchmarks if configured. Compare against the baseline (the most recent benchmark output committed under `.bender/artifacts/ghu/perf/baseline.json`, or whatever the project documents).

## Steps

1. **Resolve the benchmark command** from the constitution / Makefile:
   - Go: `go test -bench=. -benchmem ./...`
   - Cargo: `cargo bench`
   - Node: per `package.json` `bench` script.
   - Python: `pytest --benchmark-only` if `pytest-benchmark` is installed.
2. **Run the benchmarks**. Capture the per-benchmark numbers (ns/op, allocs/op, MB/s where applicable).
3. **Compare against the baseline**:
   - Regression > 5% → `high`-severity finding.
   - Regression 2–5% → `medium`.
   - Improvement → emit `info`-severity celebration finding (useful for PR summary).
4. **Update the baseline** when explicitly asked (`--update-baseline` to the orchestrator). Otherwise the baseline is read-only.

## Stop conditions

- The project has no benchmarks → emit `info`-severity finding "no benchmarks configured; static analysis from `bg-benchmarker-analyze` is the only signal".
- The benchmark command isn't documented and can't be inferred → emit `medium`-severity finding asking the user to fill in the constitution / Makefile.

## Output

Write the raw benchmark output to `.bender/artifacts/ghu/perf/<run-timestamp>/results.json` so future runs can diff against it.
