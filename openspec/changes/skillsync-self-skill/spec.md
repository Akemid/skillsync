# Spec: skillsync-self-skill

**Change**: skillsync-self-skill
**Phase**: spec
**Status**: done
**Created**: 2026-04-30

---

## Overview

Embed a `skillsync` agent skill inside the binary via Go `embed`. After a successful install wizard run, offer the user a single confirm prompt: "Install the skillsync skill? It teaches your AI agent how to use this CLI." If accepted, extract the embedded SKILL.md to `~/.agents/skills/skillsync/` and create symlinks to all configured global tool directories via the existing `installer.Install()`.

---

## Domain 1: `internal/skillasset` package — `ExtractTo`

### Package contract

```go
package skillasset

// ExtractTo writes the embedded SKILL.md to destDir/skillsync/SKILL.md
// and returns a registry.Skill ready to pass to installer.Install().
// destDir must already exist (the registry base path).
func ExtractTo(destDir string) (registry.Skill, error)
```

The embedded file lives at `internal/skillasset/skill/skillsync/SKILL.md` and is accessed via `//go:embed skill/skillsync/SKILL.md` on an `embed.FS` variable.

---

### Scenario 1.1 — Happy path: extract to empty registry dir

```
Given destDir exists and does not contain a "skillsync" subdirectory
When ExtractTo(destDir) is called
Then it creates destDir/skillsync/
And  it writes SKILL.md with the embedded content
And  it returns a registry.Skill{
       Name: "skillsync",
       Path: filepath.Join(destDir, "skillsync"),
     }
And  it returns nil error
```

**Verification**: read destDir/skillsync/SKILL.md; bytes must equal the embedded content.

---

### Scenario 1.2 — Idempotent: extract when skill already exists with identical content

```
Given destDir/skillsync/SKILL.md exists
And   its content is byte-for-byte identical to the embedded SKILL.md
When  ExtractTo(destDir) is called
Then  it returns a valid registry.Skill (same as 1.1)
And   it returns nil error
And   the file on disk is unchanged (mtime may update, content must not)
```

**Design decision**: always write — no byte-comparison short-circuit. The cost is one file write; the benefit is no special-casing. This keeps the function simple and the "idempotent" contract trivially provable.

---

### Scenario 1.3 — Overwrite: extract when skill exists with different content

```
Given destDir/skillsync/SKILL.md exists
And   its content differs from the embedded SKILL.md (simulating an older binary)
When  ExtractTo(destDir) is called
Then  it overwrites destDir/skillsync/SKILL.md with the embedded content
And   it returns a valid registry.Skill
And   it returns nil error
```

**Rationale**: the binary is the authoritative source. No version-checking logic; every `ExtractTo` call installs the current binary's version.

---

### Scenario 1.4 — Error: registry dir parent does not exist

```
Given destDir does not exist on the filesystem
When  ExtractTo(destDir) is called
Then  it returns a non-nil error
And   the error message contains "creating skill dir"
And   no files are created
```

**Constraint**: `ExtractTo` does NOT create `destDir` itself. That is the registry's responsibility. It only creates the `skillsync/` subdirectory inside an existing `destDir`.

---

### Implementation notes

- Use `os.MkdirAll(skillDir, 0755)` for `destDir/skillsync/`.
- Write with `os.WriteFile(destPath, content, 0644)` — `O_TRUNC` semantics cover the overwrite case.
- The returned `registry.Skill` must have `Name: "skillsync"` and `Path` set to the absolute directory path. Description may be left empty (caller does not use it for the self-skill offer).
- Do NOT import other internal packages except `registry` (for the return type). This preserves the no-circular-deps rule.

---

## Domain 2: Wizard — self-skill offer

### Function contract

```go
// AskSelfSkillInstall presents the self-skill offer after a successful install.
// registryPath is the expanded registry base dir (e.g. ~/.agents/skills).
// tools are the full list of configured tools (not just the ones the user picked).
// Returns nil on success or user decline; returns error only on I/O failure.
func AskSelfSkillInstall(cfg *config.Config, registryPath string) error
```

