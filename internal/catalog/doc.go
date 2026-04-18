// Package catalog loads and validates the Component Catalog — the declarative
// registry of every agent and skill the harness can scaffold, classifying each
// as mandatory or optional and declaring the embedded paths and pipeline nodes
// the component owns. The catalog ships inside the embedded defaults tree as
// `bender/catalog.yaml` and is the single source of truth consulted by the
// selection, render, and workspace packages.
package catalog
