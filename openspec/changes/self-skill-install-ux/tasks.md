# Tasks: self-skill-install-ux

## Phase 1 — WizardResult + askWizardMode (tui changes)

### T01 · RED · Add `SelfSkillRequested` field test

**Action**: In `internal/tui/wizard_test.go`, add a test that verifies `WizardResult{}.SelfSkillRequested` is `false` by default (zero value) and that setting it to `true` is observable.

```
func TestWizardResult_SelfSkillRequestedDefault(t *testing.T) {
    r := WizardResult{}
    if r.SelfSkillRequested {
        t.Fatal("SelfSkillRequested should default to false")
    }
}
```

**Done-when**: `go test ./internal/tui/... -run TestWizardResult_SelfSkillRequestedDefault` fails with "SelfSkillRequested field does not exist" / compile error.

---

### T02 · GREEN · Add `SelfSkillRequested bool` to `WizardResult`

**Action**: In `internal/tui/wizard.go`, add `SelfSkillRequested bool` field to `WizardResult` struct (after `ProjectDir`).

**Done-when**: `go test ./internal/tui/... -run TestWizardResult_SelfSkillRequestedDefault` passes.

---

### T03 · RED · Test that `askWizardMode` accepts `"self-skill"` option

**Action**: In `internal/tui/wizard_test.go`, add a table-driven test that exercises `RunWizard` routing: when `WizardResult.SelfSkillRequested == true`, the result must be non-nil and carry `SelfSkillRequested: true`. Since TUI forms can't be driven in tests, test the routing logic indirectly via a new exported helper `WizardResultForMode(mode string) *WizardResult` that `RunWizard` calls internally for the `"self-skill"` branch.

Actually — the minimal testable unit is the signal on the struct. The form itself is not unit-testable (huh requires a TTY). Write a test that instantiates `WizardResult{SelfSkillRequested: true}` and verifies the field reads back correctly. This is the compilation guard.

**Done-when**: Test file compiles and passes — field is reachable. `go test ./internal/tui/... -run TestWizardResult_SelfSkillRequested` passes.

---

### T04 · GREEN · Wire `"self-skill"` case into `RunWizard`

**Action**: In `internal/tui/wizard.go`:
1. Add `huh.NewOption("Install skillsync skill", "self-skill")` as the second option in `askWizardMode()`.
2. Add case `"self-skill"` to the switch in `RunWizard`, returning `&WizardResult{SelfSkillRequested: true}, nil`.
3. Ensure existing modes still return `SelfSkillRequested: false` (zero value — nothing to change, Go default).

**Done-when**: `go test ./internal/tui/... && go vet ./...` pass with no errors. No circular import: `tui` must not import `skillasset` or `installer` for this code path.

---

## Phase 2 — `installSelfSkill` shared helper (main.go — TDD)

### T05 · RED · Write failing tests for `installSelfSkill`

**Action**: In `cmd/skillsync/main_test.go`, add table-driven tests:

```
TestInstallSelfSkill_FreshInstall
TestInstallSelfSkill_AlreadyInstalled_ContentMatches
TestInstallSelfSkill_StaleContent_Overwrites
TestInstallSelfSkill_ExtractionError
```

Setup pattern for each: `t.TempDir()` for registry path; fake tool dirs for symlink targets (create the dir so installer can create symlinks). Pass `interactive: false` throughout (avoids stdin dependency).

- `FreshInstall`: registry dir exists, no SKILL.md yet → expect nil error, SKILL.md written, symlink present.
- `AlreadyInstalled`: SKILL.md already exists with content `== skillasset.Content()` → expect nil error, no re-extraction (check file mtime unchanged or just that no error).
- `StaleContent`: SKILL.md exists but with different content → expect nil error, SKILL.md now matches `skillasset.Content()`.
- `ExtractionError`: registry path is a file (not a dir) so `skillasset.ExtractTo` fails → expect non-nil error containing "extracting self-skill".

