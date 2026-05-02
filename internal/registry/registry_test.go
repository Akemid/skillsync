package registry

import (
	"os"
	"path/filepath"
	"testing"
)

// TestDiscover_LocalOnly tests discovering only local skills (no _remote)
func TestDiscover_LocalOnly(t *testing.T) {
	tempDir := t.TempDir()

	// Create local skill directories
	localSkill1 := filepath.Join(tempDir, "skill-a")
	localSkill2 := filepath.Join(tempDir, "skill-b")

	if err := os.MkdirAll(localSkill1, 0755); err != nil {
		t.Fatalf("failed to create skill-a: %v", err)
	}
	if err := os.MkdirAll(localSkill2, 0755); err != nil {
		t.Fatalf("failed to create skill-b: %v", err)
	}

	// Create SKILL.md files
	skill1MD := `---
name: skill-a
description: Test skill A
---
# Skill A`

	skill2MD := `---
name: skill-b
description: Test skill B
---
# Skill B`

	if err := os.WriteFile(filepath.Join(localSkill1, "SKILL.md"), []byte(skill1MD), 0644); err != nil {
		t.Fatalf("failed to write SKILL.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(localSkill2, "SKILL.md"), []byte(skill2MD), 0644); err != nil {
		t.Fatalf("failed to write SKILL.md: %v", err)
	}

	// Discover skills
	reg := New(tempDir)
	if err := reg.Discover(); err != nil {
		t.Fatalf("Discover() error = %v, want nil", err)
	}

	// Verify both skills found
	if len(reg.Skills) != 2 {
		t.Fatalf("got %d skills, want 2", len(reg.Skills))
	}

	// Verify skill names
	names := make(map[string]bool)
	for _, skill := range reg.Skills {
		names[skill.Name] = true
	}

	if !names["skill-a"] {
		t.Error("skill-a not found")
	}
	if !names["skill-b"] {
		t.Error("skill-b not found")
	}
}

// TestDiscover_LocalAndRemote tests discovering both local and remote skills
func TestDiscover_LocalAndRemote(t *testing.T) {
	tempDir := t.TempDir()

	// Create local skill
	localSkill := filepath.Join(tempDir, "local-skill")
	if err := os.MkdirAll(localSkill, 0755); err != nil {
		t.Fatalf("failed to create local skill: %v", err)
	}

	localMD := `---
name: local-skill
description: Local skill
---
# Local`

	if err := os.WriteFile(filepath.Join(localSkill, "SKILL.md"), []byte(localMD), 0644); err != nil {
		t.Fatalf("failed to write local SKILL.md: %v", err)
	}

	// Create remote bundle with skills
	remoteBundle := filepath.Join(tempDir, "_remote", "test-bundle")
	remoteSkill1 := filepath.Join(remoteBundle, "remote-skill-1")
	remoteSkill2 := filepath.Join(remoteBundle, "remote-skill-2")

	if err := os.MkdirAll(remoteSkill1, 0755); err != nil {
		t.Fatalf("failed to create remote skill 1: %v", err)
	}
	if err := os.MkdirAll(remoteSkill2, 0755); err != nil {
		t.Fatalf("failed to create remote skill 2: %v", err)
	}

	remote1MD := `---
name: remote-skill-1
description: Remote skill 1
---
# Remote 1`

	remote2MD := `---
name: remote-skill-2
description: Remote skill 2
---
# Remote 2`

	if err := os.WriteFile(filepath.Join(remoteSkill1, "SKILL.md"), []byte(remote1MD), 0644); err != nil {
		t.Fatalf("failed to write remote SKILL.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(remoteSkill2, "SKILL.md"), []byte(remote2MD), 0644); err != nil {
		t.Fatalf("failed to write remote SKILL.md: %v", err)
	}

	// Discover skills
	reg := New(tempDir)
	if err := reg.Discover(); err != nil {
		t.Fatalf("Discover() error = %v, want nil", err)
	}

	// Verify all 3 skills found
	if len(reg.Skills) != 3 {
		t.Fatalf("got %d skills, want 3", len(reg.Skills))
	}

	// Verify skill names
	names := make(map[string]bool)
	for _, skill := range reg.Skills {
		names[skill.Name] = true
	}

	if !names["local-skill"] {
		t.Error("local-skill not found")
	}
	if !names["remote-skill-1"] {
		t.Error("remote-skill-1 not found")
	}
	if !names["remote-skill-2"] {
		t.Error("remote-skill-2 not found")
	}

	// Verify paths are different
	var localPath, remotePath string
	for _, skill := range reg.Skills {
		if skill.Name == "local-skill" {
			localPath = skill.Path
		}
		if skill.Name == "remote-skill-1" {
			remotePath = skill.Path
		}
	}

	if localPath == "" {
		t.Error("local skill path is empty")
	}
	if remotePath == "" {
		t.Error("remote skill path is empty")
	}

	// Local should not contain "_remote"
	if contains(localPath, "_remote") {
		t.Errorf("local path %q should not contain '_remote'", localPath)
	}

	// Remote should contain "_remote"
	if !contains(remotePath, "_remote") {
		t.Errorf("remote path %q should contain '_remote'", remotePath)
	}
}

// TestDiscover_MissingRemoteDir tests that missing _remote/ is not an error
func TestDiscover_MissingRemoteDir(t *testing.T) {
	tempDir := t.TempDir()

	// Create only local skill (no _remote directory)
	localSkill := filepath.Join(tempDir, "local-skill")
	if err := os.MkdirAll(localSkill, 0755); err != nil {
		t.Fatalf("failed to create local skill: %v", err)
	}

	localMD := `---
name: local-skill
description: Local skill
---
# Local`

	if err := os.WriteFile(filepath.Join(localSkill, "SKILL.md"), []byte(localMD), 0644); err != nil {
		t.Fatalf("failed to write SKILL.md: %v", err)
	}

	// Discover skills (should not error even though _remote/ doesn't exist)
	reg := New(tempDir)
	if err := reg.Discover(); err != nil {
		t.Errorf("Discover() error = %v, want nil (missing _remote/ should not be an error)", err)
	}

	// Should find the local skill
	if len(reg.Skills) != 1 {
		t.Fatalf("got %d skills, want 1", len(reg.Skills))
	}

	if reg.Skills[0].Name != "local-skill" {
		t.Errorf("skill name = %q, want 'local-skill'", reg.Skills[0].Name)
	}
}

// TestDiscover_SkipsRemoteInBasePath tests that _remote directory is skipped in basePath scan
func TestDiscover_SkipsRemoteInBasePath(t *testing.T) {
	tempDir := t.TempDir()

	// Create a skill named "_remote" at the base level (should be skipped)
	remoteSkillDir := filepath.Join(tempDir, "_remote")
	if err := os.MkdirAll(remoteSkillDir, 0755); err != nil {
		t.Fatalf("failed to create _remote dir: %v", err)
	}

	remoteMD := `---
name: _remote
description: Should be skipped
---
# This should not be discovered as a skill`

	if err := os.WriteFile(filepath.Join(remoteSkillDir, "SKILL.md"), []byte(remoteMD), 0644); err != nil {
		t.Fatalf("failed to write SKILL.md: %v", err)
	}

	// Create a normal local skill
	localSkill := filepath.Join(tempDir, "normal-skill")
	if err := os.MkdirAll(localSkill, 0755); err != nil {
		t.Fatalf("failed to create normal skill: %v", err)
	}

	localMD := `---
name: normal-skill
description: Normal skill
---
# Normal`

	if err := os.WriteFile(filepath.Join(localSkill, "SKILL.md"), []byte(localMD), 0644); err != nil {
		t.Fatalf("failed to write SKILL.md: %v", err)
	}

	// Discover skills
	reg := New(tempDir)
	if err := reg.Discover(); err != nil {
		t.Fatalf("Discover() error = %v, want nil", err)
	}

	// Should only find normal-skill, not _remote
	if len(reg.Skills) != 1 {
		t.Fatalf("got %d skills, want 1 (_remote should be skipped)", len(reg.Skills))
	}

	if reg.Skills[0].Name != "normal-skill" {
		t.Errorf("skill name = %q, want 'normal-skill'", reg.Skills[0].Name)
	}

	// Verify _remote was not discovered as a skill
	for _, skill := range reg.Skills {
		if skill.Name == "_remote" {
			t.Error("_remote directory should not be discovered as a skill")
		}
	}
}

// TestDiscover_MultipleRemoteBundles tests multiple bundles in _remote/
func TestDiscover_MultipleRemoteBundles(t *testing.T) {
	tempDir := t.TempDir()

	// Create remote bundle 1
	bundle1 := filepath.Join(tempDir, "_remote", "bundle-1")
	skill1 := filepath.Join(bundle1, "skill-1")
	if err := os.MkdirAll(skill1, 0755); err != nil {
		t.Fatalf("failed to create skill-1: %v", err)
	}

	skill1MD := `---
name: skill-1
description: Skill from bundle 1
---
# Skill 1`

	if err := os.WriteFile(filepath.Join(skill1, "SKILL.md"), []byte(skill1MD), 0644); err != nil {
		t.Fatalf("failed to write SKILL.md: %v", err)
	}

	// Create remote bundle 2
	bundle2 := filepath.Join(tempDir, "_remote", "bundle-2")
	skill2 := filepath.Join(bundle2, "skill-2")
	if err := os.MkdirAll(skill2, 0755); err != nil {
		t.Fatalf("failed to create skill-2: %v", err)
	}

	skill2MD := `---
name: skill-2
description: Skill from bundle 2
---
# Skill 2`

	if err := os.WriteFile(filepath.Join(skill2, "SKILL.md"), []byte(skill2MD), 0644); err != nil {
		t.Fatalf("failed to write SKILL.md: %v", err)
	}

	// Discover skills
	reg := New(tempDir)
	if err := reg.Discover(); err != nil {
		t.Fatalf("Discover() error = %v, want nil", err)
	}

	// Should find both skills
	if len(reg.Skills) != 2 {
		t.Fatalf("got %d skills, want 2", len(reg.Skills))
	}

	// Index by name for deterministic assertions
	byName := make(map[string]Skill)
	for _, skill := range reg.Skills {
		byName[skill.Name] = skill
	}

	if _, ok := byName["skill-1"]; !ok {
		t.Error("skill-1 not found")
	}
	if _, ok := byName["skill-2"]; !ok {
		t.Error("skill-2 not found")
	}

	// W2 fix: assert Bundle values are correct per skill
	if got := byName["skill-1"].Bundle; got != "bundle-1" {
		t.Errorf("skill-1 Bundle = %q, want %q", got, "bundle-1")
	}
	if got := byName["skill-2"].Bundle; got != "bundle-2" {
		t.Errorf("skill-2 Bundle = %q, want %q", got, "bundle-2")
	}
}

// TestFindByBundleAndName verifies exact bundle+name lookup.
func TestFindByBundleAndName(t *testing.T) {
	acmeSkill := Skill{Name: "go-testing", Bundle: "acme", Path: "/fake/acme/go-testing"}
	localSkill := Skill{Name: "local-s", Bundle: "", Path: "/fake/local-s"}

	tests := []struct {
		name       string
		registry   Registry
		bundle     string
		skillName  string
		wantSkill  Skill
		wantFound  bool
	}{
		{
			name:      "found in bundle",
			registry:  Registry{Skills: []Skill{acmeSkill}},
			bundle:    "acme",
			skillName: "go-testing",
			wantSkill: acmeSkill,
			wantFound: true,
		},
		{
			name:      "wrong bundle",
			registry:  Registry{Skills: []Skill{acmeSkill}},
			bundle:    "other",
			skillName: "go-testing",
			wantSkill: Skill{},
			wantFound: false,
		},
		{
			name:      "local skill with empty bundle",
			registry:  Registry{Skills: []Skill{localSkill}},
			bundle:    "",
			skillName: "local-s",
			wantSkill: localSkill,
			wantFound: true,
		},
		{
			name:      "empty registry",
			registry:  Registry{},
			bundle:    "acme",
			skillName: "go-testing",
			wantSkill: Skill{},
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := tt.registry.FindByBundleAndName(tt.bundle, tt.skillName)
			if ok != tt.wantFound {
				t.Errorf("FindByBundleAndName() found = %v, want %v", ok, tt.wantFound)
			}
			if got.Name != tt.wantSkill.Name || got.Bundle != tt.wantSkill.Bundle || got.Path != tt.wantSkill.Path {
				t.Errorf("FindByBundleAndName() skill = %+v, want %+v", got, tt.wantSkill)
			}
		})
	}
}

// TestFindByBundle verifies that FindByBundle returns matching skills.
func TestFindByBundle(t *testing.T) {
	fe1 := Skill{Name: "react-patterns", Bundle: "frontend-cen", Path: "/fake/react-patterns"}
	fe2 := Skill{Name: "ts-strict", Bundle: "frontend-cen", Path: "/fake/ts-strict"}
	be1 := Skill{Name: "go-testing", Bundle: "backend-cen", Path: "/fake/go-testing"}
	loc1 := Skill{Name: "local-a", Bundle: "", Path: "/fake/local-a"}
	loc2 := Skill{Name: "local-b", Bundle: "", Path: "/fake/local-b"}

	tests := []struct {
		name     string
		registry Registry
		bundle   string
		wantLen  int
		wantNil  bool
	}{
		{
			name:     "two skills in frontend-cen, one in backend-cen",
			registry: Registry{Skills: []Skill{fe1, fe2, be1}},
			bundle:   "frontend-cen",
			wantLen:  2,
			wantNil:  false,
		},
		{
			name:     "local skills with empty bundle",
			registry: Registry{Skills: []Skill{loc1, loc2, be1}},
			bundle:   "",
			wantLen:  2,
			wantNil:  false,
		},
		{
			name:     "no match returns nil",
			registry: Registry{Skills: []Skill{fe1, be1}},
			bundle:   "nonexistent",
			wantLen:  0,
			wantNil:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.registry.FindByBundle(tt.bundle)
			if tt.wantNil {
				if got != nil {
					t.Errorf("FindByBundle() = %v, want nil", got)
				}
				return
			}
			if len(got) != tt.wantLen {
				t.Errorf("FindByBundle() len = %d, want %d", len(got), tt.wantLen)
			}
		})
	}
}

