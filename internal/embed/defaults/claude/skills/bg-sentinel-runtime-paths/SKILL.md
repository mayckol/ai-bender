---
name: bg-sentinel-runtime-paths
context: bg
description: "Audit input-validation paths and authentication / authorization on protected endpoints. Find unsanitized inputs reaching dangerous sinks."
provides: [security, validation, auth]
stages: [ghu, implement]
applies_to: [any]
---

# bg-sentinel-runtime-paths

Two runtime checks: untrusted-input flow analysis and auth/authz coverage on protected endpoints.

## 1. Audit inputs

Trace user-controllable input from entry points to sinks. Flag any path where input reaches a dangerous sink without sanitisation:

| Sink | Pattern | Severity if reachable |
|---|---|---|
| SQL execution | string concatenation into a query | high |
| Shell exec / `os/exec` | user input as command or argv | high |
| Path open / file IO | user input as a path component | medium |
| HTML response | unescaped user input | medium |
| HTTP request URL | user input as the host (SSRF) | high |
| Deserialisation | untrusted bytes into `gob` / `pickle` / `Marshal` | high |

Cite source → sink path with file:line for each.

## 2. Check auth paths

For each protected endpoint identified in `artifacts/specs/<slug>-<ts>.md` or `artifacts/plan/api-contract-<ts>.md`:
1. Confirm the route handler invokes auth middleware (or equivalent) BEFORE reaching business logic.
2. Confirm authorisation is checked AFTER auth (role / permission / resource-owner).
3. Emit `high`-severity finding if either check is missing.

Also: protected handlers that don't appear in the spec → emit `info`-severity finding asking "should this be in the spec?".

## Output

`artifacts/ghu/security/<run-timestamp>/runtime-<finding-id>.md` per finding. Emit `finding_reported` events. Aggregate counts in the run report under "Findings → security".
