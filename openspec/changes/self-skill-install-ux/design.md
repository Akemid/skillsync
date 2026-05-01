# Design: self-skill-install-ux

## Key Decisions

1. **`installSelfSkill` lives in `main.go`, not a new package.** The function touches `skillasset`, `installer`, `tui`, `config`, and `registry` — all of which `main.go` already imports. Extracting it into a new package would either create circular imports or require a thin wrapper that adds no value. Keeping it in `main.go` is consistent with how all other `cmd*` functions work in this coordinator pattern.

2. **`self-skill` subcommand routes BEFORE `reg.Discover()`.** The subcommand only needs config + `skillasset` + `installer`. Placing it alongside `init` (line ~52 in current main.go) means it works even on a fresh install with an empty registry. This avoids the "no skills found" fatal error at line 86.

3. **Idempotency via content comparison, not file existence.** A stale SKILL.md (from a previous version) must be overwritten. Comparing `bytes.Equal(existing, skillasset.Content())` is the cheapest check — no version strings, no hashing. This logic already exists in the post-wizard block (line 163) and is extracted into `installSelfSkill`.

4. **`WizardResult.SelfSkillRequested bool` — signal only, no execution in `tui`.** The `tui` package must NOT import `skillasset` or `installer`. The field is a boolean signal that `main.go` acts on. This preserves the coordinator pattern's no-circular-deps invariant.

5. **`install.sh` uses `|| true` to capture skill install failure.** The `set -e` at the top of install.sh means any uncaught failure exits the script. Since self-skill install is optional, the command is wrapped in `"${INSTALL_DIR}/${BINARY}" self-skill install --yes || true` to prevent a skill-install failure from rolling back a successful binary install.

6. **`--yes` flag parsed locally in `cmdSelfSkillInstall`, not in global flag parsing.** The project uses simple `os.Args` parsing per subcommand (no flag library). `--yes` is only relevant to `self-skill install`, so it's parsed in that function's arg loop — consistent with how `--force`, `--to`, etc. are parsed in other `cmd*` functions.

---

## 1. `installSelfSkill` — shared helper in main.go

```go
// installSelfSkill extracts the embedded self-skill to the registry and
// symlinks it to the given tools at global scope. When interactive is true,
// it prompts for confirmation first. Returns nil if already installed or
// user declines.
func installSelfSkill(cfg *config.Config, tools []config.Tool, interactive bool) error {
    registryPath := config.ExpandPath(cfg.RegistryPath)
    selfSkillDir := filepath.Join(registryPath, skillasset.SkillName)
    selfSkillMD := filepath.Join(selfSkillDir, "SKILL.md")

    // Idempotency: skip if content matches
    if existing, err := os.ReadFile(selfSkillMD); err == nil {
        if bytes.Equal(existing, skillasset.Content()) {
            fmt.Println("skillsync skill already installed")
            return nil
        }
    }

    // Interactive confirmation
    if interactive {
        if !tui.ConfirmSelfSkillInstall() {
            return nil
        }
    }

    // Extract embedded asset
    if err := skillasset.ExtractTo(registryPath); err != nil {
        return fmt.Errorf("extracting self-skill: %w", err)
    }

    // Symlink to all tools at global scope
    selfSkill := registry.Skill{Name: skillasset.SkillName, Path: selfSkillDir}
    results := installer.Install(
        []registry.Skill{selfSkill},
        tools,
        installer.ScopeGlobal,
        "",
    )
    tui.PrintResults(results)

    return nil
}
```

**Where it replaces existing code:** Lines 156-183 of current `main.go` (the post-wizard self-skill block) become a single call: `installSelfSkill(cfg, selectedTools, true)`.

**Caller sites:**
- `cmdSelfSkillInstall` — passes `interactive = !yesFlag`
- Post-wizard block — passes `interactive = true`
- Wizard "self-skill" menu option — passes `interactive = true`

---

## 2. `cmdSelfSkillInstall` — subcommand handler in main.go

```go
func cmdSelfSkillInstall(cfg *config.Config, yesFlag bool) error {
    // Auto-detect installed tools (same as main flow)
    cfg.Tools = tui.DetectInstalledTools(cfg.Tools)

    return installSelfSkill(cfg, cfg.Tools, !yesFlag)
}
```

**Routing in `run()`** — placed BEFORE `reg.Discover()`, alongside `init`:

```go
// Handle self-skill early (only needs config, not registry)
if len(os.Args) > 2 && os.Args[1] == "self-skill" {
    switch os.Args[2] {
    case "install":
        yesFlag := false
        for _, arg := range os.Args[3:] {
            if arg == "--yes" {
                yesFlag = true
            }
        }
        return cmdSelfSkillInstall(cfg, yesFlag)
    default:
        fmt.Fprintf(os.Stderr, "Unknown command: self-skill %s\n\n", os.Args[2])
        printUsage()
        return nil
    }
}
```

**Position in `run()`:** After config load + fallback (line ~74), before tool detection + registry init (line ~77). The function calls `DetectInstalledTools` itself because the main flow hasn't done it yet at this point.

**Config absent handling:** Already covered — lines 64-73 fall back to defaults when config file is missing. The `self-skill` dispatch happens after this block.

---

## 3. `WizardResult.SelfSkillRequested bool`

```go
type WizardResult struct {
    SkillsByBundle       map[string][]string
    SelectedSkills       []string
    SelectedTools        []string
    Scope                installer.Scope
    ProjectDir           string
    SelfSkillRequested   bool  // NEW — true when user picks "Install skillsync skill" from menu
}
```

**Default value:** `false` (Go zero value). All existing paths that construct `WizardResult` are unaffected.

**main.go branching** — after the `RunWizard` call and before the existing tool/skill resolution block:

