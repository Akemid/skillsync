package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Akemid/skillsync/internal/archive"
	"github.com/Akemid/skillsync/internal/config"
	"github.com/Akemid/skillsync/internal/registry"
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

// ---------------------------------------------------------------------------
// W1 — cmdImport prints skill description after successful import
// ---------------------------------------------------------------------------

func TestCmdImport_PrintsDescription(t *testing.T) {
	// Build a real .tar.gz archive with a skill that has a description
	base := t.TempDir()
	skillDir := filepath.Join(base, "desc-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	skillMD := "---\nname: desc-skill\ndescription: Helps users find things.\n---\n# Desc Skill\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillMD), 0644); err != nil {
		t.Fatalf("WriteFile SKILL.md: %v", err)
	}
	archivePath := filepath.Join(t.TempDir(), "desc-skill.tar.gz")
	if err := archive.Export(skillDir, archivePath); err != nil {
		t.Fatalf("Export: %v", err)
	}

	registryPath := t.TempDir()
	cfg := &config.Config{RegistryPath: registryPath}
	configPath := filepath.Join(t.TempDir(), "config.yaml")

	oldArgs := os.Args
	os.Args = []string{"skillsync", "import", archivePath}
	defer func() { os.Args = oldArgs }()

	stdout, _, err := captureOutput(t, func() error {
		return cmdImport(cfg, configPath)
	})
	if err != nil {
		t.Fatalf("cmdImport() error = %v, want nil", err)
	}

	if !strings.Contains(stdout, `"desc-skill"`) {
		t.Errorf("stdout = %q, want to contain skill name", stdout)
	}
	if !strings.Contains(stdout, "Description: Helps users find things.") {
		t.Errorf("stdout = %q, want description line", stdout)
	}
}

func TestCmdImport_NoDescriptionField(t *testing.T) {
	// Skill with no description field — should still succeed, just no Description line
	base := t.TempDir()
	skillDir := filepath.Join(base, "nodesc-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: nodesc-skill\n---\n"), 0644); err != nil {
		t.Fatalf("WriteFile SKILL.md: %v", err)
	}
	archivePath := filepath.Join(t.TempDir(), "nodesc-skill.tar.gz")
	if err := archive.Export(skillDir, archivePath); err != nil {
		t.Fatalf("Export: %v", err)
	}

	registryPath := t.TempDir()
	cfg := &config.Config{RegistryPath: registryPath}
	configPath := filepath.Join(t.TempDir(), "config.yaml")

	oldArgs := os.Args
	os.Args = []string{"skillsync", "import", archivePath}
	defer func() { os.Args = oldArgs }()

	stdout, _, err := captureOutput(t, func() error {
		return cmdImport(cfg, configPath)
	})
	if err != nil {
		t.Fatalf("cmdImport() error = %v, want nil", err)
	}

	if strings.Contains(stdout, "Description:") {
		t.Errorf("stdout = %q, should not contain Description line when field absent", stdout)
	}
}

// ---------------------------------------------------------------------------
// W2 — Error-path coverage for cmdTapAdd, cmdTapRemove, cmdUpload, cmdExport,
// cmdImport (table-driven, CLI-level integration tests)
// ---------------------------------------------------------------------------

