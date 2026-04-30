# Tasks: wizard-discover-project-skills

**Change**: Discover and surface project-local skills in share/export wizards
**Constraint**: All changes in `internal/tui/wizard.go` and `internal/tui/wizard_test.go` only
**Mode**: Strict TDD — RED test first, GREEN implementation second, per task

---

## Phase 1: Data structure + `discoverProjectSkills` helper

### Task 1.1 — RED: `TestDiscoverProjectSkills` — empty projectDir returns empty slice

**File**: `internal/tui/wizard_test.go`
**What to write**: Table case `"empty projectDir"` — call `discoverProjectSkills(cfg, "")` where `cfg` has tools configured. Assert `len(result) == 0`.
**Compile guard**: Add a stub `func discoverProjectSkills(cfg *config.Config, projectDir string) []projectSkill { return nil }` and `type projectSkill struct{}` in wizard.go so the test compiles and FAILS (nil != desired struct fields, or function not found).
**Expected**: Test fails (RED) because struct fields don't exist yet.

---

### Task 1.2 — RED: `TestDiscoverProjectSkills` — SKILL.md present returns skill

**File**: `internal/tui/wizard_test.go`
**What to write**: Case `"skill found"` — create a temp dir, write `<projectDir>/<toolLocalPath>/my-skill/SKILL.md`, pass a `cfg` with one tool whose `LocalPath = "<toolLocalPath>"`. Assert one `projectSkill` returned with `Name="my-skill"`, `ToolName=<tool.Name>`, `Path` set to the absolute path of the skill dir.
**Expected**: Fails RED — stub returns nil/empty.

---

### Task 1.3 — RED: `TestDiscoverProjectSkills` — subdir without SKILL.md is skipped

**File**: `internal/tui/wizard_test.go`
**What to write**: Case `"no SKILL.md"` — create `<projectDir>/<toolLocalPath>/not-a-skill/` (no SKILL.md inside). Assert result is empty.
**Expected**: Fails RED — stub would need real logic.

---

### Task 1.4 — RED: `TestDiscoverProjectSkills` — symlink resolving into registry is skipped

**File**: `internal/tui/wizard_test.go`
**What to write**: Case `"symlink into registry"` — create a real skill dir, create a symlink inside the tool local path pointing to it. Set `cfg.RegistryPath` to the real skill's parent. Assert result is empty (symlink guard fires).
**Expected**: Fails RED.

---

### Task 1.5 — RED: `TestDiscoverProjectSkills` — two tools sharing same LocalPath deduped

**File**: `internal/tui/wizard_test.go`
**What to write**: Case `"shared LocalPath dedup"` — two tools (`kiro-ide`, `kiro-cli`) with same `LocalPath`. One skill dir with SKILL.md. Assert only one `projectSkill` returned.
**Expected**: Fails RED.

---

### Task 1.6 — GREEN: Implement `projectSkill` struct + `discoverProjectSkills`

**File**: `internal/tui/wizard.go`
**What to implement**:
```go
type projectSkill struct {
    Name     string
    Path     string // resolved absolute path
    ToolName string
    LocalPath string // tool's LocalPath value, for display labels
}

func discoverProjectSkills(cfg *config.Config, projectDir string) []projectSkill
```
Implementation rules (from spec REQ-1 + design):
- Return nil if `projectDir == ""` or no tools configured
- Iterate `cfg.Tools`; expand `LocalPath` relative to `projectDir` via `filepath.Join`
- Dedup level 1: `seenLocalDirs` map — skip if same expanded dir already scanned
- `os.ReadDir` the expanded dir; skip errors silently
- For each subdir entry: check `SKILL.md` exists; skip if missing
- Dedup level 2: `filepath.EvalSymlinks` on the subdir; if resolved path has prefix of `config.ExpandPath(cfg.RegistryPath)` → skip
- Dedup level 3: `seenPaths` map on resolved path
- Append `projectSkill{Name, Path: resolved, ToolName, LocalPath: tool.LocalPath}`

**Expected**: All tasks 1.1–1.5 go GREEN.

---

## Phase 2: `buildMergedSkillOptions` + `resolveSkillPath`

### Task 2.1 — RED: `TestBuildMergedSkillOptions` — labels and keys are correct

**File**: `internal/tui/wizard_test.go`
**What to write**: Table-driven test with cases:
- `"registry only"` — one registry skill `"my-skill"`, no project skills → option key `"registry:my-skill"`, label `"my-skill (registry)"`
- `"project only"` — no registry skills, one project skill `{Name:"local-skill", LocalPath:".claude/skills", Path:"/abs/path"}` → key `"local:/abs/path"`, label `"local-skill (.claude/skills)"`
- `"mixed"` — one of each → two options in order registry first, then project
- `"empty"` — both empty → zero options

Stub: add `func buildMergedSkillOptions(reg *registry.Registry, projectSkills []projectSkill) []huh.Option[string] { return nil }`.
**Expected**: Fails RED.

---

### Task 2.2 — RED: `TestResolveSkillPath` — registry key returns registry path

