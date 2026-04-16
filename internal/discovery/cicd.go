package discovery

// DetectCICD reports the CI provider and any deployment-target hints.
func DetectCICD(p Probe) CICDInfo {
	var c CICDInfo
	switch {
	case p.HasAnyUnder(".github/workflows"):
		c.Provider = "GitHub Actions"
	case p.Has(".gitlab-ci.yml"):
		c.Provider = "GitLab CI"
	case p.Has(".circleci/config.yml"):
		c.Provider = "CircleCI"
	case p.Has("azure-pipelines.yml"):
		c.Provider = "Azure Pipelines"
	case p.Has("Jenkinsfile"):
		c.Provider = "Jenkins"
	case p.Has(".drone.yml"):
		c.Provider = "Drone"
	case p.Has("bitbucket-pipelines.yml"):
		c.Provider = "Bitbucket Pipelines"
	}
	if p.Has("vercel.json") {
		c.DeploymentTargets = append(c.DeploymentTargets, "Vercel")
	}
	if p.Has("netlify.toml") {
		c.DeploymentTargets = append(c.DeploymentTargets, "Netlify")
	}
	if p.Has("fly.toml") {
		c.DeploymentTargets = append(c.DeploymentTargets, "Fly.io")
	}
	if p.Has("Procfile") {
		c.DeploymentTargets = append(c.DeploymentTargets, "Heroku/Procfile")
	}
	if p.HasAnyUnder("k8s") || p.HasAnyUnder("kubernetes") || p.HasGlob("*.yaml") && p.Has("Chart.yaml") {
		c.DeploymentTargets = append(c.DeploymentTargets, "Kubernetes/Helm")
	}
	return c
}
