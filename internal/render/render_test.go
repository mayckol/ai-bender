package render

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mayckol/ai-bender/internal/catalog"
)

func TestSkill_Verbatim_NoConditionals(t *testing.T) {
	cat := fakeCat()
	src := []byte("hello world")
	got, err := Skill(src, BuildCtx(cat, map[string]bool{}), cat)
	if err != nil {
		t.Fatalf("Skill: %v", err)
	}
	if !bytes.Equal(got, src) {
		t.Errorf("got %q, want %q", got, src)
	}
}

func TestSkill_SelectedBranchesCorrectly(t *testing.T) {
	cat := fakeCat()
	src := []byte(`{{ if selected "benchmarker" }}BENCH-YES{{ else }}BENCH-NO{{ end }}`)

	got, err := Skill(src, BuildCtx(cat, map[string]bool{"benchmarker": true}), cat)
	if err != nil {
		t.Fatalf("Skill(bench selected): %v", err)
	}
	if string(got) != "BENCH-YES" {
		t.Errorf("selected branch: got %q", got)
	}

	got, err = Skill(src, BuildCtx(cat, map[string]bool{"benchmarker": false}), cat)
	if err != nil {
		t.Fatalf("Skill(bench deselected): %v", err)
	}
	if string(got) != "BENCH-NO" {
		t.Errorf("deselected branch: got %q", got)
	}
}

func TestSkill_UnknownID_FailsAtLoad(t *testing.T) {
	cat := fakeCat()
	src := []byte(`{{ if selected "ghost" }}X{{ end }}`)
	_, err := Skill(src, BuildCtx(cat, map[string]bool{}), cat)
	if err == nil {
		t.Fatal("want error for unknown id, got nil")
	}
	if !strings.Contains(err.Error(), `"ghost"`) {
		t.Errorf("error should name the unknown id: %v", err)
	}
}

func TestSkill_Determinism(t *testing.T) {
	cat := fakeCat()
	src := []byte(`{{ if selected "benchmarker" }}YES{{ end }}-{{ description "sentinel" }}`)
	ctx := BuildCtx(cat, map[string]bool{"benchmarker": true, "sentinel": true})
	first, err := Skill(src, ctx, cat)
	if err != nil {
		t.Fatalf("Skill: %v", err)
	}
	for i := 0; i < 50; i++ {
		got, err := Skill(src, ctx, cat)
		if err != nil {
			t.Fatalf("Skill iter %d: %v", i, err)
		}
		if !bytes.Equal(got, first) {
			t.Fatalf("non-deterministic output at iter %d: %q vs %q", i, got, first)
		}
	}
}

func TestFingerprint_StableAcrossRuns(t *testing.T) {
	a := Fingerprint([]byte("hello"))
	b := Fingerprint([]byte("hello"))
	if a != b {
		t.Errorf("fingerprint unstable: %s vs %s", a, b)
	}
	if Fingerprint([]byte("hello ")) == a {
		t.Errorf("whitespace change should alter fingerprint")
	}
}

func TestFingerprint_Sidecar_Roundtrip(t *testing.T) {
	dir := t.TempDir()
	if err := WriteFingerprint(dir, "deadbeef"); err != nil {
		t.Fatalf("WriteFingerprint: %v", err)
	}
	got, err := ReadFingerprint(dir)
	if err != nil {
		t.Fatalf("ReadFingerprint: %v", err)
	}
	if got != "deadbeef" {
		t.Errorf("got %q, want deadbeef", got)
	}
}