**File**: `internal/tui/wizard_test.go`
**What to write**: Case `"registry found"` — `reg` has skill `{Name:"my-skill", Path:"/abs/reg/my-skill"}`, key `"registry:my-skill"` → returns `"/abs/reg/my-skill", nil`.
Case `"registry missing"` — key `"registry:no-such"` → returns `"", error`.
Stub: `func resolveSkillPath(key string, reg *registry.Registry) (string, error) { return "", nil }`.
**Expected**: Fails RED.

---

### Task 2.3 — RED: `TestResolveSkillPath` — local key returns absolute local path

**File**: `internal/tui/wizard_test.go`
**What to write**: Case `"local found"` — create a temp dir, key `"local:/tmp/that-dir"` (use a real path) → returns that path, nil.
Case `"local missing"` — path does not exist → returns `"", error`.
Case `"invalid key"` — `"badkey"` with no colon → returns `"", error`.
**Expected**: Fails RED.

---

### Task 2.4 — GREEN: Implement `buildMergedSkillOptions` + `resolveSkillPath`

**File**: `internal/tui/wizard.go`
**What to implement**:

```go
func buildMergedSkillOptions(reg *registry.Registry, projectSkills []projectSkill) []huh.Option[string]
```
- Registry skills: filter via `localBundleSkills(reg)`, label `"<name> (registry)"`, key `"registry:<name>"`
- Project skills: label `"<name> (<ps.LocalPath>)"`, key `"local:<ps.Path>"`

```go
func resolveSkillPath(key string, reg *registry.Registry) (string, error)
```
- `strings.SplitN(key, ":", 2)` — if len < 2 → error `"invalid skill key: %q"`
- prefix `"registry"` → find `s.Path` in `reg.Skills`; error if not found
- prefix `"local"` → `os.Stat` the path; error if not exists; return path
- unknown prefix → error

**Expected**: Tasks 2.1–2.3 go GREEN.

---

## Phase 3: Wire into wizards

### Task 3.1 — Update `runShareSkillWizard` signature + body

**File**: `internal/tui/wizard.go`
**What to change**: Add `projectDir string` param.
Replace the manual skill-option block (lines ~291–305) with:
```go
projectSkills := discoverProjectSkills(cfg, projectDir)
skillOpts := buildMergedSkillOptions(reg, projectSkills)
if len(skillOpts) == 0 {
    return fmt.Errorf("no local skills available to share")
}
```
Replace the path-lookup block (~330–339) with `resolveSkillPath(selectedSkill, reg)`.
Update the early-exit error message to match spec: `"no local skills available to share"`.
**Test update**: Update `TestRunShareSkillWizard_EmptyRegistryNoSkills` to pass new `projectDir` arg (empty string `""`).
**Verify**: existing tests still compile and pass.

---

### Task 3.2 — Update `runExportWizard` signature + body

**File**: `internal/tui/wizard.go`
**What to change**: Add `cfg *config.Config, projectDir string` params.
Replace manual skill-option block with `buildMergedSkillOptions`.
Replace path-lookup with `resolveSkillPath`.
For `outputPath` default: extract display name from key — for `"local:/abs/path"` use `filepath.Base(path)` for the default filename; for `"registry:name"` use `name`.
**Test update**: Update `TestRunExportWizard_EmptyRegistry` and `TestRunExportWizard_WithSkills_NoPanic` to pass `cfg, projectDir` args.
**Verify**: existing tests still compile and pass.

---

### Task 3.3 — Update `RunWizard` to pass `projectDir` + `cfg` through

**File**: `internal/tui/wizard.go`
**What to change**: `RunWizard` already receives `cfg` and `projectDir`; update the two call sites:
```go
case "share-skill":
    return nil, runShareSkillWizard(cfg, reg, configPath, projectDir)
case "export-skill":
    return nil, runExportWizard(cfg, reg, projectDir)
```
**Verify**: `go vet ./internal/tui/...` passes.

---

### Task 3.4 — Verify callers in `cmd/skillsync/main.go` need no changes

**File**: `cmd/skillsync/main.go` (read-only check)
**What to verify**: `RunWizard` call site already passes `cfg`, `reg`, `projectDir`, `configPath`. Since `RunWizard`'s own signature does not change, no edit to main.go is needed.
**Action**: `go build ./...` and `go test ./internal/tui/...` — confirm green.

---

## Summary

| # | Type | Function | File |
|---|------|----------|------|
| 1.1–1.5 | RED | `TestDiscoverProjectSkills` | wizard_test.go |
| 1.6 | GREEN | `discoverProjectSkills` + `projectSkill` | wizard.go |
| 2.1 | RED | `TestBuildMergedSkillOptions` | wizard_test.go |
| 2.2–2.3 | RED | `TestResolveSkillPath` | wizard_test.go |
| 2.4 | GREEN | `buildMergedSkillOptions` + `resolveSkillPath` | wizard.go |
| 3.1 | WIRE | `runShareSkillWizard` | wizard.go + wizard_test.go |
| 3.2 | WIRE | `runExportWizard` | wizard.go + wizard_test.go |
| 3.3 | WIRE | `RunWizard` | wizard.go |
| 3.4 | VERIFY | callers in main.go | read-only |

**Total**: 9 tasks (5 RED + 2 GREEN + 2 WIRE/verify)
**Files touched**: `internal/tui/wizard.go`, `internal/tui/wizard_test.go` only
**Dependency order**: Phase 1 → Phase 2 → Phase 3 (linear, no parallelism)
