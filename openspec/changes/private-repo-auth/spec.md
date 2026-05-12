# Spec: private-repo-auth

**Status**: complete
**Created**: 2026-05-07
**Phase**: spec
**Depends on**: proposal.md

---

## 1. Overview

This spec defines the detailed requirements and acceptance criteria for adding private repository authentication to skillsync. The change touches three areas: config struct additions, a new `internal/gitauth` package, integration of auth into `sync` and `tap`, and SKILL.md documentation. All changes are backward-compatible — the new `ssh_key` field is optional with `omitempty`.

---

## 2. Config Struct Changes

### 2.1 Bundle.SSHKey field

**File**: `internal/config/config.go`

**Requirement**: Add `SSHKey string` field to the `Bundle` struct with YAML tag `ssh_key,omitempty`.

```go
type Bundle struct {
    Name        string     `yaml:"name"`
    Description string     `yaml:"description,omitempty"`
    Company     string     `yaml:"company,omitempty"`
    Tags        []string   `yaml:"tags,omitempty"`
    Tech        []string   `yaml:"tech,omitempty"`
    Source      *Source    `yaml:"source,omitempty"`
    Skills      []SkillRef `yaml:"skills"`
    SSHKey      string     `yaml:"ssh_key,omitempty"`
}
```

**Acceptance Criteria**:

- AC-CFG-1: A bundle YAML entry with `ssh_key: ~/.ssh/id_ed25519_work` parses correctly — `Bundle.SSHKey` equals `"~/.ssh/id_ed25519_work"`.
- AC-CFG-2: A bundle YAML entry without `ssh_key` parses correctly — `Bundle.SSHKey` equals `""`.
- AC-CFG-3: When marshaling a `Bundle` with `SSHKey == ""`, the `ssh_key` key is absent from the YAML output (omitempty).
- AC-CFG-4: When marshaling a `Bundle` with `SSHKey == "~/.ssh/id_ed25519_work"`, the key is present in YAML output.
- AC-CFG-5: Existing config files without `ssh_key` continue to load without error. No breaking change.

### 2.2 Tap.SSHKey field

**File**: `internal/config/config.go`

**Requirement**: Add `SSHKey string` field to the `Tap` struct with YAML tag `ssh_key,omitempty`.

```go
type Tap struct {
    Name   string `yaml:"name"`
    URL    string `yaml:"url"`
    Branch string `yaml:"branch"`
    SSHKey string `yaml:"ssh_key,omitempty"`
}
```

**Acceptance Criteria**:

- AC-CFG-6: A tap YAML entry with `ssh_key: ~/.ssh/id_ed25519_work` parses correctly — `Tap.SSHKey` equals `"~/.ssh/id_ed25519_work"`.
- AC-CFG-7: A tap YAML entry without `ssh_key` parses correctly — `Tap.SSHKey` equals `""`.
- AC-CFG-8: When marshaling a `Tap` with `SSHKey == ""`, the `ssh_key` key is absent from the YAML output.
- AC-CFG-9: Existing round-trip test `TestTap_RoundTrip` continues to pass unchanged.

### 2.3 Test requirements (config_test.go additions)

Table-driven test `TestBundle_SSHKey_Field` must cover:

| case | yaml input | expected SSHKey |
|------|-----------|-----------------|
| with ssh_key | `ssh_key: ~/.ssh/id_ed25519_work` | `~/.ssh/id_ed25519_work` |
| without ssh_key | (field absent) | `""` |
| empty ssh_key | `ssh_key: ""` | `""` |

Table-driven test `TestTap_SSHKey_Field` must cover the same three cases for `Tap`.

Test `TestBundle_SSHKey_OmitEmpty` verifies that a Bundle with empty SSHKey marshals to YAML without the `ssh_key` key.

---

## 3. New Package: `internal/gitauth`

### 3.1 Package structure

**Files**:
- `internal/gitauth/gitauth.go` — implementation
- `internal/gitauth/gitauth_test.go` — tests

**Package constraint**: `gitauth` MUST NOT import any other internal skillsync package. It receives all inputs as plain strings. Path expansion (`~` → home dir) is the caller's responsibility.

