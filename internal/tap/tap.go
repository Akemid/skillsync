package tap

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Akemid/skillsync/internal/config"
	"github.com/Akemid/skillsync/internal/gitauth"
)

// execCommand is a variable to allow mocking in tests
var execCommand = exec.CommandContext

// Tapper manages git-based skill uploads to tap repositories
type Tapper struct {
	registryPath string
}

// New creates a Tapper with the given registry path (supports ~ expansion)
func New(registryPath string) (*Tapper, error) {
	expanded := config.ExpandPath(registryPath)
	abs, err := filepath.Abs(expanded)
	if err != nil {
		return nil, fmt.Errorf("resolving registry path: %w", err)
	}
	return &Tapper{registryPath: abs}, nil
}

// Upload clones the tap repo to a temp dir, copies the skill into skills/<skillName>/,
// commits, and pushes. The temp dir is always removed on return (defer).
// If the skill already exists in the tap and force is false, an error is returned.
func (t *Tapper) Upload(ctx context.Context, tap config.Tap, skillPath, skillName string, force bool) error {
	branch := tap.Branch
	if branch == "" {
		branch = "main"
	}

	if err := validateGitURL(tap.URL); err != nil {
		return err
	}

	// Load SSH key before any git operations if URL is SSH and key is provided
	if gitauth.IsSSHURL(tap.URL) && tap.SSHKey != "" {
		if err := gitauth.EnsureSSHKey(ctx, config.ExpandPath(tap.SSHKey)); err != nil {
			return err
		}
	}

	// Clone into a temporary directory
	tempDir, err := os.MkdirTemp("", ".skillsync-tap-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// git clone --branch <branch> --depth 1 <url> <tempDir>
	cloneCmd := execCommand(ctx, "git", "clone", "--branch", branch, "--depth", "1", tap.URL, tempDir)
	if out, err := cloneCmd.CombinedOutput(); err != nil {
		if authErr := gitauth.WrapGitError(tap.URL, string(out)); authErr != nil {
			return authErr
		}
		return fmt.Errorf("git clone failed: %w\nOutput: %s", err, out)
	}

	// Check whether skill already exists in the clone
	destSkillDir := filepath.Join(tempDir, "skills", skillName)
	if _, err := os.Stat(filepath.Join(destSkillDir, "SKILL.md")); err == nil {
		if !force {
			return fmt.Errorf("skill %q already exists in tap %q; use --force to overwrite", skillName, tap.Name)
		}
		// Remove existing skill dir before copying
		if err := os.RemoveAll(destSkillDir); err != nil {
			return fmt.Errorf("removing existing skill: %w", err)
		}
	}

	// Create destination directory and copy skill files
	if err := os.MkdirAll(destSkillDir, 0755); err != nil {
		return fmt.Errorf("creating skill dir in clone: %w", err)
	}
	if err := copySkillDir(skillPath, destSkillDir); err != nil {
		return fmt.Errorf("copying skill files: %w", err)
	}

	// git -C <tempDir> add .
	addCmd := execCommand(ctx, "git", "-C", tempDir, "add", ".")
	if out, err := addCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git add failed: %w\nOutput: %s", err, out)
	}

	// git -C <tempDir> commit -m "Add skill: <skillName>"
	commitCmd := execCommand(ctx, "git", "-C", tempDir, "commit", "-m", fmt.Sprintf("Add skill: %s", skillName))
	if out, err := commitCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git commit failed: %w\nOutput: %s", err, out)
	}

	// git -C <tempDir> push
	pushCmd := execCommand(ctx, "git", "-C", tempDir, "push")
	if out, err := pushCmd.CombinedOutput(); err != nil {
		if authErr := gitauth.WrapGitError(tap.URL, string(out)); authErr != nil {
			return authErr
		}
		return fmt.Errorf("git push failed: %w\nOutput: %s", err, out)
	}

	return nil
}

// validateGitURL checks that the URL is a valid git remote
func validateGitURL(url string) error {
	if url == "" {
		return fmt.Errorf("git URL cannot be empty")
	}
	if !strings.HasPrefix(url, "https://") &&
		!strings.HasPrefix(url, "git://") &&
		!strings.HasPrefix(url, "git@") &&
		!strings.HasPrefix(url, "file://") {
		return fmt.Errorf("invalid git URL %q (must start with https://, git://, git@, or file://)", url)
	}
	return nil
}

// copySkillDir recursively copies srcDir into dstDir
func copySkillDir(srcDir, dstDir string) error {
	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		dest := filepath.Join(dstDir, rel)
		if info.IsDir() {
			return os.MkdirAll(dest, info.Mode())
		}
		return copyFile(path, dest)
	})
}

// copyFile copies a single file from src to dst
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}