This function lives in `internal/tui/wizard.go`. It may call `skillasset.ExtractTo()` and `installer.Install()` internally.

---

### Scenario 2.1 — Happy path: user accepts after successful install

```
Given the wizard completed and at least one skill was installed (results non-empty)
And   destDir/skillsync/SKILL.md does NOT exist (or exists with different content)
When  AskSelfSkillInstall is called
Then  a huh.Confirm prompt is shown:
        Title: "Install the skillsync skill?"
        Description: "Teaches your AI agent how to use this CLI"
And   user selects Yes
Then  skillasset.ExtractTo(registryPath) is called
And   installer.Install([]registry.Skill{skill}, cfg.Tools, ScopeGlobal, "") is called
And   a success line is printed: "✓ skillsync skill installed"
And   nil is returned
```

---

### Scenario 2.2 — Skip: user declines

```
Given the wizard completed and at least one skill was installed
When  AskSelfSkillInstall is called
And   the huh.Confirm prompt is shown
And   user selects No
Then  skillasset.ExtractTo is NOT called
And   no files are written
And   nil is returned
```

---

### Scenario 2.3 — Already installed with identical content: no prompt shown

```
Given destDir/skillsync/SKILL.md exists
And   its content is byte-for-byte identical to the embedded SKILL.md
When  AskSelfSkillInstall is called
Then  NO prompt is shown to the user
And   nil is returned
```

**Rationale**: avoid prompt fatigue. If the user already has the exact same version of the skill, there is nothing to do. A message may optionally be printed via `dimStyle` ("skillsync skill already up to date") — implementation may omit this; the key constraint is NO confirmation dialog.

**Content comparison**: read `destDir/skillsync/SKILL.md` and compare bytes with the embedded content. If equal → skip. If file is missing or content differs → show prompt.

---

### Scenario 2.4 — Not offered: no skills were installed (wizard cancelled before install)

```
Given tui.RunWizard returned an error (e.g. "installation cancelled") or result is nil
When  the main run() function handles the result
Then  AskSelfSkillInstall is NOT called
```

**Implementation note**: `AskSelfSkillInstall` is only called inside `run()` when `result != nil` and `installer.Install()` was called. The gate lives in `cmd/skillsync/main.go`, not inside `AskSelfSkillInstall` itself. This keeps the wizard function single-purpose.

---

### Integration point in `cmd/skillsync/main.go`

The call is inserted immediately after `tui.PrintResults(results)`:

```go
results := installer.Install(skills, selectedTools, result.Scope, result.ProjectDir)
tui.PrintResults(results)

// Post-install: offer self-skill
if err := tui.AskSelfSkillInstall(cfg, config.ExpandPath(cfg.RegistryPath)); err != nil {
    fmt.Fprintf(os.Stderr, "Warning: self-skill install: %v\n", err)
    // non-fatal — do not return the error
}
```

Self-skill install failure is non-fatal. The user's actual skills are already installed; degrading the session over a bonus offer would be wrong.

---

## Domain 3: SKILL.md content requirements

### File path

`internal/skillasset/skill/skillsync/SKILL.md`

### Required frontmatter

```yaml
---
name: skillsync
description: >-
  Manages Agent Skills across agentic coding tools (Claude Code, Copilot,
  Cursor, Kiro, etc.) by creating symlinks from a central registry
  (~/.agents/skills/) to each tool's skill directory.
---
```

Both `name` and `description` fields are REQUIRED. The description must be parseable by `registry.parseFrontmatter()` (standard YAML, no custom tags).

### Required trigger description

The SKILL.md body must include a section that tells the agent WHEN to activate this skill. Required phrasing (exact wording may vary; intent is mandatory):

> Use this skill when the user asks about installing, managing, syncing, or discovering agent skills with the `skillsync` CLI.

### Required CLI subcommands documentation

