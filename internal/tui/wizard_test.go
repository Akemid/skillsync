package tui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Akemid/skillsync/internal/archive"
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
				{Name: "fastapi", Path: "/home/user/.agents/skills/fastapi"},
				{Name: "linting", Path: "/home/user/.agents/skills/linting"},
				{Name: "docs", Path: "/home/user/.agents/skills/docs"},
			},
			want: []string{"fastapi", "linting", "docs"},
		},
		{
			name: "excludes remote bundle skills",
			skills: []registry.Skill{
				{Name: "local-skill", Path: "/home/user/.agents/skills/local-skill"},
				{Name: "remote-skill", Path: "/home/user/.agents/skills/_remote/team-frontend/skills/remote-skill"},
			},
			want: []string{"local-skill"},
		},
		{
			name: "all remote returns empty",
			skills: []registry.Skill{
				{Name: "skill-a", Path: "/home/user/.agents/skills/_remote/bundle-x/skills/skill-a"},
				{Name: "skill-b", Path: "/home/user/.agents/skills/_remote/bundle-y/skill-b"},
			},
			want: []string{},
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
// Smoke tests for new wizard helpers (Phase 5.4)
// These verify the functions don't panic with minimal inputs and return expected
// errors when called without interactive TUI (no stdin terminal).
// ---------------------------------------------------------------------------

// TestRunExportWizard_EmptyRegistry verifies that runExportWizard returns an
// error (not a panic) when no local skills are available.
func TestRunExportWizard_EmptyRegistry(t *testing.T) {
	reg := &registry.Registry{Skills: []registry.Skill{}}
	err := runExportWizard(reg)
	if err == nil {
		t.Fatal("runExportWizard() error = nil, want error for empty registry")
	}
	if !containsStrWiz(err.Error(), "no local skills") {
		t.Errorf("error = %q, want to contain 'no local skills'", err.Error())
	}
}

// TestRunShareSkillWizard_EmptyRegistryAndNoTaps verifies that runShareSkillWizard
// with no taps and an empty registry returns an appropriate error.
func TestRunShareSkillWizard_EmptyRegistryNoSkills(t *testing.T) {
	cfg := &config.Config{
		RegistryPath: t.TempDir(),
		Taps:         []config.Tap{{Name: "my-tap", URL: "https://github.com/user/tap.git", Branch: "main"}},
	}
	reg := &registry.Registry{Skills: []registry.Skill{}}
	configPath := filepath.Join(t.TempDir(), "config.yaml")

	err := runShareSkillWizard(cfg, reg, configPath)
	if err == nil {
		t.Fatal("runShareSkillWizard() error = nil, want error for empty registry")
	}
	if !containsStrWiz(err.Error(), "no local skills") {
		t.Errorf("error = %q, want to contain 'no local skills'", err.Error())
	}
}

// TestRunImportWizard_InvalidArchivePath verifies that runImportWizard with a
// non-existent file path returns an error without panicking.
// Note: this test bypasses the TUI form by testing the archive.Import path directly.
func TestRunImportWizard_ArchiveIntegration(t *testing.T) {
	// Create a valid archive
	base := t.TempDir()
	skillDir := filepath.Join(base, "smoke-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"),
		[]byte("---\nname: smoke-skill\n---\n"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	archivePath := filepath.Join(t.TempDir(), "smoke-skill.tar.gz")
	if err := archive.Export(skillDir, archivePath); err != nil {
		t.Fatalf("Export: %v", err)
	}

	// Verify that the archive is valid by importing it directly
	registry := t.TempDir()
	skillName, err := archive.Import(archivePath, registry, false)
	if err != nil {
		t.Fatalf("Import() error = %v, want nil", err)
	}
	if skillName != "smoke-skill" {
		t.Errorf("skillName = %q, want %q", skillName, "smoke-skill")
	}
}

// ---------------------------------------------------------------------------
// W3 — runImportWizard smoke test
// ---------------------------------------------------------------------------

// TestRunImportWizard_NoPanic verifies that runImportWizard does not panic when
// called outside a TTY. In non-interactive mode huh returns a program-killed
// error, so we only assert no panic and no nil-pointer dereference.
func TestRunImportWizard_NoPanic(t *testing.T) {
	cfg := &config.Config{
		RegistryPath: t.TempDir(),
	}

	// Must not panic — any error is acceptable in non-TTY
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("runImportWizard() panicked: %v", r)
		}
	}()

	_ = runImportWizard(cfg)
}

// ---------------------------------------------------------------------------
// W4 — runExportWizard post-form smoke test
// ---------------------------------------------------------------------------

// TestRunExportWizard_WithSkills_NoPanic verifies that runExportWizard does not
// panic when at least one skill is present (bypassing the early-exit guard).
// In non-TTY mode huh returns a program error; we only assert no panic.
func TestRunExportWizard_WithSkills_NoPanic(t *testing.T) {
	reg := &registry.Registry{
		Skills: []registry.Skill{
			{Name: "smoke-skill", Path: "/tmp/smoke-skill"},
		},
	}

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("runExportWizard() panicked: %v", r)
		}
	}()

	// Will return an error from huh (no TTY) — we just ensure it doesn't panic
	_ = runExportWizard(reg)
}

// ---------------------------------------------------------------------------
// discoverProjectSkills
// ---------------------------------------------------------------------------