// TestDiscover_BundleField verifies that Bundle is populated correctly for local and remote skills.
func TestDiscover_BundleField(t *testing.T) {
	skillMD := "---\nname: skill-name\ndescription: test\n---\n"

	tests := []struct {
		name       string
		setup      func(t *testing.T, base string)
		skillName  string
		wantBundle string
	}{
		{
			name: "local",
			setup: func(t *testing.T, base string) {
				dir := filepath.Join(base, "local-skill")
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatalf("MkdirAll: %v", err)
				}
				if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(skillMD), 0644); err != nil {
					t.Fatalf("WriteFile: %v", err)
				}
			},
			skillName:  "local-skill",
			wantBundle: "",
		},
		{
			name: "remote",
			setup: func(t *testing.T, base string) {
				dir := filepath.Join(base, "_remote", "acme", "remote-skill")
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatalf("MkdirAll: %v", err)
				}
				if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(skillMD), 0644); err != nil {
					t.Fatalf("WriteFile: %v", err)
				}
			},
			skillName:  "remote-skill",
			wantBundle: "acme",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base := t.TempDir()
			tt.setup(t, base)

			reg := New(base)
			if err := reg.Discover(); err != nil {
				t.Fatalf("Discover() error = %v, want nil", err)
			}

			var found *Skill
			for i := range reg.Skills {
				if reg.Skills[i].Name == tt.skillName {
					found = &reg.Skills[i]
					break
				}
			}
			if found == nil {
				t.Fatalf("skill %q not found after Discover()", tt.skillName)
			}
			if found.Bundle != tt.wantBundle {
				t.Errorf("Bundle = %q, want %q", found.Bundle, tt.wantBundle)
			}
		})
	}
}

// Helper function
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
