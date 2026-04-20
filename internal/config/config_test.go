package config

import (
	"os"
	"path/filepath"
	"testing"
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