**Done-when**: `go test ./cmd/skillsync/... -run TestInstallSelfSkill` fails to compile (function does not exist yet).

---

### T06 · GREEN · Implement `installSelfSkill` in `main.go`

**Action**: In `cmd/skillsync/main.go`, add:

```go
func installSelfSkill(cfg *config.Config, tools []config.Tool, interactive bool) error {
    registryPath := config.ExpandPath(cfg.RegistryPath)
    selfSkillDir := filepath.Join(registryPath, skillasset.SkillName)
    selfSkillMD  := filepath.Join(selfSkillDir, "SKILL.md")

    // Idempotency check
    if existing, err := os.ReadFile(selfSkillMD); err == nil {
        if bytes.Equal(existing, skillasset.Content()) {
            fmt.Println("skillsync skill already installed")
            return nil
        }
    }

    // Optional confirmation prompt
    if interactive {
        if !tui.ConfirmSelfSkillInstall() {
            return nil
        }
    }

    // Extract
    if err := skillasset.ExtractTo(registryPath); err != nil {
        return fmt.Errorf("extracting self-skill: %w", err)
    }

    // Symlink
    selfSkill := registry.Skill{Name: skillasset.SkillName, Path: selfSkillDir}
    results := installer.Install([]registry.Skill{selfSkill}, tools, installer.ScopeGlobal, "")
    tui.PrintResults(results)
    return nil
}
```

**Done-when**: `go test ./cmd/skillsync/... -run TestInstallSelfSkill` — all four cases pass. `go vet ./...` clean.

---

### T07 · REFACTOR · Replace post-wizard inline block with `installSelfSkill`

**Action**: In `cmd/skillsync/main.go`, replace lines 156–183 (the existing self-skill install offer after the wizard) with:

```go
return installSelfSkill(cfg, selectedTools, true)
```

This is a pure structural refactor — no behavioural change. The `run()` function still calls it with `interactive: true` after the main install.

**Done-when**: `go test ./... && go vet ./...` pass. Diff shows lines 156–183 replaced by the one-liner.

---

## Phase 3 — `cmdSelfSkillInstall` subcommand (main.go — TDD)

### T08 · RED · Write failing tests for `cmdSelfSkillInstall`

**Action**: In `cmd/skillsync/main_test.go`, add:

```
TestCmdSelfSkillInstall_UnknownSubSubcommand
TestCmdSelfSkillInstall_YesFlagParsed
```

- `UnknownSubSubcommand`: set `os.Args = []string{"skillsync", "self-skill", "foo"}` → capture stdout, expect output contains "unknown command" or usage text, error nil.
- `YesFlagParsed`: `os.Args = []string{"skillsync", "self-skill", "install", "--yes"}` with a temp registry dir → `cmdSelfSkillInstall` receives `yesFlag: true` (verify indirectly: function returns nil without panicking, non-interactive path runs).

**Done-when**: `go test ./cmd/skillsync/... -run TestCmdSelfSkillInstall` fails to compile.

---

### T09 · GREEN · Implement `cmdSelfSkillInstall`

**Action**: In `cmd/skillsync/main.go`, add:

```go
func cmdSelfSkillInstall(cfg *config.Config, yesFlag bool) error {
    return installSelfSkill(cfg, tui.DetectInstalledTools(cfg.Tools), !yesFlag)
}
```

And in `run()`, BEFORE `reg.Discover()` (alongside the `init` early-exit block), add routing:

```go
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

Also handle the fallback config (config absent on fresh install): the routing block sits after config load (lines 62–74), which already falls back to defaults if `os.ErrNotExist` — no extra work needed.

Finally, add `"self-skill"` to the existing `switch os.Args[1]` default block so it doesn't hit "Unknown command" after `reg.Discover()` — actually, because routing occurs before `reg.Discover()`, the switch is never reached for `self-skill`. No change needed in the lower switch.

Update `printUsage()` to include:
```
  skillsync self-skill install    Install the skillsync skill into the registry
  skillsync self-skill install --yes  Non-interactive self-skill install
