# Tasks: private-repo-auth

> Strict TDD order: RED (failing test) → GREEN (implementation) → REFACTOR

---

## Phase 1: Config Struct Changes

**Target**: `internal/config/config.go`, `internal/config/config_test.go`

```
T01 RED   internal/config/config_test.go  TestSourceSSHKey — verifies Source struct has SSHKey field; YAML round-trip with ssh_key tag preserves value and zero value round-trips without emitting field (omitempty)
T02 GREEN internal/config/config.go       Source.SSHKey — add `SSHKey string \`yaml:"ssh_key,omitempty"\`` field to Source struct
T03 RED   internal/config/config_test.go  TestTapSSHKey — verifies Tap struct has SSHKey field; same YAML round-trip check as T01
T04 GREEN internal/config/config.go       Tap.SSHKey — add `SSHKey string \`yaml:"ssh_key,omitempty"\`` field to Tap struct
```

---

## Phase 2: gitauth Package

**Target**: `internal/gitauth/gitauth.go`, `internal/gitauth/gitauth_test.go`

### IsSSHURL

```
T05 RED   internal/gitauth/gitauth_test.go  TestIsSSHURL — table-driven: git@ prefix → true; https:// → false; empty string → false; git@github.com:org/repo.git → true; git@gitlab.com:org/repo → true
T06 GREEN internal/gitauth/gitauth.go       IsSSHURL — implement: return strings.HasPrefix(url, "git@")
```

### EnsureSSHKey

```
T07 RED   internal/gitauth/gitauth_test.go  TestEnsureSSHKey_EmptyPath — empty keyPath returns nil without executing any command
T08 GREEN internal/gitauth/gitauth.go       EnsureSSHKey (no-op branch) — return nil when keyPath == ""
T09 RED   internal/gitauth/gitauth_test.go  TestEnsureSSHKey_Success — non-empty keyPath executes ssh-add with expanded path; mock execCommand returns exit 0; no error returned
T10 GREEN internal/gitauth/gitauth.go       EnsureSSHKey (ssh-add branch) — expand path via config.ExpandPath, run execCommand(ctx, "ssh-add", expanded), return error on failure; add execCommand var
T11 RED   internal/gitauth/gitauth_test.go  TestEnsureSSHKey_Failure — mock execCommand returns exit 1 with output; error is returned and contains meaningful message
T12 GREEN internal/gitauth/gitauth.go       EnsureSSHKey error path — wrap ssh-add failure with fmt.Errorf("ssh-add failed: %w", err)
```

### WrapGitError

```
T13 RED   internal/gitauth/gitauth_test.go  TestWrapGitError — table-driven covering all 5 patterns:
           - output contains "Permission denied (publickey)" → message suggests ssh-add
           - output contains "cannot read remote" → message suggests token/credential
           - output contains "could not read Username" → message suggests token/credential
           - output contains "Repository not found" → message suggests access/SSH key
           - output contains "Host key verification failed" → message suggests ssh-keyscan
           - output matches no pattern → wraps raw output with fmt.Errorf("%s", output)
