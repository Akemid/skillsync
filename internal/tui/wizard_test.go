package tui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Akemid/skillsync/internal/config"
	"github.com/Akemid/skillsync/internal/registry"
)

// ---------------------------------------------------------------------------
// localBundleSkills
// ---------------------------------------------------------------------------

func TestLocalBundleSkills(t *testing.T) {
	tests := []struct {
		name   string
		skills []registry.Skill
		want   []string
	}{
		{
			name:   "empty registry",
			skills: []registry.Skill{},
			want:   []string{},
		},
		{
			name: "single skill",
			skills: []registry.Skill{
				{Name: "go-testing"},
			},
			want: []string{"go-testing"},
		},
		{
			name: "multiple skills returns all names",
			skills: []registry.Skill{
				{Name: "fastapi"},
				{Name: "linting"},
				{Name: "docs"},
			},
			want: []string{"fastapi", "linting", "docs"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := &registry.Registry{Skills: tt.skills}
			got := localBundleSkills(reg)

			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d; got %v", len(got), len(tt.want), got)
			}
			for i, name := range tt.want {
				if got[i] != name {
					t.Errorf("got[%d] = %q, want %q", i, got[i], name)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// flattenSkills
// ---------------------------------------------------------------------------

func TestFlattenSkills(t *testing.T) {
	tests := []struct {
		name    string
		input   map[string][]string
		wantLen int
		wantAll []string // every name must appear exactly once
	}{
		{
			name:    "empty map",
			input:   map[string][]string{},
			wantLen: 0,
			wantAll: nil,
		},
		{
			name:    "single bundle",
			input:   map[string][]string{"local": {"go-testing", "fastapi"}},
			wantLen: 2,
			wantAll: []string{"go-testing", "fastapi"},
		},
		{
			name: "deduplicates across bundles",
			input: map[string][]string{
				"bundle-a": {"skill-1", "shared"},
				"bundle-b": {"shared", "skill-2"},
			},
			wantLen: 3,
			wantAll: []string{"skill-1", "shared", "skill-2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := flattenSkills(tt.input)

			if len(got) != tt.wantLen {
				t.Fatalf("len = %d, want %d; got %v", len(got), tt.wantLen, got)
			}

			// Verify each expected name appears exactly once
			counts := make(map[string]int, len(got))
			for _, name := range got {
				counts[name]++
			}
			for _, name := range tt.wantAll {
				if counts[name] != 1 {
					t.Errorf("%q appears %d times, want 1", name, counts[name])
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// buildSkillOptions
// ---------------------------------------------------------------------------

func TestBuildSkillOptions_Labels(t *testing.T) {
	tests := []struct {
		name     string
		skills   []registry.Skill
		wantKeys []string
		wantVals []string
	}{
		{
			name: "name only",
			skills: []registry.Skill{
				{Name: "go-testing"},
			},
			wantKeys: []string{"go-testing"},
			wantVals: []string{"go-testing"},
		},
		{
			name: "with description",
			skills: []registry.Skill{
				{Name: "linting", Description: "runs golangci-lint"},
			},
			wantKeys: []string{"linting — runs golangci-lint"},
			wantVals: []string{"linting"},
		},
		{
			name: "mixed",
			skills: []registry.Skill{
				{Name: "a", Description: "does a"},
				{Name: "b"},
			},
			wantKeys: []string{"a — does a", "b"},
			wantVals: []string{"a", "b"},
		},
		{
			name: "long description is truncated at 60 chars",
			skills: []registry.Skill{
				{Name: "long-skill", Description: "This description is longer than sixty characters for testing!"},
			},
			wantKeys: []string{"long-skill — This description is longer than sixty characters for test..."},
			wantVals: []string{"long-skill"},
		},
		{
			name:     "empty",
			skills:   []registry.Skill{},
			wantKeys: []string{},
			wantVals: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := buildSkillOptions(tt.skills)

			if len(opts) != len(tt.wantKeys) {
				t.Fatalf("len(opts) = %d, want %d", len(opts), len(tt.wantKeys))
			}

			for i, opt := range opts {
				if opt.Key != tt.wantKeys[i] {
					t.Errorf("opts[%d].Key = %q, want %q", i, opt.Key, tt.wantKeys[i])
				}
				if opt.Value != tt.wantVals[i] {
					t.Errorf("opts[%d].Value = %q, want %q", i, opt.Value, tt.wantVals[i])
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// buildToolOptions
// ---------------------------------------------------------------------------

func TestBuildToolOptions(t *testing.T) {
	tests := []struct {
		name            string
		tools           []config.Tool
		wantLen         int
		wantPreSelected []string
	}{
		{
			name: "no enabled tools",
			tools: []config.Tool{
				{Name: "claude", Enabled: false},
				{Name: "cursor", Enabled: false},
			},
			wantLen:         2,
			wantPreSelected: nil,
		},
		{
			name: "some enabled",
			tools: []config.Tool{
				{Name: "claude", Enabled: true},
				{Name: "cursor", Enabled: false},
				{Name: "copilot", Enabled: true},
			},
			wantLen:         3,
			wantPreSelected: []string{"claude", "copilot"},
		},
		{
			name:            "empty tools",
			tools:           []config.Tool{},
			wantLen:         0,
			wantPreSelected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts, preSelected := buildToolOptions(tt.tools)

			if len(opts) != tt.wantLen {
				t.Errorf("len(opts) = %d, want %d", len(opts), tt.wantLen)
			}

			if len(preSelected) != len(tt.wantPreSelected) {
				t.Fatalf("len(preSelected) = %d, want %d", len(preSelected), len(tt.wantPreSelected))
			}
			for i, name := range tt.wantPreSelected {
				if preSelected[i] != name {
					t.Errorf("preSelected[%d] = %q, want %q", i, preSelected[i], name)
				}
			}
		})
	}
}

func TestBuildToolOptions_DetectedLabel(t *testing.T) {
	tools := []config.Tool{
		{Name: "claude", Enabled: true},
		{Name: "cursor", Enabled: false},
	}

	opts, _ := buildToolOptions(tools)

	if opts[0].Key != "claude (detected)" {
		t.Errorf("enabled tool label = %q, want %q", opts[0].Key, "claude (detected)")
	}
	if opts[1].Key != "cursor" {
		t.Errorf("disabled tool label = %q, want %q", opts[1].Key, "cursor")
	}
}

// ---------------------------------------------------------------------------
// explicitBundleSkills
// ---------------------------------------------------------------------------

func TestExplicitBundleSkills(t *testing.T) {
	tests := []struct {
		name  string
		input config.Bundle
		want  []string
	}{
		{
			name:  "empty skills",
			input: config.Bundle{Name: "empty"},
			want:  []string{},
		},
		{
			name: "single skill",
			input: config.Bundle{
				Name:   "single",
				Skills: []config.SkillRef{{Name: "go-testing"}},
			},
			want: []string{"go-testing"},
		},
		{
			name: "multiple skills",
			input: config.Bundle{
				Name: "multi",
				Skills: []config.SkillRef{
					{Name: "go-testing"},
					{Name: "linting"},
					{Name: "docs"},
				},
			},
			want: []string{"go-testing", "linting", "docs"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := explicitBundleSkills(tt.input)

			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d", len(got), len(tt.want))
			}
			for i, name := range tt.want {
				if got[i] != name {
					t.Errorf("got[%d] = %q, want %q", i, got[i], name)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// readSkillsFromDir
// ---------------------------------------------------------------------------

func TestReadSkillsFromDir(t *testing.T) {
	t.Run("returns only non-hidden subdirectories", func(t *testing.T) {
		dir := t.TempDir()

		// visible skill dirs
		for _, name := range []string{"skill-a", "skill-b", "skill-c"} {
			if err := os.MkdirAll(filepath.Join(dir, name), 0755); err != nil {
				t.Fatalf("MkdirAll(%s): %v", name, err)
			}
		}
		// hidden dir — must be excluded
		if err := os.MkdirAll(filepath.Join(dir, ".git"), 0755); err != nil {
			t.Fatalf("MkdirAll(.git): %v", err)
		}
		// file — must be excluded
		if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("x"), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}

		skills, err := readSkillsFromDir(dir, "test-bundle")
		if err != nil {
			t.Fatalf("readSkillsFromDir() error = %v", err)
		}

		if len(skills) != 3 {
			t.Fatalf("got %d skills, want 3: %v", len(skills), skills)
		}
	})

	t.Run("empty directory returns empty slice", func(t *testing.T) {
		dir := t.TempDir()

		skills, err := readSkillsFromDir(dir, "empty-bundle")
		if err != nil {
			t.Fatalf("readSkillsFromDir() error = %v", err)
		}

		if len(skills) != 0 {
			t.Errorf("got %d skills, want 0", len(skills))
		}
	})

	t.Run("non-existent directory returns error", func(t *testing.T) {
		_, err := readSkillsFromDir("/tmp/this-does-not-exist-skillsync-test", "missing")
		if err == nil {
			t.Error("expected error for missing dir, got nil")
		}
	})
}

// ---------------------------------------------------------------------------
// DetectInstalledTools
// ---------------------------------------------------------------------------

func TestDetectInstalledTools(t *testing.T) {
	t.Run("detects tool when parent directory exists", func(t *testing.T) {
		toolDir := t.TempDir()
		skillsDir := filepath.Join(toolDir, "skills")
		if err := os.MkdirAll(skillsDir, 0755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}

		tools := []config.Tool{
			{Name: "present-tool", GlobalPath: skillsDir, Enabled: false},
		}

		result := DetectInstalledTools(tools)

		if !result[0].Enabled {
			t.Errorf("expected present-tool to be Enabled = true")
		}
	})

	t.Run("does not enable tool when parent directory is missing", func(t *testing.T) {
		tools := []config.Tool{
			{Name: "absent-tool", GlobalPath: "/tmp/skillsync-nonexistent-tool/skills", Enabled: false},
		}

		result := DetectInstalledTools(tools)

		if result[0].Enabled {
			t.Errorf("expected absent-tool to be Enabled = false")
		}
	})

	t.Run("does not mutate original slice", func(t *testing.T) {
		tools := []config.Tool{
			{Name: "tool", GlobalPath: "/tmp/skillsync-nonexistent-tool/skills", Enabled: false},
		}

		_ = DetectInstalledTools(tools)

		if tools[0].Enabled {
			t.Error("original slice was mutated")
		}
	})

	t.Run("preserves already-enabled tools", func(t *testing.T) {
		tools := []config.Tool{
			{Name: "already", GlobalPath: "/tmp/skillsync-nonexistent-tool/skills", Enabled: true},
		}

		result := DetectInstalledTools(tools)

		if !result[0].Enabled {
			t.Error("pre-enabled tool should remain Enabled = true")
		}
	})
}
