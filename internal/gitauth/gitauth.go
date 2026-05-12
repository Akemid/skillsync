package gitauth

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// execCommand is a variable to allow mocking in tests
var execCommand = exec.CommandContext

// SetExecCommand replaces the package-level execCommand and returns the previous value.
// Intended for use in tests that need to mock ssh-add from other packages.
func SetExecCommand(fn func(context.Context, string, ...string) *exec.Cmd) func(context.Context, string, ...string) *exec.Cmd {
	old := execCommand
	execCommand = fn
	return old
}

// IsSSHURL returns true if the URL is an SSH git URL (starts with "git@").
func IsSSHURL(url string) bool {
	return strings.HasPrefix(url, "git@")
}

// EnsureSSHKey adds the given SSH key to the agent via ssh-add.
// If keyPath is empty, it is a no-op and returns nil.
func EnsureSSHKey(ctx context.Context, keyPath string) error {
	if keyPath == "" {
		return nil
	}

	if _, err := os.Stat(keyPath); err != nil {
		return fmt.Errorf("ssh key not found at %s: %w", keyPath, err)
	}

	cmd := execCommand(ctx, "ssh-add", keyPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ssh-add failed: %w\nOutput: %s", err, output)
	}
	return nil
}

// WrapGitError inspects git command output for known authentication error patterns
// and returns a descriptive error. Returns nil if no known pattern matches.
func WrapGitError(url, output string) error {
	patterns := []struct {
		pattern string
		message string
	}{
		{
			pattern: "Permission denied (publickey)",
			message: "SSH authentication failed: run `ssh-add <path/to/key>` to load your key, or set ssh_key in config",
		},
		{
			pattern: "Authentication failed",
			message: "git authentication failed: check your token or credential helper configuration",
		},
		{
			pattern: "could not read Username",
			message: "git credential error: configure a credential helper or use an SSH URL with ssh_key in config",
		},
		{
			pattern: "Host key verification failed",
			message: "SSH host key verification failed: add the host to ~/.ssh/known_hosts (run: ssh-keyscan github.com >> ~/.ssh/known_hosts)",
		},
	}

	for _, p := range patterns {
		if strings.Contains(output, p.pattern) {
			return fmt.Errorf("%s (url: %s)", p.message, url)
		}
	}
	return nil
}
