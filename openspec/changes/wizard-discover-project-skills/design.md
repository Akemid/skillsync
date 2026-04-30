# Design: wizard-discover-project-skills

## Overview

Technical design for discovering project-local skills in the share and export wizards. All changes are scoped to `internal/tui/wizard.go` and its test file.

---

## Data Structures

### `projectSkill` struct

```go
// projectSkill represents a skill found in a project-local tool directory.
type projectSkill struct {
    Name     string // directory name (e.g., "my-skill")
    Path     string // resolved absolute path to skill dir
    ToolName string // tool whose LocalPath contained this skill (e.g., "claude")
}
```

Lives in `internal/tui/wizard.go` — unexported, wizard-internal only.

### Option key encoding

The huh form `Value` field uses a composite key to distinguish skill sources after selection:

| Source | Key format | Example |
|--------|-----------|---------|
| Registry | `registry:<name>` | `registry:find-skills` |
| Project-local | `local:<absolute-path>` | `local:/Users/me/proj/.claude/skills/my-skill` |

Parsing: `strings.SplitN(key, ":", 2)` — first element is the prefix, second is the identifier.

---

## New Function: `discoverProjectSkills`

### Signature

```go
func discoverProjectSkills(cfg *config.Config, projectDir string) []projectSkill
```

### Algorithm

```
1. if projectDir == "" → return nil
2. registryAbs := filepath.EvalSymlinks(config.ExpandPath(cfg.RegistryPath))
3. seenPaths := map[string]bool{}        // dedup by resolved abs path
4. seenLocalDirs := map[string]bool{}    // dedup by expanded LocalPath (kiro-ide/kiro-cli share .kiro/skills)
5. var result []projectSkill

6. for each tool in cfg.Tools:
     a. if tool.LocalPath == "" → skip
     b. localDir := filepath.Join(projectDir, tool.LocalPath)
     c. absLocalDir, err := filepath.Abs(localDir)
        if err → skip
     d. if seenLocalDirs[absLocalDir] → skip   // prevents scanning same dir twice
        seenLocalDirs[absLocalDir] = true
     e. entries, err := os.ReadDir(absLocalDir)
        if err → skip (dir doesn't exist or unreadable)
     f. for each entry in entries:
          i.   if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") → skip
          ii.  skillDir := filepath.Join(absLocalDir, entry.Name())
          iii. if _, err := os.Stat(filepath.Join(skillDir, "SKILL.md")); err != nil → skip
          iv.  resolved, err := filepath.EvalSymlinks(skillDir)
               if err → skip
          v.   if strings.HasPrefix(resolved, registryAbs) → skip   // symlink into registry
          vi.  if seenPaths[resolved] → skip                        // already found via another tool
               seenPaths[resolved] = true
          vii. result = append(result, projectSkill{
                 Name:     entry.Name(),
                 Path:     resolved,
                 ToolName: tool.Name,
               })

7. return result
```

### Design decisions

| Decision | Rationale |
|----------|-----------|
| No error return | Per REQ-1.8 — unreadable dirs and broken symlinks are silently skipped. The wizard degrades gracefully to showing only registry skills. |
| `seenLocalDirs` dedup before scanning | Short-circuits scanning the same directory twice (kiro-ide/kiro-cli share `.kiro/skills`). More efficient than only deduping results. |
| `strings.HasPrefix(resolved, registryAbs)` for registry check | Simple and correct. `EvalSymlinks` on both sides ensures we compare real paths, not symlink indirection. |
| `SKILL.md` required | Matches `Registry.scanSkillDir` behavior. Directories without `SKILL.md` are not skills. |
| First tool wins `ToolName` | When two tools share the same `LocalPath`, the first tool in `cfg.Tools` order labels the skill. Arbitrary but deterministic. |

---

## New Function: `buildMergedSkillOptions`

### Signature

```go
func buildMergedSkillOptions(
    reg *registry.Registry,
    projectSkills []projectSkill,
) []huh.Option[string]
```

### Algorithm

```
1. var opts []huh.Option[string]

2. // Registry skills first
   for _, name := range localBundleSkills(reg):
     label := name + " (registry)"
     key   := "registry:" + name
     opts = append(opts, huh.NewOption(label, key))

3. // Project-local skills second
   for _, ps := range projectSkills:
     // Find the tool's LocalPath for labeling
     label := ps.Name + " (" + toolLocalPath + ")"
     key   := "local:" + ps.Path
     opts = append(opts, huh.NewOption(label, key))

4. return opts
```