T14 GREEN internal/gitauth/gitauth.go       WrapGitError — implement 5-case string match with actionable messages; fallback to fmt.Errorf("%s", output)
```

---

## Phase 3: sync.go Integration

**Target**: `internal/sync/sync.go`, `internal/sync/sync_test.go`

```
T15 RED   internal/sync/sync_test.go  TestSyncBundle_SignatureAcceptsSSHKey — compile-time check: call SyncBundle with sshKey arg; verifies new signature compiles
T16 GREEN internal/sync/sync.go       SyncBundle signature — add sshKey string param: func (s *Syncer) SyncBundle(ctx context.Context, bundleName, url, branch, sshKey string) error
T17 RED   internal/sync/sync_test.go  TestSyncBundle_SSHKeyCalledWhenSSHURL — mock execCommand; when url is git@ and sshKey non-empty, EnsureSSHKey is invoked (verified via mock call count or side-effect); success path passes through
T18 GREEN internal/sync/sync.go       SyncBundle EnsureSSHKey call — before git ops, if gitauth.IsSSHURL(url) && sshKey != "", call gitauth.EnsureSSHKey(ctx, sshKey); return error on failure
T19 RED   internal/sync/sync_test.go  TestSyncBundle_AuthErrorWrapped — mock execCommand returns git clone failure with "Permission denied (publickey)" in output; returned error contains ssh-add suggestion
T20 GREEN internal/sync/sync.go       SyncBundle WrapGitError — replace raw fmt.Errorf in cloneBundle and pullBundle auth-failure paths with gitauth.WrapGitError(url, string(output))
```

---

## Phase 4: tap.go Integration

**Target**: `internal/tap/tap.go`, `internal/tap/tap_test.go`

```
T21 RED   internal/tap/tap_test.go  TestUpload_SignatureAcceptsSSHKey — compile-time check: call Upload with sshKey arg; verifies new signature compiles
T22 GREEN internal/tap/tap.go       Upload signature — add sshKey string param: func (t *Tapper) Upload(ctx context.Context, tap config.Tap, skillPath, skillName string, force bool, sshKey string) error
T23 RED   internal/tap/tap_test.go  TestUpload_SSHKeyCalledBeforeClone — when tap.URL is git@ and sshKey non-empty, EnsureSSHKey is invoked before clone; mock confirms order
T24 GREEN internal/tap/tap.go       Upload EnsureSSHKey call — before git clone, if gitauth.IsSSHURL(tap.URL) && sshKey != "", call gitauth.EnsureSSHKey(ctx, sshKey); return on error
T25 RED   internal/tap/tap_test.go  TestUpload_AuthErrorWrapped — mock execCommand returns clone failure with "Repository not found" in output; returned error contains access/SSH key suggestion
T26 GREEN internal/tap/tap.go       Upload WrapGitError — replace raw fmt.Errorf on clone failure with gitauth.WrapGitError(tap.URL, string(out))
```

---

## Phase 5: SKILL.md Update

**Target**: `internal/skillasset/skill/skillsync/SKILL.md`

```
T27 WRITE internal/skillasset/skill/skillsync/SKILL.md  PrivateRepositoriesSection — add "## Private Repositories" section covering:
           - ssh_key field usage in bundles (Source) and taps
           - SSH URL format (git@github.com:org/repo.git)
           - ssh-add manual invocation for passphrases
           - HTTPS alternative with credential helper
           - Pointer to ~/.ssh/config for persistent key config
           (No TDD — prose documentation only)
```

---

## Task Summary

| Phase | Tasks | RED | GREEN | WRITE |
|-------|-------|-----|-------|-------|
| 1 — Config structs | 4 | 2 | 2 | — |
| 2 — gitauth package | 10 | 5 | 5 | — |
| 3 — sync.go | 6 | 3 | 3 | — |
| 4 — tap.go | 6 | 3 | 3 | — |
| 5 — SKILL.md | 1 | — | — | 1 |
| **Total** | **27** | **13** | **13** | **1** |

---

## New Files to Create

- `internal/gitauth/gitauth.go` — new package (Phase 2 GREEN tasks)
- `internal/gitauth/gitauth_test.go` — new test file (Phase 2 RED tasks)

## Files Modified

- `internal/config/config.go` — add SSHKey to Source and Tap
- `internal/config/config_test.go` — new test cases for SSHKey fields
- `internal/sync/sync.go` — extended SyncBundle signature, gitauth integration
- `internal/sync/sync_test.go` — new test cases for SSH key and error wrapping
- `internal/tap/tap.go` — extended Upload signature, gitauth integration
- `internal/tap/tap_test.go` — new test cases for SSH key and error wrapping
- `internal/skillasset/skill/skillsync/SKILL.md` — private repo section

## Dependency Order

Phase 1 and Phase 2 are independent — can start in parallel.
Phase 3 and Phase 4 depend on Phase 2 (gitauth package must exist before sync/tap import it).
Phase 5 is independent of all code phases.
