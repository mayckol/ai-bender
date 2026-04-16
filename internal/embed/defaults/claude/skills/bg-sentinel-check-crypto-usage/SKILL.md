---
name: bg-sentinel-check-crypto-usage
context: bg
description: "Verify crypto usage: algorithms, key sizes, salts, IVs."
provides: [security, crypto]
stages: [ghu, implement]
applies_to: [any]
---

# bg-sentinel-check-crypto-usage

Flag deprecated algorithms (MD5/SHA1 for hashing, DES, ECB). Flag reused IVs and missing salts.
