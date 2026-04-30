package skillasset

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// TestContent_NotEmpty verifies the embedded SKILL.md is non-empty and starts with frontmatter.
func TestContent_NotEmpty(t *testing.T) {
	c := Content()
	if len(c) == 0 {
		t.Fatal("Content() returned empty slice")
	}
	if !bytes.HasPrefix(c, []byte("---\n")) {
		t.Errorf("Content() does not start with frontmatter delimiter '---\\n'")
	}
}

// TestExtractTo covers happy path, idempotent overwrite, stale overwrite, and missing destDir.
func TestExtractTo(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T) string
		wantErr bool
	}{
		{
			name: "happy path creates SKILL.md",
			setup: func(t *testing.T) string {
				return t.TempDir()
			},
		},
		{
			name: "idempotent overwrites existing with same content",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				skillDir := filepath.Join(dir, "skillsync")
				if err := os.MkdirAll(skillDir, 0755); err != nil {
					t.Fatalf("MkdirAll: %v", err)
				}
				if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), Content(), 0644); err != nil {
					t.Fatalf("WriteFile: %v", err)
				}
				return dir
			},
		},
		{
			name: "overwrites stale content",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				skillDir := filepath.Join(dir, "skillsync")
				if err := os.MkdirAll(skillDir, 0755); err != nil {
					t.Fatalf("MkdirAll: %v", err)
				}
				if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("old content"), 0644); err != nil {
					t.Fatalf("WriteFile: %v", err)
				}
				return dir
			},
		},
		{
			name: "error when destDir absent",
			setup: func(t *testing.T) string {
				return "/nonexistent/path/" + t.Name()
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			destDir := tt.setup(t)

			err := ExtractTo(destDir)

			if tt.wantErr {
				if err == nil {
					t.Fatal("ExtractTo() error = nil, want non-nil error")
				}
				return
			}

			if err != nil {
				t.Fatalf("ExtractTo() unexpected error: %v", err)
			}

			// Verify the file was written with the correct content
			written, readErr := os.ReadFile(filepath.Join(destDir, "skillsync", "SKILL.md"))
			if readErr != nil {
				t.Fatalf("ReadFile: %v", readErr)
			}
			if !bytes.Equal(written, Content()) {
				t.Error("written SKILL.md content does not match Content()")
			}
		})
	}
}

// TestExtractTo_MissingDestDir verifies the error message contains "creating skill dir".
func TestExtractTo_MissingDestDir(t *testing.T) {
	err := ExtractTo("/nonexistent/path/for/skillsync/test")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !containsStr(err.Error(), "creating skill dir") {
		t.Errorf("error = %q, want to contain 'creating skill dir'", err.Error())
	}
}

// TestExtractTo_SkillMDContentValid verifies the returned skill has the correct name
// and the written SKILL.md starts with valid frontmatter.
func TestExtractTo_SkillMDContentValid(t *testing.T) {
	destDir := t.TempDir()

	if err := ExtractTo(destDir); err != nil {
		t.Fatalf("ExtractTo() error = %v", err)
	}

	// Verify SkillName constant
	if SkillName != "skillsync" {
		t.Errorf("SkillName = %q, want %q", SkillName, "skillsync")
	}

	// Verify the written file starts with frontmatter
	written, err := os.ReadFile(filepath.Join(destDir, SkillName, "SKILL.md"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !bytes.HasPrefix(written, []byte("---\n")) {
		t.Error("written SKILL.md does not start with frontmatter delimiter '---\\n'")
	}
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
