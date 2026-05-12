# Design: private-repo-auth

## 1. Package Placement Decision

**Decision: New `internal/gitauth` package.**

### Import graph analysis (current)

```
main.go → config, registry, detector, tui, installer, sync, tap, archive, skillasset
sync    → stdlib only (no config, no other internal)
tap     → config (for config.Tap, config.ExpandPath)
```

### Proposed import graph addition

```
sync    → gitauth (new dep, stdlib-only package — no cycle)
tap     → gitauth (new dep, stdlib-only package — no cycle)
gitauth → stdlib only (os/exec, context, fmt, strings, os)
```

**No circular dependency risk.** `gitauth` imports zero internal packages. Both `sync` and `tap` can safely import it.

### Alternatives rejected

| Alternative | Why rejected |
|---|---|
| Duplicate `EnsureSSHKey`/`WrapGitError` in sync + tap | SSH agent interaction is ~30 lines with error detection patterns. Duplicating auth logic (not just validation) is a maintenance trap — auth failure patterns will evolve and diverge. `validateGitURL` duplication was acceptable (8 lines, stable). |
| Put in `main.go` (coordinator) | Violates coordinator pattern. `main.go` orchestrates; it doesn't own domain logic. Would also force `main.go` to thread auth state into both Syncer and Tapper calls, adding coupling. |
| Put in `config` package | `config` is data + YAML parsing. Adding exec calls and SSH logic violates single responsibility. |

## 2. `EnsureSSHKey` — Signature and Implementation

```go
package gitauth

// EnsureSSHKey adds the given SSH key to the running ssh-agent.
// No-op if keyPath is empty. Returns an actionable error if ssh-add fails.
func EnsureSSHKey(ctx context.Context, keyPath string) error
```

### Implementation approach

1. If `keyPath == ""` → return nil (zero-value = disabled)
2. Expand `~` in keyPath (inline, same logic as `config.ExpandPath` — duplicated to avoid importing config)
3. Validate file exists: `os.Stat(expanded)` → wrap with "ssh key not found: %s"
4. Run `execCommand(ctx, "ssh-add", expanded)` capturing combined output
5. On error → inspect output for known patterns, return wrapped error

```go
var execCommand = exec.CommandContext // testable

func EnsureSSHKey(ctx context.Context, keyPath string) error {
    if keyPath == "" {
        return nil
    }
    expanded := expandHome(keyPath)
    if _, err := os.Stat(expanded); err != nil {
        return fmt.Errorf("ssh key not found at %s: %w", expanded, err)
    }
    cmd := execCommand(ctx, "ssh-add", expanded)
    output, err := cmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("ssh-add failed for %s: %w\noutput: %s", expanded, err, output)
    }
    return nil
}
```

**Note**: `expandHome` is a private 6-line function duplicated from config.ExpandPath to avoid importing config. This keeps gitauth at zero internal deps.

## 3. `WrapGitError` — Signature, Patterns, Messages

```go
// WrapGitError inspects git command output for auth failure patterns
// and returns an actionable error. If no auth pattern matches, returns nil.
func WrapGitError(url, output string) error
```

### Pattern detection table

| Pattern (case-insensitive substring) | Diagnosis | User message |
|---|---|---|
| `Permission denied (publickey)` | SSH key not accepted | `"authentication failed for %s: SSH key rejected — check that the correct key is added to the remote (ssh_key in config) and that the deploy key has read access"` |
| `Could not read from remote repository` | General SSH auth failure | `"cannot access %s: ensure the SSH key is authorized and the repository URL is correct"` |
| `Repository not found` | HTTPS 404 or private repo | `"repository %s not found: verify the URL and that your credentials have access to this private repository"` |
| `fatal: Authentication failed` | HTTPS credential failure | `"HTTPS authentication failed for %s: check your git credential helper or switch to SSH (git@...) with ssh_key"` |
| `Host key verification failed` | Unknown host | `"SSH host verification failed for %s: run 'ssh-keyscan <host> >> ~/.ssh/known_hosts' or verify the remote host"` |

