---
name: bg-sentinel-static-scan
context: bg
description: "Scan for committed secrets, vulnerable dependencies, and weak crypto usage. Calibrate severity to exploitability."
provides: [security, secrets, cve, crypto]
stages: [ghu, implement]
applies_to: [any]
---

# bg-sentinel-static-scan

Three static checks in one skill: secrets in source, known-CVE dependencies, weak/misused crypto. **Severity must reflect exploitability, not theoretical risk.**

## 1. Secret scan

Look for hardcoded credentials in the change set:
- API key patterns: `AKIA[0-9A-Z]{16}`, `sk_live_…`, `gh[ps]_[A-Za-z0-9]{36,}`, `xox[baprs]-[A-Za-z0-9-]+`.
- Private keys: `-----BEGIN (RSA|EC|OPENSSH|PGP) PRIVATE KEY-----`.
- Hardcoded passwords: literal strings assigned to variables named `password`, `passwd`, `secret`, `token`.

Severity:
- Public repo + real-looking key → `critical`.
- Internal repo + real-looking key → `high`.
- Test fixture with obviously-fake placeholder → `info` or omit.

## 2. Dependency CVE check

Cross-reference the project's dependency manifest against known CVEs (best-effort, offline if a database is bundled). Emit one finding per affected dependency with the CVE id, affected versions, and the fixed version.

## 3. Crypto usage

Flag deprecated or misused crypto in the change set:
- Hashing: MD5, SHA-1 (for security purposes — checksums are fine).
- Symmetric: DES, ECB mode, hardcoded IVs, IV reuse with the same key.
- Random: `math/rand` for security tokens; should be `crypto/rand`.
- TLS: minimum version below 1.2; cipher suites with RC4 or 3DES.

## Output

Each finding goes to `.bender/artifacts/ghu/security/<run-timestamp>/<finding-id>.md` with a one-paragraph attack-vector explanation. Emit `finding_reported` events for the run report to aggregate.

## What you DO NOT do

- False-positive flooding. If you can't confidently describe an attack vector, set severity `info` and explain why.
- Modify code. Findings only.
