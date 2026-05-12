package sync

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Akemid/skillsync/internal/config"
	"github.com/Akemid/skillsync/internal/gitauth"
)

// execCommand is a variable to allow mocking in tests
var execCommand = exec.CommandContext

// execLookPath is a variable to allow mocking exec.LookPath in tests
var execLookPath = exec.LookPath

// Syncer manages Git-based bundle synchronization
type Syncer struct {
	remoteBaseDir string // e.g., ~/.agents/skills/_remote
}

// New creates a Syncer that manages bundles in remoteBaseDir
// remoteBaseDir should be {registry_path}/_remote
func New(remoteBaseDir string) (*Syncer, error) {
	abs, err := filepath.Abs(remoteBaseDir)
	if err != nil {
		return nil, fmt.Errorf("resolving remote dir: %w", err)
	}
	return &Syncer{remoteBaseDir: abs}, nil
}

// SyncBundle clones (first time) or pulls (subsequent) a bundle
// Uses atomic operations: clone to temp → move to final location
func (s *Syncer) SyncBundle(ctx context.Context, bundleName, url, branch, sshKey string) error {
	if branch == "" {
		branch = "main"
	}

	// Check git binary is available before doing anything
	if _, err := execLookPath("git"); err != nil {
		return fmt.Errorf("git is not installed or not in PATH: install git and retry")
	}

	// Validate URL format
	if err := validateGitURL(url); err != nil {
		return err
	}

	// Load SSH key before any git operations if URL is SSH and key is provided
	if gitauth.IsSSHURL(url) && sshKey != "" {
		if err := gitauth.EnsureSSHKey(ctx, config.ExpandPath(sshKey)); err != nil {
			return err
		}
	}

	targetDir := filepath.Join(s.remoteBaseDir, bundleName)

	// Check if bundle already exists
	if _, err := os.Stat(filepath.Join(targetDir, ".git")); err == nil {
		// Existing repo: git pull --ff-only
		return s.pullBundle(ctx, url, targetDir)
	}

	// New bundle: atomic clone via temp dir
	return s.cloneBundle(ctx, url, branch, targetDir)
}

// cloneBundle performs atomic Git clone operation
func (s *Syncer) cloneBundle(ctx context.Context, url, branch, targetDir string) error {
	// Ensure _remote/ exists
	if err := os.MkdirAll(s.remoteBaseDir, 0755); err != nil {
		return fmt.Errorf("creating remote dir: %w", err)
	}

	// Clone to temp dir first (atomic operation)
	tempDir, err := os.MkdirTemp(s.remoteBaseDir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir) // Cleanup on failure

	cmd := execCommand(ctx, "git", "clone", "--branch", branch, "--depth", "1", url, tempDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if authErr := gitauth.WrapGitError(url, string(output)); authErr != nil {
			return authErr
		}
		return fmt.Errorf("git clone failed: %w\nOutput: %s", err, output)
	}

	// Atomic move to final location
	if err := os.Rename(tempDir, targetDir); err != nil {
		return fmt.Errorf("moving to final location: %w", err)
	}

	return nil
}

// pullBundle updates an existing Git repository
func (s *Syncer) pullBundle(ctx context.Context, url, targetDir string) error {
	// Check for local modifications before pulling
	statusCmd := execCommand(ctx, "git", "-C", targetDir, "status", "--porcelain")
	statusOutput, err := statusCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git status failed: %w\nOutput: %s", err, statusOutput)
	}
	if len(strings.TrimSpace(string(statusOutput))) > 0 {
		return fmt.Errorf("bundle %q has local modifications — remove them or run: skillsync sync --clean", targetDir)
	}

	cmd := execCommand(ctx, "git", "-C", targetDir, "pull", "--ff-only")
	output, err := cmd.CombinedOutput()
	if err != nil {
		if authErr := gitauth.WrapGitError(url, string(output)); authErr != nil {
			return authErr
		}
		return fmt.Errorf("git pull failed: %w\nOutput: %s", err, output)
	}
	return nil
}

// CleanBundle removes a synced bundle (for manual cleanup)
func (s *Syncer) CleanBundle(bundleName string) error {
	targetDir := filepath.Join(s.remoteBaseDir, bundleName)
	return os.RemoveAll(targetDir)
}

// validateGitURL checks if a URL is a valid Git URL
func validateGitURL(url string) error {
	if url == "" {
		return fmt.Errorf("Git URL cannot be empty")
	}
	if !strings.HasPrefix(url, "https://") &&
		!strings.HasPrefix(url, "git://") &&
		!strings.HasPrefix(url, "git@") &&
		!strings.HasPrefix(url, "file://") {
		return fmt.Errorf("invalid Git URL %q (must start with https://, git://, git@, or file://)", url)
	}
	return nil
}
