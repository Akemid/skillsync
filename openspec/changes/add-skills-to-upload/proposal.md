# Proposal: Add Skills Upload & Sharing

**Change**: `add-skills-to-upload`
**Status**: proposed
**Date**: 2026-04-28

---

## 1. Intent

skillsync today is a one-way tool: you can **pull** skills from remote repos and install them locally, but there is no way to **push** a skill you created back to a shared repository or hand it to a colleague. This forces users to manually copy directories, write git commands, or zip files — all outside the tool that manages their skills.

This change adds two complementary sharing mechanisms:
- **Git Tap Model** — push a local skill to a git repository you own, so others can `skillsync remote add` + `skillsync sync` to consume it.
- **Local Export/Import** — produce a portable `.tar.gz` archive for air-gapped environments, Slack, email, or any non-git transfer.

Both flows get wizard modes in the TUI, keeping skillsync's interactive-first philosophy.

## 2. Scope

### In Scope
- `skillsync tap add <name> <url>` — register a writable git repo as a tap
- `skillsync tap list` — list registered taps
- `skillsync tap remove <name>` — unregister a tap
- `skillsync upload <skill> --to <tap-name>` — clone tap, copy skill, commit, push
- `skillsync export <skill-name>` — produce `<skill-name>.tar.gz` in cwd (or `--output <path>`)
- `skillsync import <file.tar.gz>` — validate, extract, install to `~/.agents/skills/`
- TUI wizard modes: "Share a skill (tap)", "Export skill", "Import skill"
- Config changes: new `taps` section

### Out of Scope
- Tap authentication management (SSH keys, tokens) — relies on user's existing git auth
- Versioning or changelogs for uploaded skills
- Conflict resolution when tap already contains a skill with the same name (fail-fast, user must resolve)
- Skill signing or integrity verification
- Publishing to a central public registry (agentskills.io)

## 3. Approach

### 3.1 Git Tap Model

A **tap** is a writable git repository the user owns. It mirrors the existing `Source` concept (read-only remote bundles) but adds write capability.

**Tap layout convention:**
```
<tap-repo>/
  skills/
    <skill-name>/
      SKILL.md
      scripts/
      references/
```

**`tap add` flow:**
1. Validate git URL format (reuse `validateGitURL`)
2. Store in config under `taps` section
3. No clone at registration time — deferred to first upload

**`upload` flow:**
1. Resolve skill from local registry by name
2. Resolve tap from config by name
3. Clone tap repo to a temp directory (shallow clone, single branch)
4. Copy skill directory into `skills/<skill-name>/` inside the clone
5. `git add . && git commit -m "Add skill: <skill-name>"` && `git push`
6. Clean up temp directory
7. Print receiver instructions: `skillsync remote add <name> <url> && skillsync sync`

**Why clone-to-temp instead of maintaining a persistent local tap clone?**
- Avoids stale state and merge conflicts
- Each upload is atomic: clone fresh, add, push
- If push fails (e.g., remote changed), user gets a clean error, no corrupt local state

### 3.2 Local Export/Import

**`export` flow:**
1. Resolve skill from local registry
2. Validate SKILL.md exists in skill directory
3. Create `<skill-name>.tar.gz` using `archive/tar` + `compress/gzip` (stdlib only)
4. Archive contains `<skill-name>/` as root directory (preserves structure)
5. Print file path and size

**`import` flow:**
1. Open `.tar.gz` file
2. Extract to temp directory
3. Validate: must contain exactly one top-level directory with a `SKILL.md`
4. Move validated skill directory to `~/.agents/skills/<skill-name>/`
5. If skill already exists, fail with error (no overwrite without `--force`)

### 3.3 TUI Wizard Integration

The wizard's `askWizardMode()` function gains three new options:

```
What do you want to do?
  > Install skills
  > Add remote repository
  > Share a skill (tap)       <-- NEW
  > Export skill               <-- NEW
  > Import skill               <-- NEW
```

**"Share a skill (tap)" wizard:**
1. Select skill from local registry (multi-select? no — one at a time for clarity)
2. Select existing tap OR register a new one inline
3. Confirm upload
4. Execute upload, show result + receiver instructions

