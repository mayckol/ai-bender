---
name: fg-bootstrap-detect-dependencies
context: fg
description: "List direct dependencies with versions and outdated flags."
provides: [detect, dependencies]
stages: [bootstrap]
applies_to: [any]
---

# fg-bootstrap-detect-dependencies

Parse the manifest, list direct deps with versions. (Outdated detection is best-effort; offline runs may skip.)