### 3.2 `IsSSHURL(url string) bool`

**Requirement**: Returns `true` if and only if `url` starts with `"git@"`.

**Acceptance Criteria**:

| input | expected |
|-------|----------|
| `"git@github.com:org/repo.git"` | `true` |
| `"git@gitlab.com:org/repo.git"` | `true` |
| `"https://github.com/org/repo"` | `false` |
| `"git://github.com/org/repo"` | `false` |
| `"file:///local/path"` | `false` |
| `""` | `false` |

- AC-GA-1: `IsSSHURL` is a pure function with no side effects.
- AC-GA-2: The `"git@"` prefix check is case-sensitive (no normalization needed — git URLs are lowercase by convention).

### 3.3 `EnsureSSHKey(ctx context.Context, keyPath string) error`

**Requirement**: Runs `ssh-add <keyPath>` via the `execCommand` variable. Used to load a key into the SSH agent before git operations.

**Behavior matrix**:

| condition | expected behavior |
|-----------|-------------------|
| `keyPath == ""` | return `nil` immediately (no-op) |
| `keyPath` is non-empty (any URL type) | run `ssh-add <keyPath>` |
| `ssh-add` exits 0 | return `nil` |
| `ssh-add` exits non-zero | return wrapped error: `fmt.Errorf("ssh-add failed: %w\nOutput: %s", err, output)` |

**Acceptance Criteria**:

- AC-GA-3: When `keyPath == ""`, `EnsureSSHKey` returns `nil` without calling `execCommand`.
- AC-GA-4: When `keyPath` is non-empty and `ssh-add` succeeds, `EnsureSSHKey` returns `nil`.
- AC-GA-5: When `keyPath` is non-empty and `ssh-add` fails, the error message contains `"ssh-add failed"`.
- AC-GA-6: The command executed is exactly `ssh-add <keyPath>` — no extra flags.
- AC-GA-7: `execCommand` is a package-level `var` (same pattern as `sync.execCommand` and `tap.execCommand`) to enable test mocking without an interface.
- AC-GA-8: `EnsureSSHKey` uses `cmd.CombinedOutput()` so stderr is captured in the error message.

**Note on caller responsibility**: The caller (sync/tap) MUST call `config.ExpandPath` on `SSHKey` before passing it to `EnsureSSHKey`. `gitauth` does not expand `~`.

### 3.4 `WrapGitError(url, output string) error`

**Requirement**: Pattern-matches known git authentication failure strings in `output` and returns an actionable error message. Returns `nil` if `output` contains no recognized auth failure pattern (the caller retains its own error wrapping for the non-auth case).

**Design rationale**: Returns `nil` for non-matches rather than wrapping a passed-in error. This keeps the function signature simple and avoids double-wrapping. The caller checks: `if authErr := gitauth.WrapGitError(url, output); authErr != nil { return authErr }` before its own error wrapping.

**Pattern table**:

| output contains | URL type | returned error message must contain |
|-----------------|----------|--------------------------------------|
| `"Permission denied (publickey)"` | SSH (`git@`) | `"ssh-add"` AND key suggestion |
| `"Permission denied (publickey)"` | SSH (`git@`) | `"ssh -T git@github.com"` (or equivalent connectivity test suggestion) |
| `"Authentication failed"` | HTTPS | `"token"` OR `"credential"` |
| `"could not read Username"` | HTTPS | `"credential"` OR `"SSH"` |
| `"Host key verification failed"` | any | `"known_hosts"` |
| (no pattern matches) | any | return `nil` |

**Acceptance Criteria**:

- AC-GA-9: `WrapGitError` with output containing `"Permission denied (publickey)"` and SSH URL returns non-nil error mentioning `ssh-add`.
- AC-GA-10: `WrapGitError` with output containing `"Authentication failed"` and HTTPS URL returns non-nil error mentioning a token or credential helper.
- AC-GA-11: `WrapGitError` with output containing `"could not read Username"` returns non-nil error with actionable guidance.
- AC-GA-12: `WrapGitError` with output containing `"Host key verification failed"` returns non-nil error mentioning `known_hosts`.
- AC-GA-13: `WrapGitError` with unrecognized output (e.g., `"network timeout"`) returns `nil`.
- AC-GA-14: `WrapGitError` with empty output returns `nil`.
- AC-GA-15: Pattern matching uses `strings.Contains` (case-sensitive) — no regex. Error messages from git are stable and well-known.
- AC-GA-16: `WrapGitError` is a pure function with no side effects.

### 3.5 Test requirements (gitauth_test.go)

All tests use table-driven subtests (`t.Run`). The `execCommand` var is saved/restored with `defer` in every test that mocks it.

**`TestIsSSHURL`**: table-driven, covers all cases in section 3.2.

**`TestEnsureSSHKey_NoOp`**: `keyPath == ""` — verifies `execCommand` is NOT called. Use a flag variable in the mock.

**`TestEnsureSSHKey_Success`**: `keyPath` non-empty, mock `ssh-add` returns exit 0 — verifies return is `nil`.

**`TestEnsureSSHKey_Failure`**: `keyPath` non-empty, mock `ssh-add` returns exit 1 with stderr `"Could not open a connection to your authentication agent"` — verifies error contains `"ssh-add failed"`.

**`TestEnsureSSHKey_CommandArgs`**: verifies the exact args passed to `execCommand` are `["ssh-add", keyPath]`.

**`TestWrapGitError`**: table-driven test covering all pattern rows in section 3.4 plus the nil-return cases.

---

## 4. sync.SyncBundle Integration

### 4.1 Signature change

**File**: `internal/sync/sync.go`

**Requirement**: `SyncBundle` must accept an `sshKey string` parameter.

```go
func (s *Syncer) SyncBundle(ctx context.Context, bundleName, url, branch, sshKey string) error
```

**Acceptance Criteria**:

- AC-SYNC-1: When `sshKey` is non-empty AND `url` starts with `"git@"`, `EnsureSSHKey` is called with the expanded key path before any git operation.
- AC-SYNC-2: When `sshKey` is non-empty AND `url` is HTTPS, `EnsureSSHKey` is NOT called.
- AC-SYNC-3: When `sshKey` is empty, `EnsureSSHKey` is NOT called regardless of URL type.
- AC-SYNC-4: If `EnsureSSHKey` returns an error, `SyncBundle` returns that error immediately without running git commands.
- AC-SYNC-5: Path expansion (`~` → home) is applied to `sshKey` before passing to `EnsureSSHKey`. Use `config.ExpandPath`.
- AC-SYNC-6: For git errors (clone/pull), `WrapGitError` is checked first. If non-nil, that error is returned. Otherwise the original wrapped error is returned.
- AC-SYNC-7: Existing callers in `cmd/skillsync/main.go` must pass `config.ExpandPath(bundle.Source.SSHKey)` — or the raw `bundle.SSHKey` field (expansion happens inside SyncBundle per AC-SYNC-5).
- AC-SYNC-8: `execLookPath` guard (git binary check) runs before `EnsureSSHKey`.

### 4.2 Test requirements (sync_test.go additions)

**`TestSyncBundle_SSHKey_LoadedBeforeClone`**: SSH URL + non-empty sshKey → mock verifies `ssh-add` is invoked before `git clone`.

**`TestSyncBundle_SSHKey_SkippedForHTTPS`**: HTTPS URL + non-empty sshKey → mock verifies `ssh-add` is NOT invoked.

**`TestSyncBundle_SSHKey_EmptyNoOp`**: SSH URL + empty sshKey → mock verifies `ssh-add` is NOT invoked.

**`TestSyncBundle_SSHKey_ErrorPropagates`**: `ssh-add` fails → `SyncBundle` returns error without calling `git clone`.

**`TestSyncBundle_AuthError_Wrapped`**: mock `git clone` outputs `"Permission denied (publickey)"` → returned error contains `"ssh-add"`.

All new tests follow existing mock pattern: `oldExec := execCommand; defer func() { execCommand = oldExec }()`.