```go
if result.SelfSkillRequested {
    return installSelfSkill(cfg, cfg.Tools, true)
}
```

This short-circuits: no tool selection, no skill selection, no confirmation — `installSelfSkill` handles its own confirmation prompt.

---

## 4. `askWizardMode()` changes

Add one new option to the existing `huh.NewSelect`:

```go
func askWizardMode() (string, error) {
    var mode string
    err := newForm(
        huh.NewGroup(
            huh.NewSelect[string]().
                Title("What do you want to do?").
                Options(
                    huh.NewOption("Install skills", "install"),
                    huh.NewOption("Install skillsync skill", "self-skill"),  // NEW
                    huh.NewOption("Add remote repository", "add-remote"),
                    huh.NewOption("Share a skill (tap)", "share-skill"),
                    huh.NewOption("Export skill to archive", "export-skill"),
                    huh.NewOption("Import skill from archive", "import-skill"),
                ).
                Value(&mode),
        ),
    ).Run()
    return mode, err
}
```

**Position:** Second option (after "Install skills"), because it's a specialized install action.

**`RunWizard` routing:** Add a case in the existing mode switch:

```go
case "self-skill":
    return &WizardResult{SelfSkillRequested: true}, nil
```

This returns immediately — no scope, bundle, or tool selection needed. `main.go` handles the rest via `installSelfSkill`.

---

## 5. `install.sh` changes

Parse `--with-skill` from args, then conditionally invoke the subcommand after successful binary install.

**Arg parsing** — add after `set -e` (top of script):

```sh
WITH_SKILL=false
for arg in "$@"; do
  case "${arg}" in
    --with-skill) WITH_SKILL=true ;;
  esac
done
```

**Self-skill install** — add at the very end of the script, after the success message:

```sh
# Optional: install the skillsync self-skill
if [ "${WITH_SKILL}" = "true" ]; then
  echo ""
  echo "Installing skillsync skill..."
  "${INSTALL_DIR}/${BINARY}" self-skill install --yes || {
    echo "Warning: self-skill install failed (binary install was successful)" >&2
  }
fi
```

**Why `|| { ... }` instead of `|| true`:** Provides a user-visible warning when the skill install fails, while still preventing `set -e` from killing the script. The binary install is already complete at this point.

**Usage with curl:** `curl -fsSL https://raw.githubusercontent.com/Akemid/skillsync/main/install.sh | sh -s -- --with-skill`

---

## 6. `printUsage()` update

Add the new subcommand to the help text:

```
  skillsync self-skill install  Install the skillsync skill [--yes]
```

Position: after `skillsync init` line, before `skillsync tap add`.

---

## 7. Post-wizard block refactor

Lines 156-183 in current `main.go` are replaced with:

```go
// Self-skill install offer (post-wizard)
if err := installSelfSkill(cfg, selectedTools, true); err != nil {
    fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
}
```

The `installSelfSkill` function handles idempotency internally. If already installed, it prints a message and returns nil. If not installed, it prompts and proceeds (or skips if declined).

---

## Test Strategy

### What gets tested

| Function | Test file | What to verify |
|----------|-----------|----------------|
| `installSelfSkill` | `cmd/skillsync/main_test.go` | Idempotency, extraction, interactive vs non-interactive, error propagation |
| `cmdSelfSkillInstall` | `cmd/skillsync/main_test.go` | `--yes` flag parsing, delegation to `installSelfSkill` |
| `install.sh --with-skill` | manual / CI script test | Flag parsed, binary invoked with `--yes` |

### `installSelfSkill` table-driven tests

Since `installSelfSkill` depends on `tui.ConfirmSelfSkillInstall()` (which reads stdin), tests focus on the **non-interactive path** (`interactive=false`) and verify file-system outcomes:

```go
func TestInstallSelfSkill(t *testing.T) {
    tests := []struct {
        name        string
        setup       func(t *testing.T, registryDir string) // pre-populate files
        interactive bool
        wantSkipped bool   // true if "already installed" path taken
        wantFile    bool   // true if SKILL.md should exist after call
        wantErr     bool
    }{
        {
            name:     "fresh install non-interactive",
            setup:    nil,
            wantFile: true,
        },
        {
            name: "already installed with matching content",
            setup: func(t *testing.T, dir string) {
                // Write current embedded content to simulate prior install
                skillDir := filepath.Join(dir, skillasset.SkillName)
                os.MkdirAll(skillDir, 0755)
                os.WriteFile(filepath.Join(skillDir, "SKILL.md"), skillasset.Content(), 0644)
            },
            wantSkipped: true,
            wantFile:    true,
        },
        {
            name: "stale content gets overwritten",
            setup: func(t *testing.T, dir string) {
                skillDir := filepath.Join(dir, skillasset.SkillName)
                os.MkdirAll(skillDir, 0755)
                os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("old content"), 0644)
            },
            wantFile: true,
        },
    }
    // Each test: t.TempDir() as registry, build config with that path,
    // call installSelfSkill(cfg, tools, false), assert file existence + content.
}
```

### Temp-dir pattern

- `t.TempDir()` for registry path — auto-cleaned by Go test framework.
- Build a `config.Config` with `RegistryPath` set to the temp dir (absolute, no `~`).
- Create fake tool global dirs under a second `t.TempDir()` so `installer.Install` has real targets for symlinks.
- Tools list: one tool with `GlobalPath` pointing to the fake tool dir.

### What is NOT tested (and why)

- `askWizardMode()` — interactive TUI; tested manually. The mode string return value is trivially verifiable.
- `install.sh` — shell script; tested via CI integration test or manual run. Not covered by `go test`.
- `ConfirmSelfSkillInstall()` — already exists, pure UI, no logic to test.