func TestIsDrifted_CorrectSidecar_NotDrifted(t *testing.T) {
	dir := t.TempDir()
	pipelinePath := filepath.Join(dir, ".bender", "pipeline.yaml")
	if err := os.MkdirAll(filepath.Dir(pipelinePath), 0o755); err != nil {
		t.Fatal(err)
	}
	content := []byte("nodes:\n  - id: scout\n")
	if err := os.WriteFile(pipelinePath, content, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteFingerprint(dir, Fingerprint(content)); err != nil {
		t.Fatal(err)
	}
	drifted, err := IsDrifted(dir, pipelinePath)
	if err != nil {
		t.Fatalf("IsDrifted: %v", err)
	}
	if drifted {
		t.Error("freshly-written file should not be drifted")
	}
}

func TestIsDrifted_UserEdited_Drifted(t *testing.T) {
	dir := t.TempDir()
	pipelinePath := filepath.Join(dir, ".bender", "pipeline.yaml")
	if err := os.MkdirAll(filepath.Dir(pipelinePath), 0o755); err != nil {
		t.Fatal(err)
	}
	original := []byte("nodes:\n  - id: scout\n")
	if err := WriteFingerprint(dir, Fingerprint(original)); err != nil {
		t.Fatal(err)
	}
	userEdited := append(original, '\n', '#', ' ', 'e', 'd', 'i', 't', '\n')
	if err := os.WriteFile(pipelinePath, userEdited, 0o644); err != nil {
		t.Fatal(err)
	}
	drifted, err := IsDrifted(dir, pipelinePath)
	if err != nil {
		t.Fatalf("IsDrifted: %v", err)
	}
	if !drifted {
		t.Error("edited file should be marked drifted")
	}
}

func TestPipeline_DropsDeselectedNodes_AndDanglingDeps(t *testing.T) {
	cat := &catalog.Catalog{
		SchemaVersion: 1,
		Components: map[string]catalog.Component{
			"scout":       {Optional: false, Description: "scout", Paths: catalog.Paths{PipelineNodes: []string{"scout"}}},
			"benchmarker": {Optional: true, Description: "bench", Paths: catalog.Paths{PipelineNodes: []string{"benchmarker"}}},
		},
	}
	src := []byte(`
nodes:
  - id: scout
    agent: scout
  - id: benchmarker
    agent: benchmarker
  - id: scribe
    agent: scribe
    depends_on: [scout, benchmarker]
`)
	out, dropped, err := Pipeline(src, cat, map[string]bool{"scout": true, "benchmarker": false})
	if err != nil {
		t.Fatalf("Pipeline: %v", err)
	}
	if len(dropped) != 1 || dropped[0] != "benchmarker" {
		t.Errorf("dropped = %v, want [benchmarker]", dropped)
	}
	if bytes.Contains(out, []byte("id: benchmarker")) {
		t.Errorf("benchmarker node still present: %s", out)
	}
	if !bytes.Contains(out, []byte("scout")) {
		t.Errorf("scout missing from output: %s", out)
	}
	// depends_on should have lost the benchmarker reference
	if bytes.Contains(out, []byte("benchmarker")) {
		t.Errorf("dangling benchmarker reference in depends_on: %s", out)
	}
}

func TestPipeline_FullSelection_PreservesEverything(t *testing.T) {
	cat := &catalog.Catalog{
		SchemaVersion: 1,
		Components: map[string]catalog.Component{
			"scout":       {Optional: false, Description: "scout", Paths: catalog.Paths{PipelineNodes: []string{"scout"}}},
			"benchmarker": {Optional: true, Description: "bench", Paths: catalog.Paths{PipelineNodes: []string{"benchmarker"}}},
		},
	}
	src := []byte(`
nodes:
  - id: scout
    agent: scout
  - id: benchmarker
    agent: benchmarker
`)
	out, dropped, err := Pipeline(src, cat, map[string]bool{"scout": true, "benchmarker": true})
	if err != nil {
		t.Fatalf("Pipeline: %v", err)
	}
	if len(dropped) != 0 {
		t.Errorf("dropped = %v, want []", dropped)
	}
	if !bytes.Contains(out, []byte("benchmarker")) {
		t.Errorf("benchmarker unexpectedly pruned: %s", out)
	}
}

func fakeCat() *catalog.Catalog {
	return &catalog.Catalog{
		SchemaVersion: 1,
		Components: map[string]catalog.Component{
			"scout":         {Optional: false, Description: "scout"},
			"benchmarker":   {Optional: true, Description: "bench"},
			"sentinel":      {Optional: true, Description: "sentinel"},
			"mistakeinator": {Optional: true, Description: "mistakeinator"},
		},
	}
}
