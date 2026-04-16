---
name: bg-sentinel-scan-deps-cve
context: bg
description: "Cross-reference dependencies against known CVEs."
provides: [security, cve]
stages: [ghu, implement]
applies_to: [any]
---

# bg-sentinel-scan-deps-cve

Best-effort offline check; if a CVE database is bundled, use it. Emit findings with CVE id + affected version.
