# Tasks: skillsync-self-skill

## Overview

13 tasks across 5 phases. Strict TDD: every RED task (failing test) precedes its GREEN task (implementation). Dependency order must be respected — do not skip ahead.

---

## Phase 1 — SKILL.md Content

No tests. Pure content authoring.

### T01 — Create embedded SKILL.md

**Action**: Create `internal/skillasset/skill/skillsync/SKILL.md`

**Content requirements**:
- Valid Agent Skills YAML frontmatter: `name: skillsync`, `description: ...`
- Sections: What is skillsync, When to use this skill, Commands Reference (all subcommands), Configuration, Common Workflows
- Commands to document: `skillsync` (wizard), `init`, `list`, `status`, `sync`, `upgrade-config`, `remote add`, `remote list`, `uninstall`, `tap add`, `tap list`, `tap remove`, `upload`, `export`, `import`, `help`

**Done when**: File exists at correct path, starts with `---\n`, frontmatter parses as valid YAML with `name` and `description` fields.

---

## Phase 2 — skillasset Package (TDD)

### T02 (RED) — Write failing tests for skillasset

**Action**: Create `internal/skillasset/skillasset_test.go`

**Tests to include**:

```go
// TestContent_NotEmpty: len(Content()) > 0 AND bytes.HasPrefix(Content(), []byte("---\n"))
// TestExtractTo — table-driven:
//   "happy path creates SKILL.md"       → t.TempDir(), expect file exists, content == Content(), no error
//   "idempotent overwrites existing"    → t.TempDir() with pre-existing SKILL.md ("old"), expect content == Content()
//   "error when destDir absent"         → "/nonexistent/path/"+t.Name(), expect error
```

**Done when**: `go test ./internal/skillasset/...` fails to compile (package doesn't exist yet). The test file itself must be syntactically correct Go.

### T03 (GREEN) — Implement skillasset package

**Action**: Create `internal/skillasset/skillasset.go`

**API**:
```go
package skillasset

//go:embed skill/skillsync/SKILL.md
var skillMD []byte

const SkillName = "skillsync"

func Content() []byte { return skillMD }

func ExtractTo(destDir string) error
// - Returns error if destDir does not exist
// - Creates destDir/skillsync/ subdirectory
// - Writes SKILL.md with O_TRUNC (always overwrite)
// - Wraps errors with fmt.Errorf("context: %w", err)
```

**Imports**: stdlib only (`embed`, `fmt`, `os`, `path/filepath`). No internal package imports.

**Done when**: `go test ./internal/skillasset/...` passes all T02 tests with zero failures.

---

## Phase 3 — tui.ConfirmSelfSkillInstall (TDD)

### T04 (RED) — Write failing smoke test for ConfirmSelfSkillInstall

**Action**: Add to `internal/tui/wizard_test.go`:

```go
func TestConfirmSelfSkillInstall_NoPanic(t *testing.T) {
    // Non-TTY environment: huh.Confirm will fail gracefully
    // Must not panic; must return false (not true) on TUI error
    got := ConfirmSelfSkillInstall()
    if got {
        t.Error("expected false in non-TTY environment")
    }
}
```

**Done when**: `go test ./internal/tui/...` fails with "undefined: ConfirmSelfSkillInstall".

### T05 (GREEN) — Implement ConfirmSelfSkillInstall in wizard.go

**Action**: Add to `internal/tui/wizard.go`:

```go
// ConfirmSelfSkillInstall prompts the user to install the skillsync skill
// into their central registry. Returns true if accepted, false otherwise
// (including TUI failures or non-TTY environments).
// Pure UI — no side effects, no skillasset or installer imports.
func ConfirmSelfSkillInstall() bool {
    var confirm bool
    err := huh.NewForm(
        huh.NewGroup(
            huh.NewConfirm().
                Title("Install the skillsync skill?").
                Description("Adds the skillsync CLI reference to your central registry so AI agents can use it.").
                Value(&confirm),
        ),
    ).Run()
    if err != nil {
        return false
    }
    return confirm
}
```

**Done when**: `go test ./internal/tui/...` passes T04 and all pre-existing tui tests.

---

## Phase 4 — main.go Integration

### T06 — Add new imports to main.go

**Action**: Add to `cmd/skillsync/main.go` import block:
- `"bytes"`
- `"github.com/Akemid/skillsync/internal/skillasset"`

**Done when**: `go build ./cmd/skillsync` succeeds (no unused import errors).

### T07 — Add self-skill post-install block to main.go

**Action**: In `cmd/skillsync/main.go`, after `tui.PrintResults(results)` in the `run()` function, add:

```go
// Self-skill install offer
registryPath := config.ExpandPath(cfg.RegistryPath)
selfSkillDir := filepath.Join(registryPath, skillasset.SkillName)
selfSkillMD := filepath.Join(selfSkillDir, "SKILL.md")

alreadyInstalled := false
if existing, err := os.ReadFile(selfSkillMD); err == nil {
    if bytes.Equal(existing, skillasset.Content()) {
        alreadyInstalled = true
    }
}

if !alreadyInstalled {
    if tui.ConfirmSelfSkillInstall() {
        if err := skillasset.ExtractTo(registryPath); err != nil {
            fmt.Fprintf(os.Stderr, "Warning: could not extract skillsync skill: %v\n", err)
        } else {
            selfSkill := registry.Skill{Name: skillasset.SkillName, Path: selfSkillDir}
            selfResults := installer.Install(
                []registry.Skill{selfSkill},
                selectedTools,
                installer.ScopeGlobal,
                "",
            )
            tui.PrintResults(selfResults)
        }
    }
}
```

**Constraints**:
- Block is non-fatal: errors go to stderr as warnings, never crash
- `selectedTools` is the variable already in scope from the wizard flow
- No new abstractions — inline logic only

**Done when**: `go build ./cmd/skillsync` succeeds with no errors.

---

## Phase 5 — Verify

### T08 — Full test suite green

**Action**: Run `go test ./...`

**Done when**: All packages pass, zero failures, zero skipped tests that shouldn't be skipped.

### T09 — Static analysis clean

**Action**: Run `go vet ./...`

**Done when**: Zero warnings or errors reported.

---

## Dependency Order

```
T01
 └─ T02 (RED) → T03 (GREEN)
                    └─ T04 (RED) → T05 (GREEN)
                                       └─ T06 → T07
                                                   └─ T08 → T09
```

Do NOT start T03 before T02 exists. Do NOT start T07 before T05 compiles.

---

## Files Affected

| File | Action |
|------|--------|
| `internal/skillasset/skill/skillsync/SKILL.md` | CREATE (T01) |
| `internal/skillasset/skillasset_test.go` | CREATE (T02) |
| `internal/skillasset/skillasset.go` | CREATE (T03) |
| `internal/tui/wizard_test.go` | MODIFY — add TestConfirmSelfSkillInstall_NoPanic (T04) |
| `internal/tui/wizard.go` | MODIFY — add ConfirmSelfSkillInstall() (T05) |
| `cmd/skillsync/main.go` | MODIFY — imports + post-install block (T06, T07) |
