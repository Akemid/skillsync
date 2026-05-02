# Tasks: non-interactive-install

**Change**: `non-interactive-install`
**Date**: 2026-05-01
**Mode**: Strict TDD — RED before GREEN for every new behavior
**Test runner**: `go test ./...`

---

## Phase 1 — Registry changes

### T01 · RED · Test Bundle field populated in Discover for remote skills
**File**: `internal/registry/registry_test.go`
**Action**: Add `TestDiscover_BundleField` — table-driven, 2 rows:
- `"local"`: `basePath/local-skill/SKILL.md` → `skill.Bundle == ""`
- `"remote"`: `basePath/_remote/acme/remote-skill/SKILL.md` → `skill.Bundle == "acme"`

Use `t.TempDir()`, write minimal SKILL.md frontmatter. Call `reg.Discover()` and find each skill by name.
**Done-when**: compile error or test failure (Bundle field doesn't exist yet).

---

### T02 · GREEN · Add Bundle field to Skill struct and populate in Discover
**File**: `internal/registry/registry.go`
**Action**:
1. Add `Bundle string` to `Skill` struct (after `Files`).
2. Add `bundle string` param to `scanSkillDir`; set `skill.Bundle = bundle`.
3. Local loop → pass `""` to `scanSkillDir`.
4. Remote loop → pass `bundleEntry.Name()` to `scanSkillDir`.
**Done-when**: `go test ./internal/registry/` passes — all tests including T01.

---

### T03 · RED · Test FindByBundleAndName
**File**: `internal/registry/registry_test.go`
**Action**: Add `TestFindByBundleAndName` — table-driven, 4 rows:
- found: `{Name:"go-testing", Bundle:"acme"}` → `(skill, true)`
- wrong bundle: same skill, `"other"` → `(Skill{}, false)`
- local skill: `{Name:"local-s", Bundle:""}`, call with `("", "local-s")` → `(skill, true)`
- not found: empty registry → `(Skill{}, false)`

Construct `Registry` directly, no file I/O.
**Done-when**: compile error (method doesn't exist yet).

---

### T04 · GREEN · Implement FindByBundleAndName
**File**: `internal/registry/registry.go`
**Action**: Add linear scan over `r.Skills` matching both `Bundle` and `Name`.
**Done-when**: `go test ./internal/registry/` passes all tests.

---

### T05 · RED · Test FindByBundle
**File**: `internal/registry/registry_test.go`
**Action**: Add `TestFindByBundle` — table-driven, 3 rows:
- 2 skills in "frontend-cen" + 1 in "backend-cen" → `len==2`
- 2 local skills (Bundle=="") → `len==2`
- no match → `nil`
**Done-when**: compile error (method doesn't exist yet).

---

### T06 · GREEN · Implement FindByBundle
**File**: `internal/registry/registry.go`
**Action**: Add linear scan returning `[]Skill` where `Bundle == bundle`. Return `nil` (not `[]Skill{}`) when none match.
**Done-when**: `go test ./internal/registry/` passes all tests.

---

## Phase 2 — main.go changes

### T07 · RED · Test parseSkillRef
**File**: `cmd/skillsync/main_test.go`
**Action**: Add `TestParseSkillRef` — table-driven, 4 rows:
- `"my-skill"` → `("", "my-skill")`
- `"acme:go-testing"` → `("acme", "go-testing")`
- `"a:b:c"` → `("a", "b:c")`
- `""` → `("", "")`
**Done-when**: compile error (function doesn't exist yet).

---

### T08 · GREEN · Implement parseSkillRef
**File**: `cmd/skillsync/main.go`
**Action**: Add `parseSkillRef(s string) (bundle, name string)` using `strings.SplitN(s, ":", 2)`.
**Done-when**: `go test ./cmd/skillsync/` passes all tests including T07.

---

### T09 · GREEN · Implement syncRemoteBundlesIfNeeded
**File**: `cmd/skillsync/main.go`
**Action**: Add `func syncRemoteBundlesIfNeeded(cfg *config.Config, bundleNames []string) error`:
1. Build `bundleByName` map for bundles where `Source != nil`.
2. `remoteBase = registryAbs/_remote`
3. For each name: skip if local; skip if `_remote/{name}/` exists; else sync via `sync.New` + `SyncBundle` (2min timeout); wrap error as `"auto-sync failed for bundle %q: %w"`.
**Done-when**: `go build ./cmd/skillsync/` succeeds.

---

### T10 · GREEN · Implement cmdInstall
**File**: `cmd/skillsync/main.go`
**Action**: Add `func cmdInstall(cfg *config.Config, reg *registry.Registry, projectDir string) error` with full algorithm:
1. Parse flags (skillFlags, bundleFlag, toolFlags, scopeFlag="global", --yes no-op)
2. Validate no skill+no bundle → usage error
3. Collect bundle names → syncRemoteBundlesIfNeeded
4. reg.Discover(cfg.Bundles...)
5. Validate scope value
6. Resolve tools (--tool filter or DetectInstalledTools)
7. Validate --bundle configured → FindByBundle → deduplicate by Path
8. Resolve each --skill: qualified→FindByBundleAndName, plain→scan (0=error, 1=use, 2+=ambiguous error)
9. installer.Install → tui.PrintResults
**Done-when**: `go build ./cmd/skillsync/` succeeds.

---

### T11 · GREEN · Wire install into run() and update printUsage
**File**: `cmd/skillsync/main.go`
**Action**:
1. Add `case "install": return cmdInstall(cfg, reg, projectDir)` in `run()` switch.
2. Add `skillsync install    Install skills non-interactively` to `printUsage()`.
**Done-when**: `go build ./cmd/skillsync/` succeeds AND `go test ./...` passes green.

---

## Task Summary

| Task | Phase | TDD | File |
|------|-------|-----|------|
| T01 | 1 | RED | `internal/registry/registry_test.go` |
| T02 | 1 | GREEN | `internal/registry/registry.go` |
| T03 | 1 | RED | `internal/registry/registry_test.go` |
| T04 | 1 | GREEN | `internal/registry/registry.go` |
| T05 | 1 | RED | `internal/registry/registry_test.go` |
| T06 | 1 | GREEN | `internal/registry/registry.go` |
| T07 | 2 | RED | `cmd/skillsync/main_test.go` |
| T08 | 2 | GREEN | `cmd/skillsync/main.go` |
| T09 | 2 | GREEN | `cmd/skillsync/main.go` |
| T10 | 2 | GREEN | `cmd/skillsync/main.go` |
| T11 | 2 | GREEN | `cmd/skillsync/main.go` |