### Implementation

```go
func WrapGitError(url, output string) error {
    lower := strings.ToLower(output)
    for _, p := range authPatterns {
        if strings.Contains(lower, p.pattern) {
            return fmt.Errorf(p.template, url)
        }
    }
    return nil // not an auth error
}
```

`authPatterns` is a package-level `[]struct{pattern, template string}` slice — table-driven, easy to extend.

## 4. Integration Points — sync.go and tap.go

### sync.go — `SyncBundle`

```go
import "github.com/Akemid/skillsync/internal/gitauth"

// SyncBundle signature changes: add sshKey parameter
func (s *Syncer) SyncBundle(ctx context.Context, bundleName, url, branch, sshKey string) error {
    // ... existing validation ...

    // NEW: before any git operation, ensure SSH key is loaded
    if strings.HasPrefix(url, "git@") && sshKey != "" {
        if err := gitauth.EnsureSSHKey(ctx, sshKey); err != nil {
            return fmt.Errorf("preparing auth for bundle %q: %w", bundleName, err)
        }
    }

    // ... existing clone/pull logic ...
    // On clone/pull error, wrap with gitauth.WrapGitError:
    output, err := cmd.CombinedOutput()
    if err != nil {
        if authErr := gitauth.WrapGitError(url, string(output)); authErr != nil {
            return authErr
        }
        return fmt.Errorf("git clone failed: %w\nOutput: %s", err, output)
    }
}
```

**Placement**: `EnsureSSHKey` is called ONCE at the top of `SyncBundle`, before `cloneBundle` or `pullBundle`. Both sub-operations benefit from the key being in the agent.

**`WrapGitError`**: called at each `CombinedOutput()` error site (clone and pull). If it returns non-nil, that replaces the generic error.

### tap.go — `Upload`

Same pattern. `EnsureSSHKey` called once at top of `Upload`, before the clone. `WrapGitError` wraps errors from clone and push operations.

```go
func (t *Tapper) Upload(ctx context.Context, tap config.Tap, skillPath, skillName string, force bool) error {
    // NEW: auth check
    if strings.HasPrefix(tap.URL, "git@") && tap.SSHKey != "" {
        if err := gitauth.EnsureSSHKey(ctx, tap.SSHKey); err != nil {
            return fmt.Errorf("preparing auth for tap %q: %w", tap.Name, err)
        }
    }
    // ... existing logic, with WrapGitError at error sites ...
}
```

## 5. Config Struct Changes

### Bundle.Source

```go
type Source struct {
    Type   string `yaml:"type"`
    URL    string `yaml:"url"`
    Branch string `yaml:"branch"`
    Path   string `yaml:"path"`
    SSHKey string `yaml:"ssh_key,omitempty"` // NEW: path to SSH private key for auth
}
```

### Tap

```go
type Tap struct {
    Name   string `yaml:"name"`
    URL    string `yaml:"url"`
    Branch string `yaml:"branch"`
    SSHKey string `yaml:"ssh_key,omitempty"` // NEW: path to SSH private key for auth
}
```

**Zero-value behavior**: `SSHKey == ""` means no SSH key management — git uses default SSH config. This is fully backward-compatible. Existing configs without `ssh_key` work identically to today.

### YAML example

```yaml
bundles:
  - name: private-skills
    source:
      type: git
      url: git@github.com:company/private-skills.git
      branch: main
      ssh_key: ~/.ssh/company_deploy_key

taps:
  - name: company-tap
    url: git@github.com:company/skill-tap.git
    branch: main
    ssh_key: ~/.ssh/company_deploy_key
```

## 6. Testing Strategy — `execCommand` Mock Pattern

### gitauth package tests

