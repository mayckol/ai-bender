---
name: bg-sentinel-audit-inputs
context: bg
description: "Audit input validation paths for SQL injection, command injection, SSRF, etc."
provides: [security, validation]
stages: [ghu, implement]
applies_to: [any]
---

# bg-sentinel-audit-inputs

Trace user input from entry points to sinks. Flag any unsanitized path.
