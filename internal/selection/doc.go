// Package selection reconciles the user's component choices against the
// catalog. It loads and saves the Selection Manifest at `.bender/selection.yaml`,
// applies `--with` / `--without` CLI flags on top of catalog defaults, and
// rejects contradictions and mandatory-component deselection before any file
// is written.
package selection