func TestCmdTapAdd_ErrorPaths(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing name and url",
			args:    []string{"skillsync", "tap", "add"},
			wantErr: "usage: skillsync tap add",
		},
		{
			name:    "missing url (only name provided)",
			args:    []string{"skillsync", "tap", "add", "my-tap"},
			wantErr: "usage: skillsync tap add",
		},
		{
			name:    "duplicate tap name",
			args:    []string{"skillsync", "tap", "add", "existing", "https://github.com/user/repo.git"},
			wantErr: "already registered",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Taps: []config.Tap{
					{Name: "existing", URL: "https://github.com/user/existing.git", Branch: "main"},
				},
			}
			configPath := filepath.Join(t.TempDir(), "config.yaml")

			oldArgs := os.Args
			os.Args = tt.args
			defer func() { os.Args = oldArgs }()

			_, _, err := captureOutput(t, func() error {
				return cmdTapAdd(cfg, configPath)
			})
			if err == nil {
				t.Fatalf("cmdTapAdd() error = nil, want error containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want to contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestCmdTapRemove_ErrorPaths(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing name arg",
			args:    []string{"skillsync", "tap", "remove"},
			wantErr: "usage: skillsync tap remove",
		},
		{
			name:    "nonexistent tap name",
			args:    []string{"skillsync", "tap", "remove", "ghost-tap"},
			wantErr: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Taps: []config.Tap{
					{Name: "real-tap", URL: "https://github.com/user/real.git", Branch: "main"},
				},
			}
			configPath := filepath.Join(t.TempDir(), "config.yaml")

			oldArgs := os.Args
			os.Args = tt.args
			defer func() { os.Args = oldArgs }()

			_, _, err := captureOutput(t, func() error {
				return cmdTapRemove(cfg, configPath)
			})
			if err == nil {
				t.Fatalf("cmdTapRemove() error = nil, want error containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want to contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestCmdUpload_ErrorPaths(t *testing.T) {
	// Build a minimal registry with one skill
	registryPath := t.TempDir()
	skillDir := filepath.Join(registryPath, "my-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: my-skill\n---\n"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	reg := registry.New(registryPath)
	if err := reg.Discover(); err != nil {
		t.Fatalf("Discover: %v", err)
	}

	tests := []struct {
		name    string
		args    []string
		taps    []config.Tap
		wantErr string
	}{
		{
			name:    "unregistered tap",
			args:    []string{"skillsync", "upload", "my-skill", "--to", "ghost-tap"},
			taps:    []config.Tap{},
			wantErr: "not found",
		},
		{
			name:    "skill not in registry",
			args:    []string{"skillsync", "upload", "ghost-skill", "--to", "my-tap"},
			taps:    []config.Tap{{Name: "my-tap", URL: "https://github.com/user/tap.git", Branch: "main"}},
			wantErr: "not found in registry",
		},
		{
			name:    "missing --to flag",
			args:    []string{"skillsync", "upload", "my-skill"},
			taps:    []config.Tap{{Name: "my-tap", URL: "https://github.com/user/tap.git", Branch: "main"}},
			wantErr: "--to",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				RegistryPath: registryPath,
				Taps:         tt.taps,
			}
			configPath := filepath.Join(t.TempDir(), "config.yaml")

			oldArgs := os.Args
			os.Args = tt.args
			defer func() { os.Args = oldArgs }()

			_, _, err := captureOutput(t, func() error {
				return cmdUpload(cfg, reg, configPath)
			})
			if err == nil {
				t.Fatalf("cmdUpload() error = nil, want error containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want to contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestCmdExport_ErrorPaths(t *testing.T) {
	registryPath := t.TempDir()
	skillDir := filepath.Join(registryPath, "real-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: real-skill\n---\n"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	reg := registry.New(registryPath)
	if err := reg.Discover(); err != nil {
		t.Fatalf("Discover: %v", err)
	}

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "nonexistent skill",
			args:    []string{"skillsync", "export", "ghost-skill"},
			wantErr: "not found in registry",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldArgs := os.Args
			os.Args = tt.args
			defer func() { os.Args = oldArgs }()

			_, _, err := captureOutput(t, func() error {
				return cmdExport(reg)
			})
			if err == nil {
				t.Fatalf("cmdExport() error = nil, want error containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want to contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestCmdImport_ErrorPaths(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "nonexistent archive file",
			args:    []string{"skillsync", "import", "/tmp/skillsync-test-nonexistent-archive.tar.gz"},
			wantErr: "import failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{RegistryPath: t.TempDir()}
			configPath := filepath.Join(t.TempDir(), "config.yaml")

			oldArgs := os.Args
			os.Args = tt.args
			defer func() { os.Args = oldArgs }()

			_, _, err := captureOutput(t, func() error {
				return cmdImport(cfg, configPath)
			})
			if err == nil {
				t.Fatalf("cmdImport() error = nil, want error containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want to contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}