---

## 5. tap.Upload Integration

### 5.1 Tap struct is already passed in

**File**: `internal/tap/tap.go`

**Current signature**:
```go
func (t *Tapper) Upload(ctx context.Context, tap config.Tap, skillPath, skillName string, force bool) error
```

`tap.SSHKey` is now available via the `config.Tap` struct — no signature change needed.

**Acceptance Criteria**:

- AC-TAP-1: At the start of `Upload`, after URL validation, when `tap.SSHKey` is non-empty AND `tap.URL` starts with `"git@"`, `EnsureSSHKey` is called with the expanded key path.
- AC-TAP-2: When `tap.SSHKey` is non-empty AND `tap.URL` is HTTPS, `EnsureSSHKey` is NOT called.
- AC-TAP-3: When `tap.SSHKey` is empty, `EnsureSSHKey` is NOT called.
- AC-TAP-4: If `EnsureSSHKey` returns an error, `Upload` returns that error immediately without cloning.
- AC-TAP-5: Path expansion (`~` → home) is applied to `tap.SSHKey` before passing to `EnsureSSHKey`. Use `config.ExpandPath`.
- AC-TAP-6: For git errors (clone/push), `WrapGitError` is checked first. If non-nil, that error is returned. Otherwise the original wrapped error is returned.
- AC-TAP-7: `EnsureSSHKey` is called once only — before the clone. It is not called again before push (the agent session persists).

### 5.2 Test requirements (tap_test.go additions)

**`TestUpload_SSHKey_LoadedBeforeClone`**: SSH URL tap + non-empty SSHKey → mock verifies `ssh-add` is invoked before `git clone`.

**`TestUpload_SSHKey_SkippedForHTTPS`**: HTTPS URL tap + non-empty SSHKey → mock verifies `ssh-add` is NOT invoked.

**`TestUpload_SSHKey_EmptyNoOp`**: SSH URL tap + empty SSHKey → mock verifies `ssh-add` is NOT invoked.

**`TestUpload_SSHKey_ErrorPropagates`**: `ssh-add` fails → `Upload` returns error without calling `git clone`.

**`TestUpload_AuthError_Wrapped`**: mock `git clone` outputs `"Authentication failed"` with HTTPS URL → returned error contains `"token"` or `"credential"`.

All new tests follow existing mock pattern: `oldExec := execCommand; defer func() { execCommand = oldExec }()`.

---

## 6. SKILL.md Documentation Section

### 6.1 New section

**File**: `internal/skillasset/skill/skillsync/SKILL.md`

**Requirement**: Append a new `## Private Repositories` section after the existing `## Common Workflows` section.

**Acceptance Criteria**:

- AC-DOC-1: The section heading is exactly `## Private Repositories`.
- AC-DOC-2: The section contains a YAML config example showing `ssh_key` on both a `bundles` entry and a `taps` entry with `git@` URLs.
- AC-DOC-3: The section shows the manual `ssh-add` command for pre-loading a passphrase-protected key.
- AC-DOC-4: The section mentions that `ssh-add` may prompt for a passphrase on keys that have one, and that pre-loading is required in non-interactive contexts (CI, automated agents).
- AC-DOC-5: The section includes an HTTPS alternative workflow using a personal access token embedded in the URL or a credential helper.
- AC-DOC-6: The section references `~/.ssh/config` as a permanent setup alternative (Host block with `IdentityFile`).
- AC-DOC-7: The section notes that SSH key file permissions must be `600` (`chmod 600 ~/.ssh/id_ed25519_work`).

**Minimum content structure**:

