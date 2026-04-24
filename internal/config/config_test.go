package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestConfigLoad_WithSource verifies loading bundle with Git source
func TestConfigLoad_WithSource(t *testing.T) {
	// Create temp directory for test fixtures
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	// Write test config with source field
	configYAML := `registry_path: "~/.agents/skills"
bundles:
  - name: "test-bundle"
    source:
      type: "git"
      url: "https://github.com/test/skills"
      branch: "main"
    skills:
      - name: "skill-a"
`
	if err := os.WriteFile(configPath, []byte(configYAML), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	// Load config
	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}

	// Verify bundle loaded
	if len(cfg.Bundles) != 1 {
		t.Fatalf("got %d bundles, want 1", len(cfg.Bundles))
	}

	bundle := cfg.Bundles[0]

	// Verify source is populated
	if bundle.Source == nil {
		t.Fatal("bundle.Source is nil, want non-nil")
	}

	if bundle.Source.Type != "git" {
		t.Errorf("bundle.Source.Type = %q, want \"git\"", bundle.Source.Type)
	}

	if bundle.Source.URL != "https://github.com/test/skills" {
		t.Errorf("bundle.Source.URL = %q, want \"https://github.com/test/skills\"", bundle.Source.URL)
	}

	if bundle.Source.Branch != "main" {
		t.Errorf("bundle.Source.Branch = %q, want \"main\"", bundle.Source.Branch)
	}
}

// TestConfigLoad_WithoutSource verifies loading bundle without source (local-only)
func TestConfigLoad_WithoutSource(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	// Write test config WITHOUT source field
	configYAML := `registry_path: "~/.agents/skills"
bundles:
  - name: "local-bundle"
    skills:
      - name: "local-skill"
`
	if err := os.WriteFile(configPath, []byte(configYAML), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}

	if len(cfg.Bundles) != 1 {
		t.Fatalf("got %d bundles, want 1", len(cfg.Bundles))
	}

	bundle := cfg.Bundles[0]

	// Verify source is nil for local-only bundle
	if bundle.Source != nil {
		t.Errorf("bundle.Source = %+v, want nil (local-only bundle)", bundle.Source)
	}
}

// TestValidateSource_InvalidType verifies rejection of non-git source types
func TestValidateSource_InvalidType(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	// Write test config with invalid source type
	configYAML := `registry_path: "~/.agents/skills"
bundles:
  - name: "invalid-bundle"
    source:
      type: "svn"
      url: "https://svn.example.com/repo"
    skills:
      - name: "skill-a"
`
	if err := os.WriteFile(configPath, []byte(configYAML), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	// Load should fail with validation error
	_, err := Load(configPath)
	if err == nil {
		t.Fatal("Load() error = nil, want error for invalid source type")
	}

	// Check error message contains expected text
	expectedMsg := "unsupported source type"
	if !contains(err.Error(), expectedMsg) {
		t.Errorf("error message = %q, want to contain %q", err.Error(), expectedMsg)
	}
}

// TestValidateSource_MissingURL verifies rejection of source without URL
func TestValidateSource_MissingURL(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	configYAML := `registry_path: "~/.agents/skills"
bundles:
  - name: "no-url-bundle"
    source:
      type: "git"
    skills:
      - name: "skill-a"
`
	if err := os.WriteFile(configPath, []byte(configYAML), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	_, err := Load(configPath)
	if err == nil {
		t.Fatal("Load() error = nil, want error for missing URL")
	}

	expectedMsg := "source.url is required"
	if !contains(err.Error(), expectedMsg) {
		t.Errorf("error message = %q, want to contain %q", err.Error(), expectedMsg)
	}
}

// TestValidateSource_InvalidURL verifies rejection of malformed Git URLs
func TestValidateSource_InvalidURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{"http instead of https", "http://github.com/test/repo", "invalid Git URL"},
		{"ftp protocol", "ftp://example.com/repo", "invalid Git URL"},
		{"no protocol", "github.com/test/repo", "invalid Git URL"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			configPath := filepath.Join(tempDir, "config.yaml")

			configYAML := `registry_path: "~/.agents/skills"
bundles:
  - name: "invalid-url-bundle"
    source:
      type: "git"
      url: "` + tt.url + `"
    skills:
      - name: "skill-a"
`
			if err := os.WriteFile(configPath, []byte(configYAML), 0644); err != nil {
				t.Fatalf("failed to write test config: %v", err)
			}

			_, err := Load(configPath)
			if err == nil {
				t.Fatal("Load() error = nil, want error for invalid URL")
			}

			if !contains(err.Error(), tt.want) {
				t.Errorf("error message = %q, want to contain %q", err.Error(), tt.want)
			}
		})
	}
}