```

**Done-when**: `go test ./cmd/skillsync/... -run TestCmdSelfSkillInstall` passes. `go test ./... && go vet ./...` clean.

---

## Phase 4 — Wire wizard self-skill path in main.go

### T10 · (no RED — integration wiring) · Handle `SelfSkillRequested` in `run()`

**Action**: In `cmd/skillsync/main.go`, after `result, err := tui.RunWizard(...)` and the nil-check, add:

```go
if result.SelfSkillRequested {
    return installSelfSkill(cfg, cfg.Tools, true)
}
```

Place this BEFORE the "Resolve selected tools" block (line ~139). The existing post-wizard `installSelfSkill` call (from T07) remains — it handles the offer after a normal install. The new block handles the wizard-menu shortcut path that skips straight to self-skill.

**Done-when**: `go build ./cmd/skillsync` succeeds. `go test ./... && go vet ./...` clean. Manual smoke: run wizard, select "Install skillsync skill" → install proceeds without entering the normal install flow.

Note: No dedicated unit test is added here — the wiring is exercised by the existing `TestWizardResult_SelfSkillRequested` compilation test (T03) and the `TestInstallSelfSkill_*` tests cover the called function. An end-to-end test would require TTY stdin mocking.

---

## Phase 5 — install.sh + README (non-Go)

### T11 · (non-testable) · Add `--with-skill` flag to `install.sh`

**Action**: In `install.sh` at the project root:
1. At the top of the script (after `set -e`), parse `--with-skill` from `$@`:
   ```sh
   WITH_SKILL=0
   for arg in "$@"; do
     case "$arg" in
       --with-skill) WITH_SKILL=1 ;;
     esac
   done
   ```
2. After the "Installation complete" success message, add:
   ```sh
   if [ "$WITH_SKILL" -eq 1 ]; then
     "${INSTALL_DIR}/${BINARY}" self-skill install --yes || {
       echo "Warning: skillsync skill install failed (binary was installed successfully)" >&2
     }
   fi
   ```

**Done-when**: `bash -n install.sh` passes (syntax check). Manual test: `./install.sh --with-skill` installs binary and then runs `skillsync self-skill install --yes`.

---

### T12 · (non-testable) · Update README install section

**Action**: In `README.md`, add a second one-liner variant below the existing install command:

```markdown
# Also installs the skillsync skill into your registry:
curl -fsSL https://raw.githubusercontent.com/Akemid/skillsync/main/install.sh | sh -s -- --with-skill
```

Label clearly. Keep the existing bare one-liner as the primary example.

**Done-when**: File contains both variants. The `--with-skill` flag documented matches the implementation in `install.sh`.

---

## Summary

| ID  | Phase | TDD  | File(s) touched |
|-----|-------|------|-----------------|
| T01 | 1     | RED  | `internal/tui/wizard_test.go` |
| T02 | 1     | GREEN| `internal/tui/wizard.go` |
| T03 | 1     | RED  | `internal/tui/wizard_test.go` |
| T04 | 1     | GREEN| `internal/tui/wizard.go` |
| T05 | 2     | RED  | `cmd/skillsync/main_test.go` |
| T06 | 2     | GREEN| `cmd/skillsync/main.go` |
| T07 | 2     | REFACTOR | `cmd/skillsync/main.go` |
| T08 | 3     | RED  | `cmd/skillsync/main_test.go` |
| T09 | 3     | GREEN| `cmd/skillsync/main.go` |
| T10 | 4     | wire | `cmd/skillsync/main.go` |
| T11 | 5     | n/a  | `install.sh` |
| T12 | 5     | n/a  | `README.md` |

**Test runner**: `go test ./...`
**Strict TDD**: every Go unit follows RED → GREEN → REFACTOR order.