The label uses the tool's `LocalPath` (e.g., `.claude/skills`) rather than `ToolName` to show the user WHERE the skill lives, not WHICH tool found it. This requires passing the tool's `LocalPath` through `projectSkill` or looking it up. To keep `projectSkill` minimal, we add one field:

```go
type projectSkill struct {
    Name      string
    Path      string
    ToolName  string
    LocalPath string // tool's LocalPath value, for display (e.g., ".claude/skills")
}
```

This avoids a cfg lookup in `buildMergedSkillOptions`.

---

## New Function: `resolveSkillPath`

### Signature

```go
func resolveSkillPath(
    key string,
    reg *registry.Registry,
) (string, error)
```

### Algorithm

```
1. parts := strings.SplitN(key, ":", 2)
   if len(parts) != 2 → return "", fmt.Errorf("invalid skill key: %s", key)

2. switch parts[0]:
   case "registry":
     name := parts[1]
     for _, s := range reg.Skills:
       if s.Name == name → return s.Path, nil
     return "", fmt.Errorf("skill %q not found in registry", name)

   case "local":
     path := parts[1]
     // Verify path still exists
     if _, err := os.Stat(path); err != nil:
       return "", fmt.Errorf("skill path %q not accessible: %w", path, err)
     return path, nil

   default:
     return "", fmt.Errorf("unknown skill source %q", parts[0])
```

---

## Modified Functions

### `runShareSkillWizard`

**Signature change:**
```go
// Before
func runShareSkillWizard(cfg *config.Config, reg *registry.Registry, configPath string) error

// After
func runShareSkillWizard(cfg *config.Config, reg *registry.Registry, configPath, projectDir string) error
```

**Body changes:**

Replace the current skill-option block (lines 291-305) with:

```go
// Build merged skill options (registry + project-local)
projectSkills := discoverProjectSkills(cfg, projectDir)
skillOpts := buildMergedSkillOptions(reg, projectSkills)
if len(skillOpts) == 0 {
    return fmt.Errorf("no local skills available to share")
}
```

Replace the current skill-path resolution block (lines 329-339) with:

```go
skillPath, err := resolveSkillPath(selectedSkill, reg)
if err != nil {
    return fmt.Errorf("resolving skill path: %w", err)
}
```

The `selectedSkill` variable now holds a composite key (`registry:name` or `local:/path`), not a bare name.

### `runExportWizard`

**Signature change:**
```go
// Before
func runExportWizard(reg *registry.Registry) error

// After
func runExportWizard(cfg *config.Config, reg *registry.Registry, projectDir string) error
```

**Body changes:**

Replace the current skill-option block (lines 371-379) with the same merged pattern:

```go
projectSkills := discoverProjectSkills(cfg, projectDir)
skillOpts := buildMergedSkillOptions(reg, projectSkills)
if len(skillOpts) == 0 {
    return fmt.Errorf("no local skills available to export")
}
```

Replace the skill-path resolution block (lines 409-419) with:

```go
skillPath, err := resolveSkillPath(selectedSkill, reg)
if err != nil {
    return fmt.Errorf("resolving skill path: %w", err)
}
```

**Note on `selectedSkill` for default output path:** The current code uses `selectedSkill` as the archive filename (`selectedSkill + ".tar.gz"`). With composite keys, we need to extract the display name:

```go
// Extract skill name from composite key for default output path
skillName := selectedSkill
if parts := strings.SplitN(selectedSkill, ":", 2); len(parts) == 2 {
    if parts[0] == "registry" {
        skillName = parts[1]
    } else {
        skillName = filepath.Base(parts[1])
    }
}
outputPath = skillName + ".tar.gz"
```

### `RunWizard` (caller)

Two call sites change:

```go
// Before
case "share-skill":
    return nil, runShareSkillWizard(cfg, reg, configPath)
case "export-skill":
    return nil, runExportWizard(reg)

// After
case "share-skill":
    return nil, runShareSkillWizard(cfg, reg, configPath, projectDir)
case "export-skill":
    return nil, runExportWizard(cfg, reg, projectDir)
```

`projectDir` is already available in `RunWizard`'s signature.

---

## Deduplication Strategy

Two levels of deduplication prevent the same skill from appearing multiple times:

### Level 1: Same local directory scanned twice (tool overlap)

