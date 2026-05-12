# Proposal: private-repo-auth

**Status**: complete
**Created**: 2026-05-07

## Problem Statement

skillsync shells out to `git clone`/`git pull`/`git push` in two packages (`sync` and `tap`) but provides zero support for private repository authentication. When git auth fails, users get raw git stderr with no actionable guidance. There is no way to configure an SSH key per-bundle or per-tap, and the embedded SKILL.md has no documentation about private repo setup.

## Proposed Solution

Three coordinated changes, ordered by value:

### Part 1 — SSH Key Auto-Loading (config + sync + tap)

**Config changes** (`internal/config/config.go`):
- Add `SSHKey string` (yaml: `ssh_key,omitempty`) to `Bundle` struct
- Add `SSHKey string` (yaml: `ssh_key,omitempty`) to `Tap` struct
- Field accepts `~`-prefixed paths (e.g., `~/.ssh/id_ed25519_work`)

**Shared helper** — new file `internal/gitauth/gitauth.go`:
- `EnsureSSHAgent(ctx context.Context, keyPath string) error` — runs `ssh-add <expanded_path>` before git operations
- Uses `execCommand` var pattern for testability (same as sync.go and tap.go)
- `IsSSHURL(url string) bool` — returns true for `git@` prefixed URLs
- `ssh-add` is idempotent: already-loaded key returns exit 0

**Integration points**:
- `sync.go:SyncBundle()` — before clone/pull, if bundle has SSHKey and URL is SSH, call `EnsureSSHAgent`
- `tap.go:Upload()` — before clone, if tap has SSHKey and URL is SSH, call `EnsureSSHAgent`

**YAML example**:
```yaml
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
```

### Part 2 — Actionable Auth Error Messages (sync + tap)

**New helper in `internal/gitauth/`**:
- `WrapGitError(err error, stderr string, url string) error` — pattern-matches known auth failures and returns actionable messages
- Patterns detected:
  - `"Permission denied (publickey)"` → suggest `ssh-add <key>` or check SSH config
  - `"Authentication failed"` → suggest credential helper or PAT for HTTPS
  - `"could not read Username"` → suggest using SSH URL or configuring credential helper
  - `"Host key verification failed"` → suggest adding host to known_hosts
- Falls through to original error if no pattern matches

**Integration**: wrap git command errors in `sync.go` and `tap.go` through `WrapGitError`.

### Part 3 — SKILL.md Documentation

Add `## Private Repositories` section to `internal/skillasset/skill/skillsync/SKILL.md`:
- SSH URL format and `ssh_key` config field
- Manual `ssh-add` for passphrase-protected keys
- HTTPS alternative with tokens
- `~/.ssh/config` for permanent setup

## Architecture Decision: New Package

**Decision**: Create `internal/gitauth/` as a new internal package.

**Rationale**:
- Follows project convention: no cross-imports between `sync` and `tap`
- Both packages need identical auth logic — DRY
- Single responsibility: auth concerns isolated from clone/pull/push logic
- `gitauth` imports nothing from other internal packages (receives expanded paths from caller)

**Dependency graph**:
```
config (SSHKey fields)
   ↓
gitauth (EnsureSSHAgent, WrapGitError, IsSSHURL)  ← no internal imports
   ↓           ↓
sync.go      tap.go
```

No circular dependencies. Path expansion happens in the caller before passing to gitauth.

## File Inventory

| File | Action | What Changes |
|------|--------|-------------|
| `internal/config/config.go` | modify | Add `SSHKey` to Bundle and Tap structs |
| `internal/config/config_test.go` | modify | Test SSHKey field parsing |
| `internal/gitauth/gitauth.go` | create | `EnsureSSHAgent`, `WrapGitError`, `IsSSHURL` |
| `internal/gitauth/gitauth_test.go` | create | Tests for all three functions |
| `internal/sync/sync.go` | modify | Call `EnsureSSHAgent` before git ops, use `WrapGitError` |
| `internal/sync/sync_test.go` | modify | Test SSH key loading and error wrapping |
| `internal/tap/tap.go` | modify | Call `EnsureSSHAgent` before git ops, use `WrapGitError` |
| `internal/tap/tap_test.go` | modify | Test SSH key loading and error wrapping |
| `internal/skillasset/skill/skillsync/SKILL.md` | modify | Add Private Repositories section |
| `cmd/skillsync/main.go` | modify | Pass expanded SSHKey paths when calling sync/tap |

## Risks

1. **`ssh-add` behavior varies across OS**: macOS uses Apple's SSH agent with Keychain; Linux may not have an agent running. Mitigation: detect `SSH_AUTH_SOCK` and warn if unset.
2. **Passphrase-protected keys in CI/agent contexts**: `ssh-add` will prompt and hang if stdin is not a terminal. Mitigation: document limitation; suggest pre-loading or deploy keys.
3. **Security**: SSH key paths in YAML are readable — keys themselves are not stored. Document that key permissions must be 600.
4. **Breaking change**: None. `ssh_key` is optional with `omitempty`. Existing configs parse identically.

## Scope Estimate

- ~200-250 lines of new/modified Go code (excluding tests)
- ~150-200 lines of tests
- ~50 lines of SKILL.md documentation
- Estimated: 8-10 task items
