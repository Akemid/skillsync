package gitauth

import (
	"context"
	"os"
	"os/exec"
	"testing"
)

// TestIsSSHURL verifies SSH URL detection
func TestIsSSHURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want bool
	}{
		{"git@github", "git@github.com:org/repo.git", true},
		{"git@gitlab", "git@gitlab.com:org/repo.git", true},
		{"https", "https://github.com/org/repo.git", false},
		{"git protocol", "git://github.com/org/repo.git", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsSSHURL(tt.url)
			if got != tt.want {
				t.Errorf("IsSSHURL(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}

// TestEnsureSSHKey_NoOp verifies that an empty keyPath results in no exec call
func TestEnsureSSHKey_NoOp(t *testing.T) {
	oldExec := execCommand
	defer func() { execCommand = oldExec }()

	called := false
	execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		called = true
		return exec.Command(name, args...)
	}

	err := EnsureSSHKey(context.Background(), "")
	if err != nil {
		t.Errorf("EnsureSSHKey(\"\") error = %v, want nil", err)
	}
	if called {
		t.Error("execCommand should NOT be called when keyPath is empty")
	}
}

// TestEnsureSSHKey_Success verifies ssh-add is called with the correct args and nil returned
func TestEnsureSSHKey_Success(t *testing.T) {
	oldExec := execCommand
	defer func() { execCommand = oldExec }()

	// Create a real temp file so os.Stat passes
	tmpFile, err := os.CreateTemp(t.TempDir(), "id_ed25519")
	if err != nil {
		t.Fatalf("creating temp key file: %v", err)
	}
	tmpFile.Close()
	keyPath := tmpFile.Name()

	var capturedName string
	var capturedArgs []string

	execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		capturedName = name
		capturedArgs = args
		return exec.Command("true")
	}

	err = EnsureSSHKey(context.Background(), keyPath)
	if err != nil {
		t.Errorf("EnsureSSHKey() error = %v, want nil", err)
	}

	if capturedName != "ssh-add" {
		t.Errorf("command name = %q, want %q", capturedName, "ssh-add")
	}
	if len(capturedArgs) != 1 || capturedArgs[0] != keyPath {
		t.Errorf("args = %v, want [%s]", capturedArgs, keyPath)
	}
}

// TestEnsureSSHKey_CommandArgs verifies the exact args passed to ssh-add
func TestEnsureSSHKey_CommandArgs(t *testing.T) {
	oldExec := execCommand
	defer func() { execCommand = oldExec }()

	// Create a real temp file so os.Stat passes
	tmpFile, err := os.CreateTemp(t.TempDir(), "deploy_key")
	if err != nil {
		t.Fatalf("creating temp key file: %v", err)
	}
	tmpFile.Close()
	keyPath := tmpFile.Name()

	var gotName string
	var gotArgs []string

	execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		gotName = name
		gotArgs = args
		return exec.Command("true")
	}

	_ = EnsureSSHKey(context.Background(), keyPath)

	if gotName != "ssh-add" {
		t.Errorf("name = %q, want ssh-add", gotName)
	}
	if len(gotArgs) != 1 || gotArgs[0] != keyPath {
		t.Errorf("args = %v, want [%s]", gotArgs, keyPath)
	}
}

// TestEnsureSSHKey_Failure verifies error contains "ssh-add failed" when command exits non-zero
func TestEnsureSSHKey_Failure(t *testing.T) {
	oldExec := execCommand
	defer func() { execCommand = oldExec }()

	// Create a real temp file so os.Stat passes — the mock will make ssh-add fail
	tmpFile, err := os.CreateTemp(t.TempDir(), "id_ed25519")
	if err != nil {
		t.Fatalf("creating temp key file: %v", err)
	}
	tmpFile.Close()
	keyPath := tmpFile.Name()

	execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.Command("sh", "-c", "echo 'error output' >&2; exit 1")
	}

	err = EnsureSSHKey(context.Background(), keyPath)
	if err == nil {
		t.Fatal("EnsureSSHKey() error = nil, want error when ssh-add fails")
	}
	if !containsStr(err.Error(), "ssh-add failed") {
		t.Errorf("error = %q, want to contain 'ssh-add failed'", err.Error())
	}
}

// TestWrapGitError covers all error patterns and nil returns
func TestWrapGitError(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		output      string
		wantNil     bool
		wantContain string
	}{
		{
			name:        "permission denied publickey",
			url:         "git@github.com:org/repo.git",
			output:      "Permission denied (publickey).",
			wantNil:     false,
			wantContain: "ssh-add",
		},
		{
			name:        "authentication failed",
			url:         "https://github.com/org/repo.git",
			output:      "Authentication failed for 'https://github.com/org/repo.git'",
			wantNil:     false,
			wantContain: "token",
		},
		{
			name:        "could not read username",
			url:         "https://github.com/org/repo.git",
			output:      "could not read Username: terminal prompts disabled",
			wantNil:     false,
			wantContain: "credential",
		},
		{
			name:        "host key verification failed",
			url:         "git@github.com:org/repo.git",
			output:      "Host key verification failed.",
			wantNil:     false,
			wantContain: "known_hosts",
		},
		{
			name:    "unrelated network timeout",
			url:     "https://github.com/org/repo.git",
			output:  "network timeout",
			wantNil: true,
		},
		{
			name:    "empty output",
			url:     "https://github.com/org/repo.git",
			output:  "",
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := WrapGitError(tt.url, tt.output)
			if tt.wantNil {
				if err != nil {
					t.Errorf("WrapGitError() = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatal("WrapGitError() = nil, want error")
			}
			if tt.wantContain != "" && !containsStr(err.Error(), tt.wantContain) {
				t.Errorf("error = %q, want to contain %q", err.Error(), tt.wantContain)
			}
		})
	}
}

// TestEnsureSSHKey_FileNotFound verifies that a nonexistent key path returns an error
// before ssh-add is ever called.
func TestEnsureSSHKey_FileNotFound(t *testing.T) {
	oldExec := execCommand
	defer func() { execCommand = oldExec }()

	sshAddCalled := false
	execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name == "ssh-add" {
			sshAddCalled = true
		}
		return exec.Command("true")
	}

	err := EnsureSSHKey(context.Background(), "/nonexistent/path/that/does/not/exist/id_ed25519")
	if err == nil {
		t.Fatal("EnsureSSHKey() error = nil, want error for nonexistent key path")
	}
	if !containsStr(err.Error(), "ssh key not found") {
		t.Errorf("error = %q, want to contain 'ssh key not found'", err.Error())
	}
	if sshAddCalled {
		t.Error("ssh-add should NOT be called when the key file does not exist")
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
