package installer_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Akemid/skillsync/internal/config"
	"github.com/Akemid/skillsync/internal/installer"
	"github.com/Akemid/skillsync/internal/registry"
)

func makeSkillDir(t *testing.T, base, name string) string {
	t.Helper()

	skillPath := filepath.Join(base, name)
	if err := os.MkdirAll(skillPath, 0755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", skillPath, err)
	}
	if err := os.WriteFile(filepath.Join(skillPath, "SKILL.md"), []byte("# test skill\n"), 0644); err != nil {
		t.Fatalf("WriteFile(SKILL.md): %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillPath, "script.sh"), []byte("echo test\n"), 0755); err != nil {
		t.Fatalf("WriteFile(script.sh): %v", err)
	}

	return skillPath
}

func TestInstall_CopyMode_Created(t *testing.T) {
	registryDir := t.TempDir()
	targetDir := t.TempDir()
	skillPath := makeSkillDir(t, registryDir, "find-skills")

	tool := config.Tool{Name: "kiro-ide", GlobalPath: targetDir, InstallMode: "copy", Enabled: true}
	skills := []registry.Skill{{Name: "find-skills", Path: skillPath}}

	results := installer.Install(skills, []config.Tool{tool}, installer.ScopeGlobal, "")
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}

	result := results[0]
	if result.Error != nil {
		t.Fatalf("Install() error = %v, want nil", result.Error)
	}
	if !result.Created {
		t.Fatalf("Result.Created = %v, want true", result.Created)
	}

	target := filepath.Join(targetDir, "find-skills")
	for _, name := range []string{"SKILL.md", "script.sh"} {
		full := filepath.Join(target, name)
		info, err := os.Lstat(full)
		if err != nil {
			t.Fatalf("Lstat(%s): %v", full, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			t.Fatalf("%s is a symlink, want regular file", full)
		}
	}
}

func TestInstall_CopyMode_Idempotent(t *testing.T) {
	registryDir := t.TempDir()
	targetDir := t.TempDir()
	skillPath := makeSkillDir(t, registryDir, "find-skills")

	tool := config.Tool{Name: "kiro-ide", GlobalPath: targetDir, InstallMode: "copy", Enabled: true}
	skills := []registry.Skill{{Name: "find-skills", Path: skillPath}}

	first := installer.Install(skills, []config.Tool{tool}, installer.ScopeGlobal, "")
	if first[0].Error != nil {
		t.Fatalf("first Install() error = %v, want nil", first[0].Error)
	}

	second := installer.Install(skills, []config.Tool{tool}, installer.ScopeGlobal, "")
	if second[0].Error != nil {
		t.Fatalf("second Install() error = %v, want nil", second[0].Error)
	}
	if !second[0].Existed {
		t.Fatalf("second Result.Existed = %v, want true", second[0].Existed)
	}
}

func TestInstall_CopyMode_ConflictSymlink(t *testing.T) {
	registryDir := t.TempDir()
	targetDir := t.TempDir()
	skillPath := makeSkillDir(t, registryDir, "find-skills")

	target := filepath.Join(targetDir, "find-skills")
	if err := os.Symlink(skillPath, target); err != nil {
		t.Fatalf("Symlink(%s -> %s): %v", target, skillPath, err)
	}

	tool := config.Tool{Name: "kiro-ide", GlobalPath: targetDir, InstallMode: "copy", Enabled: true}
	skills := []registry.Skill{{Name: "find-skills", Path: skillPath}}

	results := installer.Install(skills, []config.Tool{tool}, installer.ScopeGlobal, "")
	if results[0].Error == nil {
		t.Fatal("Install() error = nil, want conflict error")
	}
	if !strings.Contains(results[0].Error.Error(), "conflict") {
		t.Fatalf("error = %q, want to contain %q", results[0].Error.Error(), "conflict")
	}
}

func TestInstall_SymlinkMode_Unchanged(t *testing.T) {
	registryDir := t.TempDir()
	targetDir := t.TempDir()
	skillPath := makeSkillDir(t, registryDir, "find-skills")

	tool := config.Tool{Name: "kiro-cli", GlobalPath: targetDir, Enabled: true}
	skills := []registry.Skill{{Name: "find-skills", Path: skillPath}}

	results := installer.Install(skills, []config.Tool{tool}, installer.ScopeGlobal, "")
	if results[0].Error != nil {
		t.Fatalf("Install() error = %v, want nil", results[0].Error)
	}
	if !results[0].Created {
		t.Fatalf("Result.Created = %v, want true", results[0].Created)
	}

	target := filepath.Join(targetDir, "find-skills")
	info, err := os.Lstat(target)
	if err != nil {
		t.Fatalf("Lstat(%s): %v", target, err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("target mode = %v, want symlink", info.Mode())
	}
}

func TestInstall_LegacyKiro_DefaultSymlinkMode(t *testing.T) {
	registryDir := t.TempDir()
	targetDir := t.TempDir()
	skillPath := makeSkillDir(t, registryDir, "find-skills")

	// Legacy config entry: name=kiro and no install_mode must behave as symlink mode.
	tool := config.Tool{Name: "kiro", GlobalPath: targetDir, InstallMode: "", Enabled: true}
	skills := []registry.Skill{{Name: "find-skills", Path: skillPath}}

	results := installer.Install(skills, []config.Tool{tool}, installer.ScopeGlobal, "")
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	if results[0].Error != nil {
		t.Fatalf("Install() error = %v, want nil", results[0].Error)
	}
	if !results[0].Created {
		t.Fatalf("Result.Created = %v, want true", results[0].Created)
	}

	target := filepath.Join(targetDir, "find-skills")
	info, err := os.Lstat(target)
	if err != nil {
		t.Fatalf("Lstat(%s): %v", target, err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("target mode = %v, want symlink", info.Mode())
	}
}

func TestUninstall_CopyMode_WithSentinel(t *testing.T) {
	targetDir := t.TempDir()
	target := filepath.Join(targetDir, "find-skills")
	if err := os.MkdirAll(target, 0755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", target, err)
	}
	if err := os.WriteFile(filepath.Join(target, "SKILL.md"), []byte("# test\n"), 0644); err != nil {
		t.Fatalf("WriteFile(SKILL.md): %v", err)
	}

	tool := config.Tool{Name: "kiro-ide", GlobalPath: targetDir, InstallMode: "copy", Enabled: true}
	results := installer.Uninstall([]string{"find-skills"}, []config.Tool{tool}, installer.ScopeGlobal, "")

	if results[0].Error != nil {
		t.Fatalf("Uninstall() error = %v, want nil", results[0].Error)
	}
	if !results[0].Created {
		t.Fatalf("Result.Created = %v, want true", results[0].Created)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("target should not exist after uninstall, stat err = %v", err)
	}
}

func TestUninstall_CopyMode_NoSentinel(t *testing.T) {
	targetDir := t.TempDir()
	target := filepath.Join(targetDir, "find-skills")
	if err := os.MkdirAll(target, 0755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", target, err)
	}

	tool := config.Tool{Name: "kiro-ide", GlobalPath: targetDir, InstallMode: "copy", Enabled: true}
	results := installer.Uninstall([]string{"find-skills"}, []config.Tool{tool}, installer.ScopeGlobal, "")

	if results[0].Error == nil {
		t.Fatal("Uninstall() error = nil, want refusing error")
	}
	if !strings.Contains(results[0].Error.Error(), "refusing") {
		t.Fatalf("error = %q, want to contain %q", results[0].Error.Error(), "refusing")
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("target should still exist, stat err = %v", err)
	}
}
