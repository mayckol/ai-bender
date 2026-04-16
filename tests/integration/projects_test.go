package integration_test

import (
	"strings"
	"testing"
)

// TestRegisterAndListProjects: T076 + T077.
func TestRegisterAndListProjects(t *testing.T) {
	bin := buildBenderOnce(t)
	registryHome := t.TempDir()

	root1 := mkProject(t)
	root2 := mkProject(t)

	out, err := runBenderEnv(t, bin, root1, []string{"XDG_CONFIG_HOME=" + registryHome}, "register-project", root1, "--name", "alpha")
	if err != nil {
		t.Fatalf("register alpha: %v\n%s", err, out)
	}
	out, err = runBenderEnv(t, bin, root2, []string{"XDG_CONFIG_HOME=" + registryHome}, "register-project", root2, "--name", "beta")
	if err != nil {
		t.Fatalf("register beta: %v\n%s", err, out)
	}

	out, err = runBenderEnv(t, bin, root1, []string{"XDG_CONFIG_HOME=" + registryHome}, "list-projects")
	if err != nil {
		t.Fatalf("list-projects: %v\n%s", err, out)
	}
	if !strings.Contains(out, "alpha") || !strings.Contains(out, "beta") {
		t.Fatalf("expected both projects in output:\n%s", out)
	}
	// alpha should be marked current (we ran from inside root1).
	if !strings.Contains(out, "alpha") || !strings.Contains(out, "current") {
		t.Fatalf("expected alpha to be marked current:\n%s", out)
	}
}

// TestRegisterProject_RejectsDuplicate: contributes to T076.
func TestRegisterProject_RejectsDuplicate(t *testing.T) {
	bin := buildBenderOnce(t)
	registryHome := t.TempDir()
	root := mkProject(t)
	if out, err := runBenderEnv(t, bin, root, []string{"XDG_CONFIG_HOME=" + registryHome}, "register-project", root, "--name", "dup"); err != nil {
		t.Fatalf("first register: %v\n%s", err, out)
	}
	out, err := runBenderEnv(t, bin, root, []string{"XDG_CONFIG_HOME=" + registryHome}, "register-project", root, "--name", "dup")
	if err == nil {
		t.Fatalf("expected duplicate-name error, got success:\n%s", out)
	}
	if !strings.Contains(out, "already registered") {
		t.Fatalf("expected duplicate error message in output:\n%s", out)
	}
}

// TestRegisterProject_AutoName: contributes to T076.
func TestRegisterProject_AutoName(t *testing.T) {
	bin := buildBenderOnce(t)
	registryHome := t.TempDir()
	root := mkNamedProject(t, "MyService_v2")
	out, err := runBenderEnv(t, bin, root, []string{"XDG_CONFIG_HOME=" + registryHome}, "register-project", root)
	if err != nil {
		t.Fatalf("register: %v\n%s", err, out)
	}
	if !strings.Contains(out, "myservice-v2") {
		t.Fatalf("expected auto-derived name myservice-v2 in output:\n%s", out)
	}
}
