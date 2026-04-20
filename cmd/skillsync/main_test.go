package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/Akemid/skillsync/internal/config"
)

// skipIfNoGit skips the test if git is not available in PATH
func skipIfNoGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not found in PATH")
	}
}

// mustRunIn runs a shell command in dir, failing the test on error
func mustRunIn(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s: %v\n%s", name, err, output)
	}
}

// TestCmdSync_NoRemoteBundles verifies cmdSync returns nil when no bundles have a source
func TestCmdSync_NoRemoteBundles(t *testing.T) {
	cfg := &config.Config{
		RegistryPath: t.TempDir(),
		Bundles: []config.Bundle{
			{Name: "local-only", Skills: []config.SkillRef{{Name: "my-skill"}}},
		},
	}

	if err := cmdSync(cfg); err != nil {
		t.Errorf("cmdSync() error = %v, want nil", err)
	}
}

// TestCmdSync_WithRemoteBundle syncs a bundle from a real local git repo
func TestCmdSync_WithRemoteBundle(t *testing.T) {
	skipIfNoGit(t)

	// Create source git repo with a skill
	sourceRepo := t.TempDir()
	mustRunIn(t, sourceRepo, "git", "init")
	mustRunIn(t, sourceRepo, "git", "symbolic-ref", "HEAD", "refs/heads/main")
	mustRunIn(t, sourceRepo, "git", "config", "user.email", "test@skillsync.test")
	mustRunIn(t, sourceRepo, "git", "config", "user.name", "SkillSync Test")

	skillDir := filepath.Join(sourceRepo, "sync-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("creating skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: sync-skill\ndescription: CLI integration test\n---\n# Sync Skill"), 0644); err != nil {
		t.Fatalf("writing SKILL.md: %v", err)
	}
	mustRunIn(t, sourceRepo, "git", "add", ".")
	mustRunIn(t, sourceRepo, "git", "commit", "-m", "init")

	registryPath := t.TempDir()
	cfg := &config.Config{
		RegistryPath: registryPath,
		Bundles: []config.Bundle{
			{
				Name: "remote-bundle",
				Source: &config.Source{
					Type:   "git",
					URL:    "file://" + sourceRepo,
					Branch: "main",
				},
			},
		},
	}

	if err := cmdSync(cfg); err != nil {
		t.Errorf("cmdSync() error = %v, want nil", err)
	}

	// Verify bundle was cloned under _remote/
	bundleDir := filepath.Join(registryPath, "_remote", "remote-bundle")
	if _, err := os.Stat(bundleDir); err != nil {
		t.Errorf("bundle dir not found after sync: %v", err)
	}

	// Verify skill is present
	if _, err := os.Stat(filepath.Join(bundleDir, "sync-skill", "SKILL.md")); err != nil {
		t.Errorf("sync-skill/SKILL.md not found in cloned bundle: %v", err)
	}
}