`kiro-ide` and `kiro-cli` both define `LocalPath: ".kiro/skills"`. The `seenLocalDirs` map in `discoverProjectSkills` prevents scanning the same expanded absolute directory twice.

### Level 2: Symlink into registry

A project-local skill at `.claude/skills/find-skills` that is actually a symlink to `~/.agents/skills/find-skills` would appear both as a registry skill and a project-local skill. `filepath.EvalSymlinks` resolves the symlink, and the `strings.HasPrefix(resolved, registryAbs)` check skips it from the project-local list. It still appears once, as a registry skill.

### Non-deduplication: same name, different locations

If `my-skill` exists in both the registry AND as a genuinely different skill in `.claude/skills/my-skill/` (not a symlink), both appear in the list with different labels. This is intentional per the proposal — the user picks the one they want.

---

## Dependency Flow

```
RunWizard(cfg, reg, projectDir, configPath)
  │
  ├─ "share-skill" → runShareSkillWizard(cfg, reg, configPath, projectDir)
  │                     ├─ discoverProjectSkills(cfg, projectDir)
  │                     ├─ buildMergedSkillOptions(reg, projectSkills)
  │                     └─ resolveSkillPath(selectedKey, reg)
  │
  └─ "export-skill" → runExportWizard(cfg, reg, projectDir)
                         ├─ discoverProjectSkills(cfg, projectDir)
                         ├─ buildMergedSkillOptions(reg, projectSkills)
                         └─ resolveSkillPath(selectedKey, reg)
```

No new package imports. No cross-package dependencies added. `config.ExpandPath` is the only external call (already imported).

---

## Test Design

All tests in `internal/tui/wizard_test.go`. Table-driven, stdlib only.

### `TestDiscoverProjectSkills`

Setup: `t.TempDir()` as projectDir, create tool directories and skill subdirs with/without `SKILL.md`.

| Case | Setup | Expected |
|------|-------|----------|
| empty projectDir | `projectDir = ""` | nil |
| no tools configured | `cfg.Tools = nil` | nil |
| skill with SKILL.md | `.claude/skills/my-skill/SKILL.md` exists | 1 result: Name="my-skill", ToolName="claude" |
| dir without SKILL.md | `.claude/skills/not-a-skill/` (no SKILL.md) | 0 results |
| symlink into registry | `.claude/skills/linked` → `registryPath/linked` | 0 results (skipped by registry prefix check) |
| shared local path dedup | kiro-ide + kiro-cli both `.kiro/skills`, one skill | 1 result (not 2) |
| multiple tools, multiple skills | claude has skill-a, kiro has skill-b | 2 results with correct ToolName |

For the symlink test case, create a temp "registry" dir, create a skill in it, then symlink from the project-local tool dir to it. Set `cfg.RegistryPath` to the temp registry dir.

### `TestBuildMergedSkillOptions`

| Case | Input | Expected |
|------|-------|----------|
| registry only | 2 registry skills, 0 project | 2 options, all keys start with "registry:" |
| project only | 0 registry, 2 project | 2 options, all keys start with "local:" |
| mixed | 1 registry + 1 project | 2 options, labels contain "(registry)" and "(.claude/skills)" respectively |
| empty | 0 + 0 | 0 options |

Requires a `*registry.Registry` with populated `Skills` slice. Construct directly — no Discover() needed.

### `TestResolveSkillPath`

| Case | Key | Expected |
|------|-----|----------|
| registry key, skill exists | `"registry:my-skill"` | returns `reg.Skills[i].Path` |
| registry key, skill missing | `"registry:nonexistent"` | error contains "not found" |
| local key, path exists | `"local:/tmp/dir/skill"` | returns `/tmp/dir/skill` |
| local key, path missing | `"local:/nonexistent/path"` | error contains "not accessible" |
| invalid key format | `"garbage"` | error contains "invalid skill key" |
| unknown prefix | `"cloud:my-skill"` | error contains "unknown skill source" |

For local key tests, use `t.TempDir()` to create real paths.

---

## File Impact Summary

| File | Change |
|------|--------|
| `internal/tui/wizard.go` | Add `projectSkill` struct, `discoverProjectSkills`, `buildMergedSkillOptions`, `resolveSkillPath`. Modify `runShareSkillWizard`, `runExportWizard`, `RunWizard` call sites. |
| `internal/tui/wizard_test.go` | Add `TestDiscoverProjectSkills`, `TestBuildMergedSkillOptions`, `TestResolveSkillPath`. |

No other files modified.
