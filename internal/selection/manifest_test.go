package selection

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mayckol/ai-bender/internal/catalog"
)

func TestLoad_Absent_ReturnsNil(t *testing.T) {
	dir := t.TempDir()
	m, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if m != nil {
		t.Fatalf("want nil manifest when file absent, got %+v", m)
	}
}

func TestSaveLoadRoundtrip(t *testing.T) {
	dir := t.TempDir()
	in := map[string]bool{"benchmarker": false, "sentinel": true, "mistakeinator": true}
	if err := Save(dir, in); err != nil {
		t.Fatalf("Save: %v", err)
	}
	m, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if m == nil {
		t.Fatal("Load returned nil after Save")
	}
	if m.SchemaVersion != 1 {
		t.Errorf("schema_version = %d, want 1", m.SchemaVersion)
	}
	for id, want := range in {
		if got := m.Components[id].Selected; got != want {
			t.Errorf("%s selected = %v, want %v", id, got, want)
		}
	}
}

func TestValidate_UnknownID_Error(t *testing.T) {
	m := &Manifest{
		SchemaVersion: 1,
		Components:    map[string]ManifestEntry{"ghostagent": {Selected: true}},
	}
	cat := fakeCatalog()
	if err := m.Validate(cat); err == nil {
		t.Fatal("want error for unknown component, got nil")
	}
}

func TestValidate_MandatoryDeselected_Error(t *testing.T) {
	m := &Manifest{
		SchemaVersion: 1,
		Components:    map[string]ManifestEntry{"scout": {Selected: false}},
	}
	cat := fakeCatalog()
	if err := m.Validate(cat); err == nil {
		t.Fatal("want error for mandatory deselection, got nil")
	}
}

func TestResolve_Precedence_CatalogThenManifestThenFlags(t *testing.T) {
	cat := fakeCatalog()

	// Only catalog
	got, err := Resolve(cat, nil, Flags{})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !got["scout"] || !got["benchmarker"] {
		t.Errorf("catalog defaults: scout=%v benchmarker=%v, want both true", got["scout"], got["benchmarker"])
	}

	// Manifest overrides
	m := &Manifest{SchemaVersion: 1, Components: map[string]ManifestEntry{"benchmarker": {Selected: false}}}
	got, err = Resolve(cat, m, Flags{})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got["benchmarker"] {
		t.Errorf("manifest deselection not applied for benchmarker")
	}

	// Flag overrides manifest
	got, err = Resolve(cat, m, Flags{With: []string{"benchmarker"}})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !got["benchmarker"] {
		t.Errorf("--with did not re-enable benchmarker")
	}
}

func TestResolve_MandatoryDeselection_Rejected(t *testing.T) {
	cat := fakeCatalog()
	if _, err := Resolve(cat, nil, Flags{Without: []string{"scout"}}); err == nil {
		t.Fatal("want error when --without names a mandatory component")
	}
}

func TestResolve_UnknownComponent_Rejected(t *testing.T) {
	cat := fakeCatalog()
	if _, err := Resolve(cat, nil, Flags{Without: []string{"nonexistent"}}); err == nil {
		t.Fatal("want error when --without names an unknown component")
	}
	if _, err := Resolve(cat, nil, Flags{With: []string{"nonexistent"}}); err == nil {
		t.Fatal("want error when --with names an unknown component")
	}
}

func TestResolve_ContradictoryFlags_Rejected(t *testing.T) {
	cat := fakeCatalog()
	_, err := Resolve(cat, nil, Flags{With: []string{"benchmarker"}, Without: []string{"benchmarker"}})
	if err == nil {
		t.Fatal("want error for --with X --without X, got nil")
	}
}

func TestSave_WritesMapKeyedSchemaV1(t *testing.T) {
	dir := t.TempDir()
	if err := Save(dir, map[string]bool{"benchmarker": false}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, ".bender", "selection.yaml"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	s := string(data)
	if !contains(s, "schema_version: 1") {
		t.Errorf("missing schema_version in %q", s)
	}
	if !contains(s, "components:") {
		t.Errorf("missing components: in %q", s)
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (func() bool {
		for i := 0; i+len(needle) <= len(haystack); i++ {
			if haystack[i:i+len(needle)] == needle {
				return true
			}
		}
		return false
	})()
}

func fakeCatalog() *catalog.Catalog {
	tru := true
	return &catalog.Catalog{
		SchemaVersion: 1,
		Components: map[string]catalog.Component{
			"scout":         {Optional: false, Description: "scout"},
			"architect":     {Optional: false, Description: "architect"},
			"benchmarker":   {Optional: true, Default: &tru, Description: "benchmarker"},
			"sentinel":      {Optional: true, Default: &tru, Description: "sentinel"},
			"surgeon":       {Optional: true, Default: &tru, Description: "surgeon"},
			"mistakeinator": {Optional: true, Default: &tru, Description: "mistakeinator"},
		},
	}
}
