package discovery

// Result is the aggregate output of all detectors. Sections without signal are zero values; the
// constitution renderer marks them `pending: true` per section.
type Result struct {
	Stack        StackInfo
	Structure    StructureInfo
	Tests        TestsInfo
	Lint         LintInfo
	Build        BuildInfo
	CICD         CICDInfo
	Dependencies []Dependency
	Pending      []string
}

// StackInfo describes the primary technology stack.
type StackInfo struct {
	Language       string
	PackageManager string
	Frameworks     []string
}

// IsZero reports whether no stack signal was found.
func (s StackInfo) IsZero() bool {
	return s.Language == "" && s.PackageManager == "" && len(s.Frameworks) == 0
}

// StructureInfo describes the project's high-level layout.
type StructureInfo struct {
	Folders     []string
	EntryPoints []string
}

func (s StructureInfo) IsZero() bool { return len(s.Folders) == 0 && len(s.EntryPoints) == 0 }

// TestsInfo describes the test framework and conventions.
type TestsInfo struct {
	Framework    string
	Conventions  string
	CoverageTool string
}

func (t TestsInfo) IsZero() bool { return t.Framework == "" && t.Conventions == "" && t.CoverageTool == "" }

// LintInfo describes the project's lint and formatting toolchain.
type LintInfo struct {
	Linters    []string
	Formatters []string
	PreCommit  bool
}

func (l LintInfo) IsZero() bool { return len(l.Linters) == 0 && len(l.Formatters) == 0 && !l.PreCommit }

// BuildInfo describes the project's build tool.
type BuildInfo struct {
	Tool       string
	HasMakefile bool
	HasDockerfile bool
}

func (b BuildInfo) IsZero() bool { return b.Tool == "" && !b.HasMakefile && !b.HasDockerfile }

// CICDInfo describes the project's CI provider and deployment targets.
type CICDInfo struct {
	Provider          string
	DeploymentTargets []string
}

func (c CICDInfo) IsZero() bool { return c.Provider == "" && len(c.DeploymentTargets) == 0 }

// Dependency is one direct dependency.
type Dependency struct {
	Name    string
	Version string
}
