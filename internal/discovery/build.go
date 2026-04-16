package discovery

// DetectBuild reports the project's build tool plus Makefile/Dockerfile presence.
func DetectBuild(p Probe) BuildInfo {
	var b BuildInfo
	switch {
	case p.Has("go.mod"):
		b.Tool = "go build"
	case p.Has("package.json"):
		b.Tool = "npm/yarn/pnpm scripts"
	case p.Has("Cargo.toml"):
		b.Tool = "cargo"
	case p.Has("pyproject.toml"):
		b.Tool = "build/poetry/uv"
	case p.Has("pom.xml"):
		b.Tool = "maven"
	case p.Has("build.gradle") || p.Has("build.gradle.kts"):
		b.Tool = "gradle"
	case p.Has("Gemfile"):
		b.Tool = "bundler"
	}
	b.HasMakefile = p.Has("Makefile") || p.Has("makefile")
	b.HasDockerfile = p.Has("Dockerfile") || p.HasGlob("Dockerfile.*")
	return b
}
