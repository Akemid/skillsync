# Proposal: wizard-discover-project-skills

## Intent

The "Share a skill (tap)" and "Export skill to archive" wizards only show skills from the central registry (`~/.agents/skills/`). Skills created directly in project-local tool directories (e.g., `.kiro/skills/my-skill/`, `.claude/skills/my-skill/`) are invisible and cannot be shared or exported. This blocks a common workflow where users author skills locally before publishing them.

## Scope

### IN

- Discover skills from project-local tool directories (e.g., `.claude/skills/`, `.kiro/skills/`) and include them in the wizard's skill list for share (tap) and export flows.
- Clearly label local-project skills vs central-registry skills in the selection UI so the user knows where each skill lives.
- Resolve the correct absolute path for local-project skills so `tap.Upload` and `archive.Export` receive a valid `skillPath`.

### OUT

- No changes to the install flow (it already works with bundles + registry).
- No changes to `Registry.Discover()` -- project-local skills are NOT added to the central registry struct. They are discovered ad-hoc in the wizard only.
- No changes to `config.Config` schema.
- No global-path scanning (only project-local paths, i.e., `LocalPath` from `config.Tool`).
- No deduplication logic between registry and project-local (same-name skill in both locations shows twice, labeled by source).

## Approach

### 1. New helper: `discoverProjectSkills` in `internal/tui/wizard.go`

```go
type projectSkill struct {
    Name     string
    Path     string // absolute path to skill directory
    ToolName string // which tool directory it was found in (e.g., "claude", "kiro-ide")
}

func discoverProjectSkills(cfg *config.Config, projectDir string) []projectSkill
```

- Iterates `cfg.Tools`, expands each `tool.LocalPath` relative to `projectDir`.
- For each tool directory that exists, scans for subdirectories containing `SKILL.md`.
- Deduplicates by absolute path (multiple tools may share the same local path, e.g., `kiro-ide` and `kiro-cli` both use `.kiro/skills`).
- Returns the merged list with tool attribution.

### 2. Modify `runShareSkillWizard`

Current flow:
```
localSkills := localBundleSkills(reg)  // only central registry
```

New flow:
```
registrySkills := localBundleSkills(reg)         // central registry skills
projectSkills := discoverProjectSkills(cfg, cwd) // project-local skills
// merge into unified option list with source labels
```

- Skill options show source: `"my-skill (registry)"` vs `"my-skill (.claude/skills)"`.
- After selection, resolve path from the correct source (registry `reg.Skills` or `projectSkill.Path`).
- Requires passing `projectDir` into `runShareSkillWizard` (currently not passed -- signature change needed).

### 3. Modify `runExportWizard`

Same pattern: merge registry + project-local skills, label them, resolve path from correct source. Requires passing `cfg` and `projectDir` (currently only receives `reg`).

### 4. Signature changes

| Function | Current | New |
|---|---|---|
| `runShareSkillWizard` | `(cfg, reg, configPath)` | `(cfg, reg, configPath, projectDir)` |
| `runExportWizard` | `(reg)` | `(cfg, reg, projectDir)` |
| `RunWizard` (caller) | already has `projectDir` | passes it through to both |

### 5. No changes to `registry` package

Project-local skills are a wizard concern, not a registry concern. The registry represents the central skill store. Mixing project-local paths into `Registry.Skills` would violate its single responsibility (central registry scanning).

## Key Decisions

| Decision | Rationale |
|---|---|
| Scan in wizard, not in registry | Registry = central store. Project-local discovery is a UI/workflow concern. Keeps `registry` package focused. |
| Label skills by source | User must know WHERE a skill lives -- sharing a project-local skill vs a registry skill has different implications. |
| Show duplicates, don't merge | If `my-skill` exists in both registry and `.claude/skills/`, show both. User picks the one they want. Avoids silent precedence bugs. |
| Use `SKILL.md` presence as skill indicator | Consistent with how `Registry.scanSkillDir` works. A directory without `SKILL.md` is still listed (name-only), matching existing behavior. |
| Deduplicate by absolute path | `kiro-ide` and `kiro-cli` share `.kiro/skills` -- scanning it twice would produce duplicates. Deduplicate before presenting options. |

## Risks

| Risk | Mitigation |
|---|---|
| Symlinked project skills pointing back to registry | `discoverProjectSkills` should resolve symlinks and skip any path that resolves inside `cfg.RegistryPath`. Prevents showing the same skill twice through different routes. |
| `projectDir` is empty (no project context) | Guard: if `projectDir == ""`, skip project-local discovery. Only registry skills shown (current behavior). |
| Tool local paths overlap (e.g., kiro-ide/kiro-cli) | Deduplicate by resolved absolute path before building options. |
| Large number of skills clutters the UI | Unlikely in practice. If needed later, add filtering/search. Not in scope now. |