The body must document ALL of the following subcommands (derived from `printUsage()` in `cmd/skillsync/main.go`):

| Subcommand | Purpose |
|---|---|
| `skillsync` (no args) | Run interactive install wizard |
| `skillsync list` | List skills in registry |
| `skillsync status` | Show installed skills per tool |
| `skillsync sync` | Fetch/update remote bundles from Git |
| `skillsync upgrade-config` | Migrate existing config safely |
| `skillsync remote list` | List configured remote bundles |
| `skillsync remote add` | Add a remote bundle to config |
| `skillsync uninstall` | Remove a skill symlink |
| `skillsync init` | Generate default config file |
| `skillsync tap add` | Register a writable git repo (tap) |
| `skillsync tap list` | List registered taps |
| `skillsync tap remove` | Remove a registered tap |
| `skillsync upload` | Upload a local skill to a tap |
| `skillsync export` | Export a skill to a .tar.gz archive |
| `skillsync import` | Import a skill from a .tar.gz archive |
| `skillsync help` | Show help |

Each subcommand entry must include at minimum: the command name, its flags (if any), and a one-sentence description of what it does.

### Required configuration section

The body must document:
- Default config path: `~/.config/skillsync/skillsync.yaml`
- `SKILLSYNC_CONFIG` env var override
- `--config <path>` flag

### Required registry section

The body must document:
- Default registry location: `~/.agents/skills/`
- SKILL.md frontmatter format (name + description fields)
- Scope: global vs. project-local installs

### Frontmatter parse compliance

The file MUST pass `registry.parseFrontmatter()` without error. Specifically:
- First line must be `---\n`
- Closing delimiter must be `\n---` (newline before dashes)
- YAML between delimiters must be valid and contain at minimum `name` and `description` keys

---

## Test coverage requirements

### `internal/skillasset/skillasset_test.go`

All tests use `t.TempDir()` for filesystem fixtures. No external test frameworks.

| Test name | Scenario |
|---|---|
| `TestExtractTo_HappyPath` | 1.1 |
| `TestExtractTo_Idempotent` | 1.2 |
| `TestExtractTo_Overwrite` | 1.3 |
| `TestExtractTo_MissingDestDir` | 1.4 |
| `TestExtractTo_SkillMDContentValid` | Verify returned skill has correct Name and the written SKILL.md passes `parseFrontmatter` |

### `internal/tui/wizard_test.go` (or new file)

| Test name | Scenario |
|---|---|
| `TestAskSelfSkillInstall_AlreadyInstalled_SameContent` | 2.3 — no prompt, no writes |

Note: interactive Huh form scenarios (2.1, 2.2) are NOT unit-testable without a terminal. They are covered by manual smoke test only.

---

## Files affected

| File | Action |
|---|---|
| `internal/skillasset/skillasset.go` | CREATE — embed + ExtractTo |
| `internal/skillasset/skillasset_test.go` | CREATE — table-driven tests |
| `internal/skillasset/skill/skillsync/SKILL.md` | CREATE — embedded skill content |
| `internal/tui/wizard.go` | MODIFY — add AskSelfSkillInstall() |
| `cmd/skillsync/main.go` | MODIFY — post-install hook call |

---

## Constraints and invariants

1. `internal/skillasset` must NOT import any other internal package except `registry` (for the return type). This preserves the no-circular-deps rule.
2. `internal/tui/wizard.go` MAY import `skillasset` — this is a new dependency edge but does not create a cycle since `skillasset` does not import `tui`.
3. The embed path `//go:embed skill/skillsync/SKILL.md` MUST reference a path relative to the file declaring it (`internal/skillasset/skillasset.go`). The file MUST be under `internal/skillasset/`.
4. `AskSelfSkillInstall` is global-scope only. It never creates project-local symlinks.
5. Self-skill install failure is non-fatal — logged to stderr as a warning, not returned as an error.
6. No new CLI subcommands are added in this change.
7. No config schema changes.