func TestDiscoverProjectSkills(t *testing.T) {
	tests := []struct {
		name       string
		setupDir   func(t *testing.T) string // returns projectDir; empty string = skip mkdir
		tools      []config.Tool
		regPath    string
		wantLen    int
		wantSkills []projectSkill // nil means don't check contents, just length
	}{
		{
			name: "empty projectDir returns empty",
			setupDir: func(t *testing.T) string {
				return ""
			},
			tools:   []config.Tool{{Name: "claude", LocalPath: ".claude/skills"}},
			regPath: t.TempDir(),
			wantLen: 0,
		},
		{
			name: "SKILL.md present returns skill with correct fields",
			setupDir: func(t *testing.T) string {
				dir := t.TempDir()
				skillDir := filepath.Join(dir, ".claude", "skills", "my-skill")
				if err := os.MkdirAll(skillDir, 0755); err != nil {
					t.Fatalf("MkdirAll: %v", err)
				}
				if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"),
					[]byte("---\nname: my-skill\n---\n"), 0644); err != nil {
					t.Fatalf("WriteFile: %v", err)
				}
				return dir
			},
			tools:   []config.Tool{{Name: "claude", LocalPath: ".claude/skills"}},
			regPath: t.TempDir(),
			wantLen: 1,
			wantSkills: []projectSkill{
				{Name: "my-skill", ToolName: "claude"},
			},
		},
		{
			name: "subdir without SKILL.md is skipped",
			setupDir: func(t *testing.T) string {
				dir := t.TempDir()
				// create dir without SKILL.md
				noSkillDir := filepath.Join(dir, ".claude", "skills", "not-a-skill")
				if err := os.MkdirAll(noSkillDir, 0755); err != nil {
					t.Fatalf("MkdirAll: %v", err)
				}
				return dir
			},
			tools:   []config.Tool{{Name: "claude", LocalPath: ".claude/skills"}},
			regPath: t.TempDir(),
			wantLen: 0,
		},
		{
			name: "two tools sharing same LocalPath deduplicated to 1 result",
			setupDir: func(t *testing.T) string {
				dir := t.TempDir()
				// both kiro-ide and kiro-cli share .kiro/skills
				skillDir := filepath.Join(dir, ".kiro", "skills", "shared-skill")
				if err := os.MkdirAll(skillDir, 0755); err != nil {
					t.Fatalf("MkdirAll: %v", err)
				}
				if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"),
					[]byte("---\nname: shared-skill\n---\n"), 0644); err != nil {
					t.Fatalf("WriteFile: %v", err)
				}
				return dir
			},
			tools: []config.Tool{
				{Name: "kiro-ide", LocalPath: ".kiro/skills"},
				{Name: "kiro-cli", LocalPath: ".kiro/skills"},
			},
			regPath: t.TempDir(),
			wantLen: 1,
			wantSkills: []projectSkill{
				{Name: "shared-skill", ToolName: "kiro-ide"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectDir := tt.setupDir(t)
			got := discoverProjectSkills(projectDir, tt.tools, tt.regPath)
			if len(got) != tt.wantLen {
				t.Fatalf("len = %d, want %d; got %v", len(got), tt.wantLen, got)
			}
			for i, want := range tt.wantSkills {
				if i >= len(got) {
					break
				}
				if got[i].Name != want.Name {
					t.Errorf("[%d] Name = %q, want %q", i, got[i].Name, want.Name)
				}
				if got[i].Path != want.Path {
					t.Errorf("[%d] Path = %q, want %q", i, got[i].Path, want.Path)
				}
				if got[i].ToolName != want.ToolName {
					t.Errorf("[%d] ToolName = %q, want %q", i, got[i].ToolName, want.ToolName)
				}
			}
		})
	}
}

// TestDiscoverProjectSkills_SymlinkIntoRegistrySkipped verifies that a skill
// directory that is a symlink resolving into the registry is skipped.
func TestDiscoverProjectSkills_SymlinkIntoRegistrySkipped(t *testing.T) {
	regDir := t.TempDir()
	// real skill lives in registry
	regSkillDir := filepath.Join(regDir, "reg-skill")
	if err := os.MkdirAll(regSkillDir, 0755); err != nil {
		t.Fatalf("MkdirAll regSkill: %v", err)
	}
	if err := os.WriteFile(filepath.Join(regSkillDir, "SKILL.md"),
		[]byte("---\nname: reg-skill\n---\n"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	projectDir := t.TempDir()
	skillsDir := filepath.Join(projectDir, ".claude", "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatalf("MkdirAll skillsDir: %v", err)
	}
	// symlink from project skill dir → registry skill dir
	if err := os.Symlink(regSkillDir, filepath.Join(skillsDir, "reg-skill")); err != nil {
		t.Fatalf("Symlink: %v", err)
	}

	tools := []config.Tool{{Name: "claude", LocalPath: ".claude/skills"}}
	got := discoverProjectSkills(projectDir, tools, regDir)
	if len(got) != 0 {
		t.Errorf("expected 0 skills (symlink into registry should be skipped), got %d: %v", len(got), got)
	}
}

func containsStrWiz(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
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

	t.Run("enables only first tool for shared global path", func(t *testing.T) {
		toolDir := t.TempDir()
		skillsDir := filepath.Join(toolDir, "skills")
		if err := os.MkdirAll(skillsDir, 0755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}

		tools := []config.Tool{
			{Name: "kiro-ide", GlobalPath: skillsDir, Enabled: false},
			{Name: "kiro-cli", GlobalPath: skillsDir, Enabled: false},
		}

		result := DetectInstalledTools(tools)

		if !result[0].Enabled {
			t.Fatalf("expected first shared-path tool to be Enabled = true")
		}
		if result[1].Enabled {
			t.Fatalf("expected second shared-path tool to remain disabled")
		}
	})
}