```markdown
## Private Repositories

skillsync can authenticate to private Git repositories using SSH keys or HTTPS tokens.

### SSH key authentication

Add `ssh_key` to a bundle source or tap in your config:

\`\`\`yaml
bundles:
  - name: private-skills
    source:
      type: git
      url: git@github.com:company/skills.git
      branch: main
    ssh_key: ~/.ssh/id_ed25519_work

taps:
  - name: team-tap
    url: git@github.com:company/tap.git
    branch: main
    ssh_key: ~/.ssh/id_ed25519_work
\`\`\`

skillsync runs `ssh-add <key>` automatically before each git operation.

For passphrase-protected keys, pre-load the key once per session:

\`\`\`bash
ssh-add ~/.ssh/id_ed25519_work
\`\`\`

In non-interactive contexts (CI, automated agents), use a deploy key without a passphrase.

Key file permissions must be `600`:

\`\`\`bash
chmod 600 ~/.ssh/id_ed25519_work
\`\`\`

To verify SSH connectivity:

\`\`\`bash
ssh -T git@github.com
\`\`\`

For permanent setup without per-command `ssh_key`, add a `Host` block to `~/.ssh/config`:

\`\`\`
Host github.com
  IdentityFile ~/.ssh/id_ed25519_work
  AddKeysToAgent yes
\`\`\`

### HTTPS token authentication

For HTTPS URLs, use a personal access token (PAT) via a credential helper:

\`\`\`bash
git config --global credential.helper store
\`\`\`

Or embed the token in the URL (not recommended for shared configs):

\`\`\`yaml
bundles:
  - name: private-skills
    source:
      type: git
      url: https://<token>@github.com/company/skills.git
      branch: main
\`\`\`
```

---

## 7. Non-Requirements (Explicit Exclusions)

The following are intentionally out of scope for this change:

- **HTTPS credential storage**: skillsync does not manage git credential helpers. The SKILL.md documents the option but skillsync does not invoke `git config credential.helper`.
- **SSH_AUTH_SOCK detection**: The proposal mentioned detecting `SSH_AUTH_SOCK`. This is deferred — `EnsureSSHKey` will fail with a clear error from `ssh-add` if no agent is running, which is sufficient guidance.
- **`ssh-add -K` (macOS Keychain) flag**: Not added. Keychain behavior varies by macOS version and is not universal across platforms.
- **Key validation**: `EnsureSSHKey` does not validate that the key file exists before running `ssh-add`. `ssh-add` will return a clear error if the path is invalid.
- **Multiple SSH keys per bundle**: `SSHKey` is a single string field. Multiple keys per bundle are not supported. Users can use `~/.ssh/config` for that use case.

---

## 8. Dependency Graph

```
internal/config (Bundle.SSHKey, Tap.SSHKey)
        |
        v
internal/gitauth (EnsureSSHKey, WrapGitError, IsSSHURL)
        |              |
        v              v
internal/sync     internal/tap
        |              |
        v              v
cmd/skillsync/main.go (passes expanded SSHKey via bundle.SSHKey / tap.SSHKey)
```

`gitauth` imports: `context`, `fmt`, `os/exec`, `strings` — no internal imports.

---

## 9. File Summary

| File | Action | Key changes |
|------|--------|-------------|
| `internal/config/config.go` | modify | `SSHKey string` on `Bundle` and `Tap` |
| `internal/config/config_test.go` | modify | `TestBundle_SSHKey_Field`, `TestTap_SSHKey_Field`, `TestBundle_SSHKey_OmitEmpty` |
| `internal/gitauth/gitauth.go` | create | `IsSSHURL`, `EnsureSSHKey`, `WrapGitError`, `execCommand` var |
| `internal/gitauth/gitauth_test.go` | create | Full table-driven test suite for all three functions |
| `internal/sync/sync.go` | modify | `SyncBundle` adds `sshKey string` param; calls `EnsureSSHKey` + `WrapGitError` |
| `internal/sync/sync_test.go` | modify | 5 new test functions for SSH key and auth error behavior |
| `internal/tap/tap.go` | modify | `Upload` reads `tap.SSHKey`; calls `EnsureSSHKey` + `WrapGitError` |
| `internal/tap/tap_test.go` | modify | 5 new test functions for SSH key and auth error behavior |
| `internal/skillasset/skill/skillsync/SKILL.md` | modify | `## Private Repositories` section appended |
| `cmd/skillsync/main.go` | modify | Pass `bundle.SSHKey` to `SyncBundle`; no change needed for `tap.Upload` (struct already passed) |
