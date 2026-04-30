package tap

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/Akemid/skillsync/internal/config"
)

// callLog records which git subcommands were invoked by the mock
type callLog struct {
	cmds []string
}

// mockExec returns an execCommand mock. For each git invocation it appends the
// subcommand to log and returns the given cmd.
func successExec(log *callLog) func(ctx context.Context, name string, args ...string) *exec.Cmd {
	return func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name == "git" && len(args) > 0 {
			switch args[0] {
			case "clone":
				log.cmds = append(log.cmds, "clone")
			case "-C":
				if len(args) > 2 {
					log.cmds = append(log.cmds, args[2])
				}
			}
		}
		return exec.Command("true")
	}
}

// TestUpload_Success verifies the happy-path: clone → copy → commit → push all
// succeed and the temp directory is cleaned up afterwards.
func TestUpload_Success(t *testing.T) {
	oldExec := execCommand
	defer func() { execCommand = oldExec }()

	registryPath := t.TempDir()

	// Create a real skill dir to copy
	skillDir := filepath.Join(registryPath, "my-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: my-skill\n---\n"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	var log callLog
	execCommand = successExec(&log)

	tapper, err := New(registryPath)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	tap := config.Tap{Name: "remote-tap", URL: "https://github.com/user/tap.git", Branch: "main"}
	ctx := context.Background()

	if err := tapper.Upload(ctx, tap, skillDir, "my-skill", false); err != nil {
		t.Fatalf("Upload() error = %v, want nil", err)
	}

	// Verify clone was called
	foundClone := false
	for _, c := range log.cmds {
		if c == "clone" {
			foundClone = true
		}
	}
	if !foundClone {
		t.Error("expected git clone to be called")
	}

	// Verify no .skillsync-tap-* temp dirs left behind
	entries, _ := os.ReadDir(os.TempDir())
	const tapPrefix = ".skillsync-tap-"
	for _, e := range entries {
		if e.IsDir() && len(e.Name()) >= len(tapPrefix) && e.Name()[:len(tapPrefix)] == tapPrefix {
			t.Errorf("temp dir %q left behind after successful upload", e.Name())
		}
	}
}

// TestUpload_SkillAlreadyExists_NoForce verifies that if skills/<name>/SKILL.md
// already exists in the cloned repo and force=false, Upload returns an error
// containing "already exists".
func TestUpload_SkillAlreadyExists_NoForce(t *testing.T) {
	oldExec := execCommand
	defer func() { execCommand = oldExec }()

	registryPath := t.TempDir()

	// Create a real skill dir
	skillDir := filepath.Join(registryPath, "my-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: my-skill\n---\n"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Mock that simulates a clone by writing the conflict file into the temp dir
	execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name == "git" && len(args) > 0 && args[0] == "clone" {
			// The last arg is the temp dir destination
			cloneTarget := args[len(args)-1]
			conflictPath := filepath.Join(cloneTarget, "skills", "my-skill")
			_ = os.MkdirAll(conflictPath, 0755)
			_ = os.WriteFile(filepath.Join(conflictPath, "SKILL.md"), []byte("existing"), 0644)
		}
		return exec.Command("true")
	}

	tapper, err := New(registryPath)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	tap := config.Tap{Name: "remote-tap", URL: "https://github.com/user/tap.git", Branch: "main"}
	ctx := context.Background()

	err = tapper.Upload(ctx, tap, skillDir, "my-skill", false)
	if err == nil {
		t.Fatal("Upload() error = nil, want error when skill already exists")
	}
	if !containsStr(err.Error(), "already exists") {
		t.Errorf("error = %q, want to contain 'already exists'", err.Error())
	}
}

// TestUpload_SkillAlreadyExists_Force verifies that with force=true the upload
// proceeds even when the skill already exists in the clone.
func TestUpload_SkillAlreadyExists_Force(t *testing.T) {
	oldExec := execCommand
	defer func() { execCommand = oldExec }()

	registryPath := t.TempDir()

	skillDir := filepath.Join(registryPath, "my-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: my-skill\n---\n"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name == "git" && len(args) > 0 && args[0] == "clone" {
			cloneTarget := args[len(args)-1]
			conflictPath := filepath.Join(cloneTarget, "skills", "my-skill")
			_ = os.MkdirAll(conflictPath, 0755)
			_ = os.WriteFile(filepath.Join(conflictPath, "SKILL.md"), []byte("existing"), 0644)
		}
		return exec.Command("true")
	}

	tapper, err := New(registryPath)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	tap := config.Tap{Name: "remote-tap", URL: "https://github.com/user/tap.git", Branch: "main"}
	ctx := context.Background()

	if err := tapper.Upload(ctx, tap, skillDir, "my-skill", true); err != nil {
		t.Fatalf("Upload() with force=true error = %v, want nil", err)
	}
}

// TestUpload_PushFails_NoResidue verifies that when git push fails the temp
// directory is removed and an error is returned.
func TestUpload_PushFails_NoResidue(t *testing.T) {
	oldExec := execCommand
	defer func() { execCommand = oldExec }()

	registryPath := t.TempDir()

	skillDir := filepath.Join(registryPath, "my-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: my-skill\n---\n"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	var capturedTempDir string
	execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name == "git" && len(args) > 0 {
			switch args[0] {
			case "clone":
				// Record temp dir so we can check it after
				capturedTempDir = args[len(args)-1]
				return exec.Command("true")
			case "-C":
				if len(args) > 2 && args[2] == "push" {
					// Simulate push failure
					return exec.Command("sh", "-c", "exit 1")
				}
				return exec.Command("true")
			}
		}
		return exec.Command("true")
	}

	tapper, err := New(registryPath)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	tap := config.Tap{Name: "remote-tap", URL: "https://github.com/user/tap.git", Branch: "main"}
	ctx := context.Background()

	err = tapper.Upload(ctx, tap, skillDir, "my-skill", false)
	if err == nil {
		t.Fatal("Upload() error = nil, want error when push fails")
	}

	// Verify temp dir was cleaned up
	if capturedTempDir != "" {
		if _, statErr := os.Stat(capturedTempDir); !os.IsNotExist(statErr) {
			t.Errorf("temp dir %q still exists after push failure — residue not cleaned up", capturedTempDir)
		}
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
