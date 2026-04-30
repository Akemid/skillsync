# Spec: wizard-discover-project-skills

## Overview

Extend the "Share a skill (tap)" and "Export skill to archive" wizards to discover and present skills from project-local tool directories (e.g., `.claude/skills/`, `.kiro/skills/`), in addition to skills already visible from the central registry (`~/.agents/skills/`).

---

## Requirements

### REQ-1: Project-local skill discovery (`discoverProjectSkills`)

A new internal helper `discoverProjectSkills(cfg *config.Config, projectDir string) []projectSkill` MUST be implemented in `internal/tui/wizard.go`.

**REQ-1.1** — When `projectDir` is a non-empty string, the helper MUST iterate every entry in `cfg.Tools` and expand `tool.LocalPath` relative to `projectDir` (using `filepath.Join`).

**REQ-1.2** — For each expanded local path that exists on disk, the helper MUST scan immediate subdirectories. A subdirectory is treated as a skill candidate if it contains a `SKILL.md` file at its root.

**REQ-1.3** — The helper MUST deduplicate candidates by resolved absolute path (via `filepath.EvalSymlinks`). Multiple tools that share the same `LocalPath` (e.g., `kiro-ide` and `kiro-cli` both map to `.kiro/skills`) MUST NOT produce duplicate entries.

**REQ-1.4** — The helper MUST skip any candidate whose resolved absolute path has the central registry path (`cfg.RegistryPath`, expanded via `config.ExpandPath`) as a prefix. This prevents skills symlinked from the registry into a local tool dir from appearing twice in the list.

**REQ-1.5** — When `projectDir` is empty (`""`), the helper MUST return an empty slice immediately, without scanning anything.

**REQ-1.6** — When no `cfg.Tools` have a non-empty `LocalPath`, the helper MUST return an empty slice.

**REQ-1.7** — Each returned `projectSkill` MUST carry:
- `Name` — the directory name of the skill (not full path)
- `Path` — the resolved absolute path to the skill directory
- `ToolName` — the name of the first tool whose `LocalPath` contained this skill (used for labeling)

**REQ-1.8** — The helper MUST NOT return an error. Directories that cannot be read or symlinks that cannot be resolved MUST be silently skipped.

---

### REQ-2: Share wizard uses merged skill pool

**REQ-2.1** — `runShareSkillWizard` MUST accept a `projectDir string` parameter (new, appended to existing signature).

**REQ-2.2** — The wizard MUST build its skill option list by merging:
1. Registry skills from `localBundleSkills(reg)` — labeled as `"<name> (registry)"`
2. Project-local skills from `discoverProjectSkills(cfg, projectDir)` — labeled as `"<name> (<relative-tool-dir>)"` where `<relative-tool-dir>` is the `LocalPath` value from the tool (e.g., `.claude/skills`)

**REQ-2.3** — The option value (the string passed to the form `Value`) MUST encode enough information to resolve the absolute skill path after selection. A composite key such as `"registry:<name>"` / `"local:<absolute-path>"` is the expected approach.

**REQ-2.4** — After the user selects a skill, the wizard MUST resolve `skillPath` from:
- The registry (`reg.Skills`) if the selection is a registry skill
- `projectSkill.Path` directly if the selection is a project-local skill

**REQ-2.5** — The existing inline tap-registration flow (when `cfg.Taps` is empty) MUST remain unchanged.

**REQ-2.6** — When the merged skill list is empty (no registry skills AND no project-local skills), the wizard MUST return an error `"no local skills available to share"` — same message as today.

**REQ-2.7** — `RunWizard` MUST pass `projectDir` through to `runShareSkillWizard`.

---

### REQ-3: Export wizard uses merged skill pool

**REQ-3.1** — `runExportWizard` MUST accept `cfg *config.Config` and `projectDir string` parameters (new, added to existing signature that currently only receives `reg`).

**REQ-3.2** — The wizard MUST build its skill option list using the same merge + labeling strategy defined in REQ-2.2 and REQ-2.3.

**REQ-3.3** — After the user selects a skill, `skillPath` MUST be resolved using the same strategy as REQ-2.4.

**REQ-3.4** — `archive.Export(skillPath, outputPath)` MUST receive the resolved absolute path regardless of whether the skill is from the registry or a project-local dir.

**REQ-3.5** — When the merged skill list is empty, the wizard MUST return an error `"no local skills available to export"` — same message as today.

**REQ-3.6** — `RunWizard` MUST pass `cfg` and `projectDir` through to `runExportWizard`.

---

### REQ-4: No changes outside `internal/tui/wizard.go`

**REQ-4.1** — `Registry.Discover()` and the `registry` package MUST NOT be modified.

**REQ-4.2** — `config.Config` schema MUST NOT be modified.

**REQ-4.3** — No other packages (installer, detector, sync) MUST be modified.

---

## BDD Scenarios

### Feature: Project-local skill discovery

---

**Scenario: skill exists only in a project-local tool directory**

```
Given a config with tool "claude" whose LocalPath is ".claude/skills"
And the project directory contains ".claude/skills/my-skill/SKILL.md"
And the central registry does NOT contain "my-skill"
When discoverProjectSkills(cfg, projectDir) is called
Then the result contains one entry with Name="my-skill"
And Path resolves to "<projectDir>/.claude/skills/my-skill"
And ToolName="claude"
```

---

