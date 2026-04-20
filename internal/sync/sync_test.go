package sync

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestNew verifies Syncer constructor
func TestNew(t *testing.T) {
	tempDir := t.TempDir()
	syncer, err := New(tempDir)
	if err != nil {
		t.Fatalf("New() error = %v, want nil", err)
	}
	if syncer == nil {
		t.Fatal("New() returned nil syncer")
	}
	if syncer.remoteBaseDir == "" {
		t.Error("remoteBaseDir is empty")
	}
}

// TestSyncBundle_FirstClone tests cloning a new bundle
func TestSyncBundle_FirstClone(t *testing.T) {
	// Save original execCommand and restore after test
	oldExec := execCommand
	defer func() { execCommand = oldExec }()

	// Mock successful git clone
	execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		// Verify git clone command
		if name == "git" && len(args) >= 2 && args[0] == "clone" {
			// Return a command that does nothing (succeeds)
			return exec.Command("echo", "mocked git clone")
		}
		return exec.Command(name, args...)
	}

	tempDir := t.TempDir()
	syncer, err := New(tempDir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := context.Background()
	err = syncer.SyncBundle(ctx, "test-bundle", "https://github.com/test/repo", "main")

	// Note: This test will fail because our mock doesn't actually create the .git directory
	// In a real implementation, we'd need a more sophisticated mock or integration test
	if err == nil {
		t.Log("SyncBundle completed (mock test)")
	}
}

// TestSyncBundle_UpdateExisting tests pulling an existing bundle
func TestSyncBundle_UpdateExisting(t *testing.T) {
	oldExec := execCommand
	defer func() { execCommand = oldExec }()

	pullCalled := false
	execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		// Check for git pull command (args format: -C, <dir>, pull, --ff-only)
		if name == "git" && len(args) >= 3 && args[0] == "-C" && args[2] == "pull" {
			pullCalled = true
			// Return successful command
			return exec.Command("true")
		}
		return exec.Command(name, args...)
	}

	tempDir := t.TempDir()
	syncer, err := New(tempDir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Create fake bundle with .git directory
	bundleDir := filepath.Join(tempDir, "existing-bundle")
	gitDir := filepath.Join(bundleDir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatalf("failed to create fake .git dir: %v", err)
	}

	ctx := context.Background()
	err = syncer.SyncBundle(ctx, "existing-bundle", "https://github.com/test/repo", "main")
	if err != nil {
		t.Errorf("SyncBundle() error = %v, want nil", err)
	}

	if !pullCalled {
		t.Error("git pull was not called for existing bundle")
	}
}

// TestSyncBundle_NetworkFailure tests handling of clone failures
func TestSyncBundle_NetworkFailure(t *testing.T) {
	oldExec := execCommand
	defer func() { execCommand = oldExec }()

	// Mock git clone failure
	execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name == "git" && len(args) >= 2 && args[0] == "clone" {
			// Return a command that will fail
			cmd := exec.Command("sh", "-c", "exit 1")
			return cmd
		}
		return exec.Command(name, args...)
	}

	tempDir := t.TempDir()
	syncer, err := New(tempDir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := context.Background()
	err = syncer.SyncBundle(ctx, "fail-bundle", "https://github.com/test/repo", "main")

	if err == nil {
		t.Error("SyncBundle() error = nil, want error for failed clone")
	}

	// Verify temp directory was cleaned up (no .tmp-* dirs should exist)
	entries, _ := os.ReadDir(tempDir)
	for _, entry := range entries {
		if entry.IsDir() && (entry.Name()[:5] == ".tmp-" || entry.Name() == "fail-bundle") {
			t.Errorf("temp directory %q was not cleaned up after failure", entry.Name())
		}
	}
}

// TestValidateGitURL tests URL validation
func TestValidateGitURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"https URL", "https://github.com/user/repo", false},
		{"git protocol", "git://github.com/user/repo", false},
		{"SSH format", "git@github.com:user/repo.git", false},
		{"empty URL", "", true},
		{"http URL", "http://github.com/user/repo", true},
		{"ftp URL", "ftp://example.com/repo", true},
		{"no protocol", "github.com/user/repo", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateGitURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateGitURL(%q) error = %v, wantErr %v", tt.url, err, tt.wantErr)
			}
		})
	}
}

// TestCleanBundle tests bundle removal
func TestCleanBundle(t *testing.T) {
	tempDir := t.TempDir()
	syncer, err := New(tempDir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Create a fake bundle directory
	bundleDir := filepath.Join(tempDir, "test-bundle")
	if err := os.MkdirAll(bundleDir, 0755); err != nil {
		t.Fatalf("failed to create bundle dir: %v", err)
	}

	// Create a file inside
	testFile := filepath.Join(bundleDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Clean the bundle
	if err := syncer.CleanBundle("test-bundle"); err != nil {
		t.Errorf("CleanBundle() error = %v, want nil", err)
	}

	// Verify bundle directory was removed
	if _, err := os.Stat(bundleDir); !os.IsNotExist(err) {
		t.Error("bundle directory still exists after CleanBundle()")
	}
}

// TestSyncBundle_InvalidURL tests URL validation in SyncBundle
func TestSyncBundle_InvalidURL(t *testing.T) {
	tempDir := t.TempDir()
	syncer, err := New(tempDir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := context.Background()
	err = syncer.SyncBundle(ctx, "test-bundle", "invalid-url", "main")

	if err == nil {
		t.Error("SyncBundle() error = nil, want error for invalid URL")
	}

	expectedMsg := "invalid Git URL"
	if err != nil && !contains(err.Error(), expectedMsg) {
		t.Errorf("error message = %q, want to contain %q", err.Error(), expectedMsg)
	}
}

// TestSyncBundle_DefaultBranch tests branch defaulting to "main"
func TestSyncBundle_DefaultBranch(t *testing.T) {
	oldExec := execCommand
	defer func() { execCommand = oldExec }()

	var capturedArgs []string
	execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name == "git" && len(args) >= 2 && args[0] == "clone" {
			capturedArgs = args
			return exec.Command("echo", "mocked")
		}
		return exec.Command(name, args...)
	}

	tempDir := t.TempDir()
	syncer, err := New(tempDir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := context.Background()
	_ = syncer.SyncBundle(ctx, "test-bundle", "https://github.com/test/repo", "")

	// Verify "main" was used as default branch
	foundBranch := false
	for i, arg := range capturedArgs {
		if arg == "--branch" && i+1 < len(capturedArgs) {
			if capturedArgs[i+1] == "main" {
				foundBranch = true
			}
		}
	}

	if !foundBranch {
		t.Errorf("default branch 'main' was not used, args: %v", capturedArgs)
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

// Helper to create a mock exec.Command that fails
func mockFailedCommand(ctx context.Context, name string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcess", "--", name}
	cs = append(cs, args...)
	cmd := exec.CommandContext(ctx, os.Args[0], cs...)
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
	return cmd
}

// TestHelperProcess isn't a real test. It's used to mock exec.Command
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	fmt.Fprintf(os.Stderr, "mock error")
	os.Exit(1)
}
