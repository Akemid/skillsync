## Verify Report: private-repo-auth (RE-VERIFY)

**Date**: 2026-05-08
**Tests**: 222 passed (was 219), 0 failed
**go vet**: clean

---

## RE-VERIFY Verdict: PASS

All 4 critical items from the previous run are resolved.

---

## Critical Items — Resolved

### CRIT-1: `pullBundle` calls `WrapGitError(url, ...)` — RESOLVED

`pullBundle` signature is now `func (s *Syncer) pullBundle(ctx context.Context, url, targetDir string) error`.
It calls `gitauth.WrapGitError(url, string(output))` before returning the generic error on `git pull` failure.
The `cloneBundle` path also calls `WrapGitError(url, ...)` on clone failure.

### CRIT-2: `tap.Upload` push error calls `WrapGitError` — RESOLVED

Push failure path in `internal/tap/tap.go` now calls `gitauth.WrapGitError(tap.URL, string(out))` before
returning the generic `fmt.Errorf("git push failed: ...")`. Clone path is also wrapped.

### CRIT-3: `EnsureSSHKey` validates file existence — RESOLVED

`EnsureSSHKey` now calls `os.Stat(keyPath)` and returns `"ssh key not found at %s: %w"` before invoking
`ssh-add`. `TestEnsureSSHKey_FileNotFound` exists at `internal/gitauth/gitauth_test.go:219`.

### CRIT-4: SSH key tests in `internal/sync/sync_test.go` — RESOLVED

Both required tests now exist:
- `TestSyncBundle_SSHKey_SkippedForHTTPS` (line 351)
- `TestSyncBundle_SSHKey_EmptyNoOp` (line 393)

Plus the pre-existing `TestSyncBundle_SSHKey_LoadedBeforeClone` (line 280).

---

## Additional Checks — PASS

- `internal/skillasset/skill/skillsync/SKILL.md` has `## Private Repositories` section.
- `cmd/skillsync/main.go` both SyncBundle call sites pass the `sshKey` param:
  - Line 317: `syncer.SyncBundle(ctx, bundle.Name, bundle.Source.URL, bundle.Source.Branch, bundle.SSHKey)`
  - Line 1037: `syncer.SyncBundle(ctx, name, b.Source.URL, b.Source.Branch, b.SSHKey)`

---

## Previously Reported Warnings (carry-over, non-blocking)

- **WARN-1**: `SSHKey` on `Bundle` not `Source` — design/spec discrepancy; implementation follows spec (correct).
- **WARN-2**: `WrapGitError` is case-sensitive — matches spec, design was misleading.
- **WARN-3**: `TestSyncBundle_SSHKey_LoadedBeforeClone` tests failure path, not success path — name is misleading.
- **WARN-4**: `tasks.md` T22 stale — implementation correctly followed spec instead.
- **WARN-5**: `SetExecCommand` exported from gitauth leaks test scaffolding.

---

## AC Coverage Matrix (updated)

| AC | Status | Notes |
|----|--------|-------|
| AC-GA-3 to AC-GA-16 | PASS | All gitauth ACs pass |
| AC-SYNC-1 | PASS | EnsureSSHKey called for SSH URL + non-empty sshKey |
| AC-SYNC-2 | PASS | TestSyncBundle_SSHKey_SkippedForHTTPS now exists |
| AC-SYNC-3 | PASS | TestSyncBundle_SSHKey_EmptyNoOp now exists |
| AC-SYNC-4 | PASS | TestSyncBundle_SSHKey_LoadedBeforeClone |
| AC-SYNC-5 | PASS | config.ExpandPath(sshKey) called before EnsureSSHKey |
| AC-SYNC-6 | PASS | pullBundle calls WrapGitError — FIXED |
| AC-SYNC-7 | PASS | main.go passes bundle.SSHKey at both call sites |
| AC-SYNC-8 | PASS | execLookPath guard runs before EnsureSSHKey |
| AC-TAP-2 | PASS | TestUpload_SSHKey_SkippedForHTTPS |
| AC-TAP-6 | PASS | push error wrapped with WrapGitError — FIXED |
| AC-DOC-1 to AC-DOC-7 | PASS | All doc ACs pass |
