// Package render materialises the embedded defaults against a resolved
// selection. It renders skill templates (text/template with AST-level
// identifier validation against the catalog), prunes pipeline nodes whose
// agents have been deselected, and computes fingerprints used by the
// drift-detection path in `internal/workspace`.
package render
