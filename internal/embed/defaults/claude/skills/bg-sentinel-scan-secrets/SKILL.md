---
name: bg-sentinel-scan-secrets
context: bg
description: "Scan for committed secrets."
provides: [security, secrets]
stages: [ghu, implement]
applies_to: [any]
---

# bg-sentinel-scan-secrets

Look for API key patterns, private keys, hardcoded passwords. Severity ≥ high if exploitable; medium if test-only.
