package discovery

// DetectLint reports linters, formatters, and pre-commit presence.
func DetectLint(p Probe) LintInfo {
	var l LintInfo
	if p.Has(".golangci.yml") || p.Has(".golangci.yaml") {
		l.Linters = append(l.Linters, "golangci-lint")
	}
	if p.Has(".eslintrc") || p.Has(".eslintrc.js") || p.Has(".eslintrc.json") || p.Has("eslint.config.js") || p.Has("eslint.config.mjs") {
		l.Linters = append(l.Linters, "eslint")
	}
	if p.Has(".prettierrc") || p.Has(".prettierrc.json") || p.Has("prettier.config.js") {
		l.Formatters = append(l.Formatters, "prettier")
	}
	if p.Has(".rubocop.yml") {
		l.Linters = append(l.Linters, "rubocop")
	}
	if p.Has(".rustfmt.toml") || p.Has("rustfmt.toml") {
		l.Formatters = append(l.Formatters, "rustfmt")
	}
	if p.Has(".pylintrc") || p.Has("pyproject.toml") {
		// pyproject.toml may declare ruff/black/mypy; we cheaply check for those tokens.
		body, _ := p.Read("pyproject.toml")
		text := string(body)
		if contains(text, "ruff") {
			l.Linters = append(l.Linters, "ruff")
		}
		if contains(text, "black") {
			l.Formatters = append(l.Formatters, "black")
		}
		if contains(text, "mypy") {
			l.Linters = append(l.Linters, "mypy")
		}
	}
	if p.Has("go.mod") {
		l.Formatters = append(l.Formatters, "gofmt")
	}
	if p.Has(".pre-commit-config.yaml") {
		l.PreCommit = true
	}
	return l
}
