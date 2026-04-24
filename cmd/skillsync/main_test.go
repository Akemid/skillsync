package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Akemid/skillsync/internal/config"
)

func captureOutput(t *testing.T, fn func() error) (stdout string, stderr string, err error) {
	t.Helper()

	oldStdout := os.Stdout
	oldStderr := os.Stderr

	stdoutR, stdoutW, pipeErr := os.Pipe()
	if pipeErr != nil {
		t.Fatalf("creating stdout pipe: %v", pipeErr)
	}
	stderrR, stderrW, pipeErr := os.Pipe()
	if pipeErr != nil {
		t.Fatalf("creating stderr pipe: %v", pipeErr)
	}

	os.Stdout = stdoutW
	os.Stderr = stderrW

	err = fn()

	_ = stdoutW.Close()
	_ = stderrW.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	var outBuf bytes.Buffer
	var errBuf bytes.Buffer
	_, _ = outBuf.ReadFrom(stdoutR)
	_, _ = errBuf.ReadFrom(stderrR)

	_ = stdoutR.Close()
	_ = stderrR.Close()

	return outBuf.String(), errBuf.String(), err
}

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

func TestCmdInit_WarnsWhenConfigAlreadyExists(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "skillsync.yaml")
	if err := os.WriteFile(configPath, []byte("registry_path: ~/.agents/skills\n"), 0644); err != nil {
		t.Fatalf("seed config file: %v", err)
	}

	cfg := &config.Config{
		RegistryPath: "~/.agents/skills",
		Bundles:      config.DefaultBundles(),
		Tools:        config.DefaultTools(),
	}

	_, stderr, err := captureOutput(t, func() error {
		return cmdInit(cfg, configPath)
	})
	if err != nil {
		t.Fatalf("cmdInit() error = %v, want nil", err)
	}

	if !strings.Contains(stderr, "config already exists") {
		t.Fatalf("expected existing-config warning, got stderr: %q", stderr)
	}
	if !strings.Contains(stderr, "upgrade-config") {
		t.Fatalf("expected warning to suggest upgrade-config, got stderr: %q", stderr)
	}
}

func TestCmdInit_NoWarningWhenConfigMissing(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "skillsync.yaml")

	cfg := &config.Config{
		RegistryPath: "~/.agents/skills",
		Bundles:      config.DefaultBundles(),
		Tools:        config.DefaultTools(),
	}

	_, stderr, err := captureOutput(t, func() error {
		return cmdInit(cfg, configPath)
	})
	if err != nil {
		t.Fatalf("cmdInit() error = %v, want nil", err)
	}

	if strings.TrimSpace(stderr) != "" {
		t.Fatalf("expected no warning on missing config, got stderr: %q", stderr)
	}
}

func TestCmdUpgradeConfig_MissingConfig(t *testing.T) {
	missingPath := filepath.Join(t.TempDir(), "missing.yaml")

	_, _, err := captureOutput(t, func() error {
		return cmdUpgradeConfig(missingPath)
	})
	if err == nil {
		t.Fatal("cmdUpgradeConfig() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "no config found") {
		t.Fatalf("expected missing-config message, got: %v", err)
	}
}

func TestCmdUpgradeConfig_MigratesLegacyKiro(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "skillsync.yaml")
	configYAML := `registry_path: "~/.agents/skills-custom"
bundles:
  - name: "bundle-a"
    skills:
      - name: "my-skill"
tools:
  - name: "kiro"
    global_path: "~/.kiro/custom"
    local_path: ".kiro/custom"
    enabled: true
  - name: "my-custom-tool"
    global_path: "~/.custom/skills"
    local_path: ".custom/skills"
    enabled: true
`
	if err := os.WriteFile(configPath, []byte(configYAML), 0644); err != nil {
		t.Fatalf("write config fixture: %v", err)
	}

	stdout, _, err := captureOutput(t, func() error {
		return cmdUpgradeConfig(configPath)
	})
	if err != nil {
		t.Fatalf("cmdUpgradeConfig() error = %v, want nil", err)
	}

	if !strings.Contains(stdout, "migrated legacy tool: kiro") {
		t.Fatalf("expected migration summary in stdout, got: %q", stdout)
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("load upgraded config: %v", err)
	}
	if cfg.RegistryPath != "~/.agents/skills-custom" {
		t.Fatalf("registry_path changed unexpectedly: got %q", cfg.RegistryPath)
	}

	hasLegacy := false
	hasIDE := false
	hasCLI := false
	hasCustom := false
	for _, tool := range cfg.Tools {
		switch tool.Name {
		case "kiro":
			hasLegacy = true
		case "kiro-ide":
			hasIDE = true
		case "kiro-cli":
			hasCLI = true
		case "my-custom-tool":
			hasCustom = true
		}
	}

	if hasLegacy {
		t.Fatal("legacy kiro entry should be removed")
	}
	if !hasIDE || !hasCLI {
		t.Fatal("expected kiro-ide and kiro-cli entries after migration")
	}
	if !hasCustom {
		t.Fatal("expected custom tool to be preserved")
	}
}

func TestCmdUpgradeConfig_AlreadyCurrentShowsNoChanges(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "skillsync.yaml")
	configYAML := `registry_path: "~/.agents/skills"
tools:
  - name: "kiro-ide"
    global_path: "~/.kiro/skills"
    local_path: ".kiro/skills"
    enabled: true
    install_mode: "copy"
  - name: "kiro-cli"
    global_path: "~/.kiro/skills"
    local_path: ".kiro/skills"
    enabled: false
    install_mode: "symlink"
`
	if err := os.WriteFile(configPath, []byte(configYAML), 0644); err != nil {
		t.Fatalf("write config fixture: %v", err)
	}

	stdout, _, err := captureOutput(t, func() error {
		return cmdUpgradeConfig(configPath)
	})
	if err != nil {
		t.Fatalf("cmdUpgradeConfig() error = %v, want nil", err)
	}

	if !strings.Contains(stdout, "- no changes required") {
		t.Fatalf("expected no-changes summary line, got: %q", stdout)
	}
}

func TestCmdUpgradeConfig_IdempotentAcrossTwoRuns(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "skillsync.yaml")
	configYAML := `registry_path: "~/.agents/skills"
tools:
  - name: "kiro"
    global_path: "~/.kiro/custom"
    local_path: ".kiro/custom"
    enabled: true
`
	if err := os.WriteFile(configPath, []byte(configYAML), 0644); err != nil {
		t.Fatalf("write config fixture: %v", err)
	}

	firstStdout, _, err := captureOutput(t, func() error {
		return cmdUpgradeConfig(configPath)
	})
	if err != nil {
		t.Fatalf("first cmdUpgradeConfig() error = %v, want nil", err)
	}

	firstContent, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config after first run: %v", err)
	}

	secondStdout, _, err := captureOutput(t, func() error {
		return cmdUpgradeConfig(configPath)
	})
	if err != nil {
		t.Fatalf("second cmdUpgradeConfig() error = %v, want nil", err)
	}

	secondContent, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config after second run: %v", err)
	}

	if string(firstContent) != string(secondContent) {
		t.Fatal("config content changed across repeated upgrade-config runs")
	}
	if !strings.Contains(firstStdout, "migrated legacy tool: kiro") {
		t.Fatalf("expected migration on first run, got: %q", firstStdout)
	}
	if !strings.Contains(secondStdout, "- no changes required") {
		t.Fatalf("expected no-changes summary on second run, got: %q", secondStdout)
	}
}
