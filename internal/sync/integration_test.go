package sync

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Akemid/skillsync/internal/registry"
)

// skipIfNoGit skips the test if git is not available in PATH
func skipIfNoGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not found in PATH")
	}
}

// createTestRepo creates a real git repo with one commit containing a skill directory.
// Returns the repo directory path.
func createTestRepo(t *testing.T) string {
	t.Helper()

	repoDir := t.TempDir()

	setup := [][]string{
		{"git", "init"},
		{"git", "symbolic-ref", "HEAD", "refs/heads/main"},
		{"git", "config", "user.email", "test@skillsync.test"},
		{"git", "config", "user.name", "SkillSync Test"},
	}
	for _, args := range setup {
		mustRunIn(t, repoDir, args[0], args[1:]...)
	}

	skillDir := filepath.Join(repoDir, "test-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("creating skill dir: %v", err)
	}
	skillMD := "---\nname: test-skill\ndescription: Integration test skill\n---\n# Test Skill"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillMD), 0644); err != nil {
		t.Fatalf("writing SKILL.md: %v", err)
	}

	mustRunIn(t, repoDir, "git", "add", ".")
	mustRunIn(t, repoDir, "git", "commit", "-m", "initial commit")

	return repoDir
}

// mustRunIn runs a command in dir, failing the test on any error
func mustRunIn(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %s: %v\n%s", name, strings.Join(args, " "), err, output)
	}
}

// TestSyncBundle_EndToEnd clones a real local repo, adds a commit, then pulls it
func TestSyncBundle_EndToEnd(t *testing.T) {
	skipIfNoGit(t)

	sourceRepo := createTestRepo(t)
	remoteBase := t.TempDir()

	syncer, err := New(remoteBase)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := context.Background()
	url := "file://" + sourceRepo

	// First sync: should clone
	if err := syncer.SyncBundle(ctx, "test-bundle", url, "main", ""); err != nil {
		t.Fatalf("SyncBundle (clone) error = %v", err)
	}

	bundleDir := filepath.Join(remoteBase, "test-bundle")
	if _, err := os.Stat(filepath.Join(bundleDir, ".git")); err != nil {
		t.Errorf(".git directory not found after clone: %v", err)
	}

	// Add a new commit to the source repo
	newSkillDir := filepath.Join(sourceRepo, "new-skill")
	if err := os.MkdirAll(newSkillDir, 0755); err != nil {
		t.Fatalf("creating new skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(newSkillDir, "SKILL.md"), []byte("---\nname: new-skill\n---\n# New Skill"), 0644); err != nil {
		t.Fatalf("writing new SKILL.md: %v", err)
	}
	mustRunIn(t, sourceRepo, "git", "add", ".")
	mustRunIn(t, sourceRepo, "git", "commit", "-m", "add new-skill")

	// Second sync: should pull the new commit
	// Unshallow first so pull works from a depth-1 clone
	mustRunIn(t, bundleDir, "git", "fetch", "--unshallow")
	if err := syncer.SyncBundle(ctx, "test-bundle", url, "main", ""); err != nil {
		t.Fatalf("SyncBundle (pull) error = %v", err)
	}

	// Verify new skill is present after pull
	if _, err := os.Stat(filepath.Join(bundleDir, "new-skill", "SKILL.md")); err != nil {
		t.Errorf("new-skill/SKILL.md not found after pull: %v", err)
	}
}

// TestSyncBundle_AtomicOperations verifies no partial state remains after a clone failure
func TestSyncBundle_AtomicOperations(t *testing.T) {
	skipIfNoGit(t)

	remoteBase := t.TempDir()
	syncer, err := New(remoteBase)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := context.Background()
	// Point to a nonexistent path to force git clone failure
	badURL := "file:///nonexistent-path-that-does-not-exist"
	err = syncer.SyncBundle(ctx, "fail-bundle", badURL, "main", "")
	if err == nil {
		t.Fatal("SyncBundle() error = nil, want error for bad URL")
	}

	// Verify no partial directories remain inside remoteBase
	entries, _ := os.ReadDir(remoteBase)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".tmp-") || e.Name() == "fail-bundle" {
			t.Errorf("unexpected entry %q left behind after failure — atomic cleanup failed", e.Name())
		}
	}
}

// TestRegistry_DiscoverAfterSync syncs a real bundle then verifies registry finds its skills
func TestRegistry_DiscoverAfterSync(t *testing.T) {
	skipIfNoGit(t)

	sourceRepo := createTestRepo(t)
	registryBase := t.TempDir()

	remoteBase := filepath.Join(registryBase, "_remote")
	syncer, err := New(remoteBase)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := context.Background()
	url := "file://" + sourceRepo

	if err := syncer.SyncBundle(ctx, "test-bundle", url, "main", ""); err != nil {
		t.Fatalf("SyncBundle error = %v", err)
	}

	// Registry should discover skills from _remote/test-bundle/
	reg := registry.New(registryBase)
	if err := reg.Discover(); err != nil {
		t.Fatalf("Discover() error = %v", err)
	}

	if len(reg.Skills) == 0 {
		t.Fatal("no skills discovered after sync, want at least 1")
	}

	found := false
	for _, s := range reg.Skills {
		if s.Name == "test-skill" {
			found = true
			break
		}
	}
	if !found {
		names := make([]string, len(reg.Skills))
		for i, s := range reg.Skills {
			names[i] = s.Name
		}
		t.Errorf("test-skill not found in discovered skills: %v", names)
	}
}
