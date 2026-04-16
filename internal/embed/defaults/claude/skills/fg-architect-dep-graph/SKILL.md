---
name: fg-architect-dep-graph
context: fg
description: "Build a dependency graph of modules; flag cycles."
provides: [architect, deps]
stages: [plan]
applies_to: [architectural]
---

# fg-architect-dep-graph

Walk imports across the codebase. Render a Mermaid graph in the data-model artifact. Reject cycles.
