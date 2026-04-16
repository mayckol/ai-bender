package discovery

import (
	"bufio"
	"bytes"
	"encoding/json"
	"strings"
)

// DetectDependencies extracts a project's direct dependencies from the most informative manifest available.
// We deliberately avoid network lookups; "outdated" is left to a future enhancement.
func DetectDependencies(p Probe) []Dependency {
	switch {
	case p.Has("go.mod"):
		return parseGoMod(p)
	case p.Has("package.json"):
		return parsePackageJSON(p)
	case p.Has("Cargo.toml"):
		return parseCargoToml(p)
	case p.Has("pyproject.toml"):
		return parsePyProjectToml(p)
	case p.Has("requirements.txt"):
		return parseRequirementsTxt(p)
	}
	return nil
}

func parseGoMod(p Probe) []Dependency {
	body, err := p.Read("go.mod")
	if err != nil {
		return nil
	}
	var out []Dependency
	scanner := bufio.NewScanner(bytes.NewReader(body))
	inRequireBlock := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		if strings.HasPrefix(line, "require (") {
			inRequireBlock = true
			continue
		}
		if inRequireBlock && line == ")" {
			inRequireBlock = false
			continue
		}
		var rest string
		switch {
		case inRequireBlock:
			rest = line
		case strings.HasPrefix(line, "require "):
			rest = strings.TrimPrefix(line, "require ")
		default:
			continue
		}
		if strings.Contains(rest, "// indirect") {
			continue
		}
		fields := strings.Fields(rest)
		if len(fields) >= 2 {
			out = append(out, Dependency{Name: fields[0], Version: fields[1]})
		}
	}
	return out
}

func parsePackageJSON(p Probe) []Dependency {
	body, err := p.Read("package.json")
	if err != nil {
		return nil
	}
	var pkg struct {
		Dependencies map[string]string `json:"dependencies"`
	}
	if err := json.Unmarshal(body, &pkg); err != nil {
		return nil
	}
	var out []Dependency
	for name, version := range pkg.Dependencies {
		out = append(out, Dependency{Name: name, Version: version})
	}
	return out
}

func parseCargoToml(p Probe) []Dependency {
	body, err := p.Read("Cargo.toml")
	if err != nil {
		return nil
	}
	return scanTomlSection(string(body), "[dependencies]")
}

func parsePyProjectToml(p Probe) []Dependency {
	body, err := p.Read("pyproject.toml")
	if err != nil {
		return nil
	}
	return scanTomlSection(string(body), "[project.dependencies]")
}

func scanTomlSection(text, header string) []Dependency {
	idx := strings.Index(text, header)
	if idx < 0 {
		return nil
	}
	rest := text[idx+len(header):]
	end := strings.Index(rest, "\n[")
	if end >= 0 {
		rest = rest[:end]
	}
	var out []Dependency
	for _, line := range strings.Split(rest, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		eq := strings.Index(line, "=")
		if eq < 0 {
			continue
		}
		name := strings.TrimSpace(line[:eq])
		version := strings.TrimSpace(strings.Trim(line[eq+1:], " \""))
		if name == "" {
			continue
		}
		out = append(out, Dependency{Name: name, Version: version})
	}
	return out
}

func parseRequirementsTxt(p Probe) []Dependency {
	body, err := p.Read("requirements.txt")
	if err != nil {
		return nil
	}
	var out []Dependency
	scanner := bufio.NewScanner(bytes.NewReader(body))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		name, version := splitRequirement(line)
		out = append(out, Dependency{Name: name, Version: version})
	}
	return out
}

func splitRequirement(line string) (string, string) {
	for _, op := range []string{"==", ">=", "<=", "~=", ">", "<"} {
		if i := strings.Index(line, op); i >= 0 {
			return strings.TrimSpace(line[:i]), strings.TrimSpace(line[i:])
		}
	}
	return line, ""
}
