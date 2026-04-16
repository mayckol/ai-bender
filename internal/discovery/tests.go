package discovery

// DetectTests infers the test framework, conventions, and coverage tool.
func DetectTests(p Probe) TestsInfo {
	switch {
	case p.Has("go.mod"):
		// Go uses the stdlib testing package and `_test.go` convention.
		return TestsInfo{Framework: "go test (stdlib)", Conventions: "*_test.go siblings", CoverageTool: "go test -cover"}
	case p.Has("package.json"):
		return detectNodeTests(p)
	case p.Has("pyproject.toml") || p.Has("requirements.txt"):
		return detectPythonTests(p)
	case p.Has("Cargo.toml"):
		return TestsInfo{Framework: "cargo test", Conventions: "#[cfg(test)] modules", CoverageTool: "tarpaulin"}
	case p.Has("Gemfile"):
		return TestsInfo{Framework: "rspec or minitest", Conventions: "spec/ or test/"}
	}
	return TestsInfo{}
}

func detectNodeTests(p Probe) TestsInfo {
	body, _ := p.Read("package.json")
	text := string(body)
	switch {
	case contains(text, "\"vitest\":"):
		return TestsInfo{Framework: "vitest", Conventions: "*.test.ts siblings", CoverageTool: "vitest --coverage"}
	case contains(text, "\"jest\":"):
		return TestsInfo{Framework: "jest", Conventions: "*.test.{ts,js} siblings", CoverageTool: "jest --coverage"}
	case contains(text, "\"mocha\":"):
		return TestsInfo{Framework: "mocha", Conventions: "test/ folder"}
	case contains(text, "\"playwright\":"):
		return TestsInfo{Framework: "playwright", Conventions: "tests/e2e/"}
	}
	return TestsInfo{Framework: "(unknown JS test framework)"}
}

func detectPythonTests(p Probe) TestsInfo {
	body, _ := p.Read("pyproject.toml")
	text := string(body)
	if contains(text, "pytest") {
		return TestsInfo{Framework: "pytest", Conventions: "tests/ folder, test_*.py", CoverageTool: "pytest-cov"}
	}
	body, _ = p.Read("requirements.txt")
	if contains(string(body), "pytest") {
		return TestsInfo{Framework: "pytest", Conventions: "tests/ folder, test_*.py", CoverageTool: "pytest-cov"}
	}
	return TestsInfo{Framework: "unittest", Conventions: "tests/ folder"}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (substr == "" || indexOf(s, substr) >= 0)
}

func indexOf(s, substr string) int {
	// Simple wrapper to avoid pulling strings here unnecessarily; small surface keeps imports minimal.
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
