package discovery

import (
	"strings"
)

// DetectStack inspects manifests in priority order; the first manifest that matches wins.
func DetectStack(p Probe) StackInfo {
	switch {
	case p.Has("go.mod"):
		return StackInfo{Language: "Go", PackageManager: "go modules"}
	case p.Has("package.json"):
		return detectNodeStack(p)
	case p.Has("pyproject.toml"):
		return detectPythonStack(p, "pyproject.toml")
	case p.Has("requirements.txt"):
		return StackInfo{Language: "Python", PackageManager: "pip"}
	case p.Has("Cargo.toml"):
		return StackInfo{Language: "Rust", PackageManager: "cargo"}
	case p.Has("pom.xml"):
		return StackInfo{Language: "Java", PackageManager: "maven"}
	case p.Has("build.gradle") || p.Has("build.gradle.kts"):
		return StackInfo{Language: "Java/Kotlin", PackageManager: "gradle"}
	case p.Has("Gemfile"):
		return StackInfo{Language: "Ruby", PackageManager: "bundler"}
	case p.Has("composer.json"):
		return StackInfo{Language: "PHP", PackageManager: "composer"}
	case p.Has("Package.swift"):
		return StackInfo{Language: "Swift", PackageManager: "swiftpm"}
	}
	return inferStackFromExtensions(p)
}

func detectNodeStack(p Probe) StackInfo {
	pm := "npm"
	switch {
	case p.Has("pnpm-lock.yaml"):
		pm = "pnpm"
	case p.Has("yarn.lock"):
		pm = "yarn"
	case p.Has("bun.lockb"):
		pm = "bun"
	}
	lang := "JavaScript"
	if p.HasGlob("tsconfig.json") || p.CountByExt(".ts") > 0 {
		lang = "TypeScript"
	}
	frameworks := detectNodeFrameworks(p)
	return StackInfo{Language: lang, PackageManager: pm, Frameworks: frameworks}
}

func detectNodeFrameworks(p Probe) []string {
	body, err := p.Read("package.json")
	if err != nil {
		return nil
	}
	text := string(body)
	var fws []string
	hints := map[string]string{
		"\"next\":":       "Next.js",
		"\"react\":":      "React",
		"\"vue\":":        "Vue",
		"\"@angular/":     "Angular",
		"\"svelte\":":     "Svelte",
		"\"express\":":    "Express",
		"\"fastify\":":    "Fastify",
		"\"@nestjs/":      "NestJS",
	}
	for needle, name := range hints {
		if strings.Contains(text, needle) {
			fws = append(fws, name)
		}
	}
	return fws
}

func detectPythonStack(p Probe, manifest string) StackInfo {
	pm := "pip"
	switch {
	case p.Has("poetry.lock"):
		pm = "poetry"
	case p.Has("uv.lock"):
		pm = "uv"
	case p.Has("Pipfile.lock"):
		pm = "pipenv"
	}
	frameworks := detectPythonFrameworks(p, manifest)
	return StackInfo{Language: "Python", PackageManager: pm, Frameworks: frameworks}
}

func detectPythonFrameworks(p Probe, manifest string) []string {
	body, err := p.Read(manifest)
	if err != nil {
		return nil
	}
	text := strings.ToLower(string(body))
	var fws []string
	for _, fw := range []string{"django", "fastapi", "flask", "starlette", "pyramid", "tornado"} {
		if strings.Contains(text, fw) {
			fws = append(fws, fw)
		}
	}
	return fws
}

func inferStackFromExtensions(p Probe) StackInfo {
	type pair struct {
		ext  string
		lang string
	}
	candidates := []pair{
		{".go", "Go"},
		{".rs", "Rust"},
		{".ts", "TypeScript"},
		{".js", "JavaScript"},
		{".py", "Python"},
		{".java", "Java"},
		{".rb", "Ruby"},
		{".php", "PHP"},
		{".swift", "Swift"},
		{".kt", "Kotlin"},
	}
	bestCount := 0
	best := ""
	for _, c := range candidates {
		if n := p.CountByExt(c.ext); n > bestCount {
			best = c.lang
			bestCount = n
		}
	}
	if best == "" {
		return StackInfo{}
	}
	return StackInfo{Language: best}
}