```go
// gitauth/gitauth_test.go
func TestEnsureSSHKey_addsKey(t *testing.T) {
    // Override execCommand
    oldExec := execCommand
    defer func() { execCommand = oldExec }()

    execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
        // Verify ssh-add is called with the right key
        cs := []string{"-test.run=TestHelperProcess", "--", name}
        cs = append(cs, args...)
        cmd := exec.Command(os.Args[0], cs...)
        cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
        return cmd
    }
    // ... assert no error, assert correct args passed ...
}
```

Uses the standard Go `TestHelperProcess` pattern (same as sync and tap tests likely use).

### Table-driven tests for WrapGitError

```go
func TestWrapGitError(t *testing.T) {
    tests := []struct {
        name    string
        url     string
        output  string
        wantNil bool
        wantMsg string
    }{
        {"ssh permission denied", "git@github.com:co/repo", "Permission denied (publickey)", false, "SSH key rejected"},
        {"https auth failed", "https://github.com/co/repo", "fatal: Authentication failed", false, "HTTPS authentication"},
        {"unrelated error", "git@host:repo", "network unreachable", true, ""},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := WrapGitError(tt.url, tt.output)
            if tt.wantNil && err != nil { t.Fatalf(...) }
            if !tt.wantNil && !strings.Contains(err.Error(), tt.wantMsg) { t.Fatalf(...) }
        })
    }
}
```

### Integration in sync_test.go and tap_test.go

Existing tests that mock `execCommand` continue to work — `gitauth.EnsureSSHKey` is only called when `sshKey != ""`, so existing tests with empty sshKey skip it entirely. New tests add cases with `sshKey` set and a mock that validates `ssh-add` was called before `git clone`.

## 7. Edge Cases (Noted, Not Solved in v1)

| Edge case | Notes |
|---|---|
| `SSH_AUTH_SOCK` unset | `ssh-add` will fail with "Could not open connection to agent". `EnsureSSHKey` error message should be clear enough. Future: detect and suggest `eval $(ssh-agent)`. |
| macOS Keychain integration | macOS ssh-add supports `--apple-use-keychain`. Not needed for v1 — standard `ssh-add` works. |
| Windows | OpenSSH on Windows uses a service (`ssh-agent`), not a socket. `ssh-add` may or may not work depending on setup. Document as "best-effort on Windows". |
| Key already loaded | `ssh-add` is idempotent — adding an already-loaded key is a no-op. No special handling needed. |
| Passphrase-protected keys | `ssh-add` will prompt for passphrase on stdin. In non-interactive CLI mode this will hang. Future: detect TTY, warn user. For v1, document that deploy keys should be passphrase-less or pre-loaded. |
| HTTPS URLs with ssh_key set | `EnsureSSHKey` is gated on `strings.HasPrefix(url, "git@")`. HTTPS URLs with ssh_key set are silently ignored — correct behavior, SSH keys don't apply to HTTPS. |
| Key path with spaces | `expandHome` + `os.Stat` handle this. `ssh-add` receives the path as a single argument (not shell-expanded), so spaces are safe. |

## 8. File Inventory

| File | Action |
|---|---|
| `internal/gitauth/gitauth.go` | **CREATE** — `EnsureSSHKey`, `WrapGitError`, `expandHome`, `execCommand` var |
| `internal/gitauth/gitauth_test.go` | **CREATE** — table-driven tests for both functions |
| `internal/config/config.go` | **MODIFY** — add `SSHKey` field to `Source` and `Tap` structs |
| `internal/sync/sync.go` | **MODIFY** — import gitauth, add `sshKey` param to `SyncBundle`, call `EnsureSSHKey` + `WrapGitError` |
| `internal/tap/tap.go` | **MODIFY** — import gitauth, call `EnsureSSHKey` + `WrapGitError` in `Upload` |
| `cmd/skillsync/main.go` | **MODIFY** — pass `bundle.Source.SSHKey` to `SyncBundle` calls |
| `internal/sync/sync_test.go` | **MODIFY** — add test cases with sshKey |
| `internal/tap/tap_test.go` | **MODIFY** — add test cases with sshKey |