**"Export skill" wizard:**
1. Select skill from local registry
2. Input output path (default: `./<skill-name>.tar.gz`)
3. Confirm and export

**"Import skill" wizard:**
1. Input file path (with file browser if feasible, text input otherwise)
2. Show skill name + description extracted from archive
3. Confirm import destination (`~/.agents/skills/<name>/`)
4. Import and show result

## 4. New Packages & Commands

### New Package: `internal/tap`

Responsibility: git-based tap operations (clone, copy skill, commit, push).

```go
type Tap struct {
    Name   string
    URL    string
    Branch string
}

func New(name, url, branch string) *Tap
func (t *Tap) Upload(ctx context.Context, skillPath, skillName string) error
```

Internally uses `os/exec` for git commands, same pattern as `internal/sync`.

### New Package: `internal/archive`

Responsibility: tar.gz creation and extraction with validation.

```go
func Export(skillPath, outputPath string) error
func Import(archivePath, registryPath string) (skillName string, err error)
```

Stdlib only: `archive/tar`, `compress/gzip`, `os`, `path/filepath`.

### New Commands in `main.go`

| Command | Function | Description |
|---------|----------|-------------|
| `tap add <name> <url>` | `cmdTapAdd` | Register a tap |
| `tap list` | `cmdTapList` | List taps |
| `tap remove <name>` | `cmdTapRemove` | Unregister a tap |
| `upload <skill> --to <tap>` | `cmdUpload` | Push skill to tap |
| `export <skill>` | `cmdExport` | Create .tar.gz |
| `import <file>` | `cmdImport` | Install from .tar.gz |

### Changes to Existing Files

| File | Change |
|------|--------|
| `internal/config/config.go` | Add `Tap` struct and `Taps []Tap` to `Config` |
| `internal/tui/wizard.go` | Add 3 new wizard modes + helper functions |
| `cmd/skillsync/main.go` | Add command routing for `tap`, `upload`, `export`, `import` |

## 5. Config Changes

New `taps` section in `skillsync.yaml`:

```yaml
registry_path: ~/.agents/skills
taps:
  - name: my-skills
    url: git@github.com:user/my-agent-skills.git
    branch: main
  - name: team-skills
    url: https://github.com/org/team-skills.git
    branch: main
bundles:
  # ... existing bundles unchanged
tools:
  # ... existing tools unchanged
```

New structs in config:

```go
type Tap struct {
    Name   string `yaml:"name"`
    URL    string `yaml:"url"`
    Branch string `yaml:"branch"`
}
```

`Config` struct gains:
```go
Taps []Tap `yaml:"taps,omitempty"`
```

`upgrade-config` must handle migration: existing configs without `taps` continue to work (omitempty + nil slice = no change).

## 6. Rollback Plan

All changes are additive:
- New packages (`internal/tap`, `internal/archive`) can be deleted without affecting existing code
- New config field (`taps`) uses `omitempty` — existing configs remain valid
- New wizard modes are additional options — removing them restores the original two-option menu
- No changes to existing `sync`, `installer`, or `registry` packages

To rollback: revert the commits. No data migration needed.

## 7. Risks

| Risk | Impact | Mitigation |
|------|--------|------------|
| **Git push failures** (auth, permissions, conflicts) | Upload fails | Clone-to-temp ensures no corrupt local state; clear error messages guide user |
| **Path traversal in tar.gz import** | Security: files written outside registry | Validate all paths are relative and within the expected root during extraction |
| **Large skill directories** | Slow export/upload, large archives | Not mitigated in v1; could add size warnings in future |
| **Tap repo divergence** | Push rejected if tap was updated by others | Fail-fast with clear error; user must resolve manually. Future: `--force` flag |
| **SKILL.md missing in exported archive** | Broken skill imported | Validation step in import rejects archives without SKILL.md |
| **Wizard complexity creep** | 5 modes in one menu may overwhelm | Group related actions (Share submenu?) — evaluate after user feedback |

## 8. Dependency Summary

- No new external dependencies
- `internal/tap` uses `os/exec` (same as `internal/sync`)
- `internal/archive` uses `archive/tar` + `compress/gzip` (stdlib)
- Charm Huh already imported in `internal/tui`
