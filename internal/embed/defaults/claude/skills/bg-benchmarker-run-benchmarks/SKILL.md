---
name: bg-benchmarker-run-benchmarks
context: bg
description: "Run the project's benchmarks if configured."
provides: [perf, run]
stages: [ghu, implement]
applies_to: [performance]
---

# bg-benchmarker-run-benchmarks

Use the constitution's benchmark command. Capture results; compare against baseline if available.
