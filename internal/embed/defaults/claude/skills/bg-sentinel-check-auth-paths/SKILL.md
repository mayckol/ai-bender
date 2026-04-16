---
name: bg-sentinel-check-auth-paths
context: bg
description: "Verify authentication and authorization checks on protected endpoints."
provides: [security, auth]
stages: [ghu, implement]
applies_to: [any]
---

# bg-sentinel-check-auth-paths

Identify protected endpoints from the spec; verify each has auth/authz before reaching the business logic.