func TestToolIsCopyMode(t *testing.T) {
	tests := []struct {
		name string
		mode string
		want bool
	}{
		{name: "copy mode", mode: "copy", want: true},
		{name: "empty mode defaults to symlink", mode: "", want: false},
		{name: "explicit symlink mode", mode: "symlink", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool := Tool{InstallMode: tt.mode}
			if got := tool.IsCopyMode(); got != tt.want {
				t.Fatalf("Tool.IsCopyMode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDefaultTools_KiroEntries(t *testing.T) {
	tools := DefaultTools()

	var hasKiroIDE bool
	var hasKiroCLI bool
	var hasLegacyKiro bool

	for _, tool := range tools {
		switch tool.Name {
		case "kiro-ide":
			hasKiroIDE = true
			if tool.InstallMode != "copy" {
				t.Fatalf("kiro-ide InstallMode = %q, want %q", tool.InstallMode, "copy")
			}
			if !tool.Enabled {
				t.Fatalf("kiro-ide Enabled = %v, want true", tool.Enabled)
			}
		case "kiro-cli":
			hasKiroCLI = true
			if tool.InstallMode == "copy" {
				t.Fatalf("kiro-cli InstallMode = %q, want non-copy mode", tool.InstallMode)
			}
		case "kiro":
			hasLegacyKiro = true
		}
	}

	if !hasKiroIDE {
		t.Fatal("DefaultTools() missing kiro-ide entry")
	}
	if !hasKiroCLI {
		t.Fatal("DefaultTools() missing kiro-cli entry")
	}
	if hasLegacyKiro {
		t.Fatal("DefaultTools() should not include legacy kiro entry")
	}
}

func TestLoad_InstallModeRoundTrip(t *testing.T) {
	t.Run("yaml with install_mode copy persists", func(t *testing.T) {
		tempDir := t.TempDir()
		configPath := filepath.Join(tempDir, "config-copy.yaml")

		configYAML := `registry_path: "~/.agents/skills"
tools:
  - name: "kiro-ide"
    global_path: "~/.kiro/skills"
    local_path: ".kiro/skills"
    enabled: true
    install_mode: "copy"
`
		if err := os.WriteFile(configPath, []byte(configYAML), 0644); err != nil {
			t.Fatalf("failed to write test config: %v", err)
		}

		cfg, err := Load(configPath)
		if err != nil {
			t.Fatalf("Load() error = %v, want nil", err)
		}
		if len(cfg.Tools) != 1 {
			t.Fatalf("len(cfg.Tools) = %d, want 1", len(cfg.Tools))
		}
		if cfg.Tools[0].InstallMode != "copy" {
			t.Fatalf("InstallMode = %q, want %q", cfg.Tools[0].InstallMode, "copy")
		}
	})

	t.Run("yaml without install_mode keeps empty value", func(t *testing.T) {
		tempDir := t.TempDir()
		configPath := filepath.Join(tempDir, "config-empty.yaml")

		configYAML := `registry_path: "~/.agents/skills"
tools:
  - name: "kiro"
    global_path: "~/.kiro/skills"
    local_path: ".kiro/skills"
    enabled: true
`
		if err := os.WriteFile(configPath, []byte(configYAML), 0644); err != nil {
			t.Fatalf("failed to write test config: %v", err)
		}

		cfg, err := Load(configPath)
		if err != nil {
			t.Fatalf("Load() error = %v, want nil", err)
		}
		if len(cfg.Tools) != 1 {
			t.Fatalf("len(cfg.Tools) = %d, want 1", len(cfg.Tools))
		}
		if cfg.Tools[0].InstallMode != "" {
			t.Fatalf("InstallMode = %q, want empty string", cfg.Tools[0].InstallMode)
		}
	})
}

func TestMigrateTools_LegacyKiroMigratesWithInheritedPaths(t *testing.T) {
	existing := []Tool{
		{Name: "claude", GlobalPath: "~/.claude/skills", LocalPath: ".claude/skills", Enabled: true},
		{Name: "kiro", GlobalPath: "~/.kiro/custom", LocalPath: ".kiro/custom", Enabled: true},
	}

	migrated, summary := MigrateTools(existing)

	if !summary.MigratedLegacy {
		t.Fatal("expected summary.MigratedLegacy to be true")
	}
	if !summary.Changed || summary.Unchanged {
		t.Fatal("expected changed migration summary")
	}

	var hasLegacy bool
	var hasIDE bool
	var hasCLI bool

	for _, tool := range migrated {
		switch tool.Name {
		case "kiro":
			hasLegacy = true
		case "kiro-ide":
			hasIDE = true
			if tool.GlobalPath != "~/.kiro/custom" || tool.LocalPath != ".kiro/custom" {
				t.Fatalf("kiro-ide paths not inherited: got (%s, %s)", tool.GlobalPath, tool.LocalPath)
			}
			if tool.InstallMode != "copy" || !tool.Enabled {
				t.Fatalf("kiro-ide shape invalid: mode=%q enabled=%v", tool.InstallMode, tool.Enabled)
			}
		case "kiro-cli":
			hasCLI = true
			if tool.GlobalPath != "~/.kiro/custom" || tool.LocalPath != ".kiro/custom" {
				t.Fatalf("kiro-cli paths not inherited: got (%s, %s)", tool.GlobalPath, tool.LocalPath)
			}
			if tool.InstallMode != "symlink" || tool.Enabled {
				t.Fatalf("kiro-cli shape invalid: mode=%q enabled=%v", tool.InstallMode, tool.Enabled)
			}
		}
	}

	if hasLegacy {
		t.Fatal("legacy kiro should be removed after migration")
	}
	if !hasIDE || !hasCLI {
		t.Fatal("expected both kiro-ide and kiro-cli after migration")
	}
}

func TestMigrateTools_PreservesCustomAndNoDuplicateSplitEntries(t *testing.T) {
	existing := []Tool{
		{Name: "custom-tool", GlobalPath: "~/.custom/skills", LocalPath: ".custom/skills", Enabled: true},
		{Name: "kiro", GlobalPath: "~/.kiro/skills", LocalPath: ".kiro/skills", Enabled: true},
		{Name: "kiro-ide", GlobalPath: "~/.kiro/skills", LocalPath: ".kiro/skills", Enabled: true, InstallMode: "copy"},
		{Name: "kiro-cli", GlobalPath: "~/.kiro/skills", LocalPath: ".kiro/skills", Enabled: false, InstallMode: "symlink"},
	}

	migrated, summary := MigrateTools(existing)

	if !summary.MigratedLegacy {
		t.Fatal("expected legacy migration flag")
	}
	if len(summary.AddedTools) != 0 {
		t.Fatalf("expected no added tools when split entries already exist, got %v", summary.AddedTools)
	}

	var customCount, ideCount, cliCount int
	for _, tool := range migrated {
		switch tool.Name {
		case "custom-tool":
			customCount++
		case "kiro-ide":
			ideCount++
		case "kiro-cli":
			cliCount++
		}
	}

	if customCount != 1 || ideCount != 1 || cliCount != 1 {
		t.Fatalf("unexpected tool counts custom=%d ide=%d cli=%d", customCount, ideCount, cliCount)
	}
}

func TestMigrateTools_Idempotent(t *testing.T) {
	existing := []Tool{
		{Name: "claude", GlobalPath: "~/.claude/skills", LocalPath: ".claude/skills", Enabled: true},
		{Name: "kiro", GlobalPath: "~/.kiro/skills", LocalPath: ".kiro/skills", Enabled: true},
	}

	first, firstSummary := MigrateTools(existing)
	second, secondSummary := MigrateTools(first)

	if !firstSummary.Changed {
		t.Fatal("first migration should report changes")
	}
	if !secondSummary.Unchanged {
		t.Fatal("second migration should be unchanged")
	}

	firstYAML, err := yaml.Marshal(first)
	if err != nil {
		t.Fatalf("yaml marshal first: %v", err)
	}
	secondYAML, err := yaml.Marshal(second)
	if err != nil {
		t.Fatalf("yaml marshal second: %v", err)
	}
	if string(firstYAML) != string(secondYAML) {
		t.Fatal("migrated tools are not byte-equivalent across repeated runs")
	}
}

func TestMigrateTools_DoesNotMutateInput(t *testing.T) {
	existing := []Tool{
		{Name: "kiro", GlobalPath: "~/.kiro/skills", LocalPath: ".kiro/skills", Enabled: true},
	}
	before := append([]Tool(nil), existing...)

	_, _ = MigrateTools(existing)

	if !reflect.DeepEqual(existing, before) {
		t.Fatal("MigrateTools should not mutate input slice")
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