**Scenario: skill exists in both registry and a project-local directory**

```
Given a config with tool "claude" whose LocalPath is ".claude/skills"
And the central registry contains "shared-skill"
And the project directory contains ".claude/skills/shared-skill/SKILL.md"
When the share wizard builds its option list
Then the list contains TWO entries for "shared-skill"
And one is labeled "shared-skill (registry)"
And the other is labeled "shared-skill (.claude/skills)"
And both entries are independently selectable
```

---

**Scenario: project-local skill is a symlink into the registry**

```
Given a config with tool "claude" whose LocalPath is ".claude/skills"
And ".claude/skills/find-skills" is a symlink pointing to "<registryPath>/find-skills"
When discoverProjectSkills(cfg, projectDir) is called
Then the result does NOT contain an entry for "find-skills"
  (because its resolved path is inside the registry path)
```

---

**Scenario: multiple tools share the same local path**

```
Given a config with tools "kiro-ide" and "kiro-cli" both having LocalPath ".kiro/skills"
And the project directory contains ".kiro/skills/my-skill/SKILL.md"
When discoverProjectSkills(cfg, projectDir) is called
Then the result contains exactly ONE entry for "my-skill"
  (resolved absolute path deduplication prevents double-listing)
```

---

**Scenario: no local tools are configured**

```
Given a config whose Tools list is empty (or all tools have empty LocalPath)
When discoverProjectSkills(cfg, projectDir) is called
Then the result is an empty slice
```

---

**Scenario: projectDir is empty**

```
Given any config
When discoverProjectSkills(cfg, "") is called
Then the result is an empty slice immediately (no filesystem access)
```

---

### Feature: Share wizard uses merged skill pool

---

**Scenario: share a project-local skill successfully**

```
Given a registered tap "my-tap"
And a project-local skill "draft-skill" in ".claude/skills/draft-skill/"
And the registry does NOT contain "draft-skill"
When the user opens the share wizard with a valid projectDir
And selects "draft-skill (.claude/skills)" from the skill list
And selects "my-tap" as the destination
And confirms overwrite=false
Then tap.Upload is called with skillPath="<abs-path-to>/.claude/skills/draft-skill"
And the wizard prints a success message
```

---

**Scenario: share wizard with no taps and a project-local skill**

```
Given cfg.Taps is empty
And a project-local skill "draft-skill" exists in ".claude/skills/"
When the user opens the share wizard
Then the wizard prompts to register a tap inline (existing behavior)
After tap registration the skill list includes "draft-skill (.claude/skills)"
And the user can select it and upload successfully
```

---

**Scenario: share wizard — merged list is empty**

```
Given the registry has no local skills (empty or remote-only)
And discoverProjectSkills returns an empty slice
When the share wizard runs
Then the wizard returns error "no local skills available to share"
```

---

### Feature: Export wizard uses merged skill pool

---

**Scenario: export a project-local skill**

```
Given a project-local skill "local-tool" in ".kiro/skills/local-tool/"
And the registry does NOT contain "local-tool"
When the user opens the export wizard with a valid projectDir
And selects "local-tool (.kiro/skills)" from the skill list
And confirms the output path "local-tool.tar.gz"
Then archive.Export is called with skillPath="<abs-path-to>/.kiro/skills/local-tool"
And the .tar.gz file is created at the specified output path
And the wizard prints a success message with the file size
```

---

**Scenario: export wizard — merged list is empty**

```
Given the registry has no local skills
And discoverProjectSkills returns an empty slice
When the export wizard runs
Then the wizard returns error "no local skills available to export"
```

---

## Data Structures

```go
// projectSkill represents a skill found in a project-local tool directory.
type projectSkill struct {
    Name     string // directory name of the skill
    Path     string // resolved absolute path to the skill directory
    ToolName string // name of the tool whose LocalPath contained this skill
}
```

Option value encoding for the selection form (internal detail, not user-visible):

| Source | Option label | Option value |
|--------|-------------|--------------|
| Registry | `"my-skill (registry)"` | `"registry:my-skill"` |
| Project-local | `"my-skill (.claude/skills)"` | `"local:/abs/path/to/.claude/skills/my-skill"` |

---

## Signature Changes

| Function | Before | After |
|----------|--------|-------|
| `runShareSkillWizard` | `(cfg *config.Config, reg *registry.Registry, configPath string)` | `(cfg *config.Config, reg *registry.Registry, configPath string, projectDir string)` |
| `runExportWizard` | `(reg *registry.Registry)` | `(cfg *config.Config, reg *registry.Registry, projectDir string)` |
| `RunWizard` | already has `projectDir` | passes `projectDir` to both; passes `cfg` to `runExportWizard` |

---

## Test Coverage (table-driven, stdlib only)

Tests MUST be added to `internal/tui/wizard_test.go`.

| Test function | Cases |
|---------------|-------|
| `TestDiscoverProjectSkills` | empty projectDir → nil; no tools → nil; skill found → correct Name/Path/ToolName; symlink into registry → skipped; shared local path → deduplicated |
| `TestBuildMergedSkillOptions` | registry-only → all labeled "(registry)"; project-only → all labeled with tool LocalPath; mixed → both groups present |
| `TestResolveSkillPath` | registry key → registry path; local key → project skill path; unknown key → error |

Each test case MUST be a struct row in a `tests := []struct{...}` slice with `t.Run(tc.name, ...)`.
