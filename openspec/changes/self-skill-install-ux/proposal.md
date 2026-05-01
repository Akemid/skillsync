# Proposal: self-skill-install-ux

## Intent

The skillsync self-skill (embedded SKILL.md that teaches AI agents how to use the CLI) is currently only offered at the END of the full install wizard, gated behind completing a skill installation. Users who already have skills installed, or who want to install just the self-skill, have no way to do so. This change adds three new entry points — a CLI subcommand, a main menu option, and an install script flag — so the self-skill is accessible from any context without friction.

## Scope IN

- **`skillsync self-skill install [--yes]` subcommand**: New command routed from `main.go`. Without `--yes`: interactive confirmation prompt via `tui.ConfirmSelfSkillInstall()`. With `--yes`: silent install (no TTY required, suitable for CI/scripting). Extracts the embedded SKILL.md to the registry and creates symlinks to all detected tools at global scope.
- **Main wizard menu option**: Add "Install skillsync skill" as a new option in `askWizardMode()` (alongside "Install skills", "Add remote repository", etc.). Selecting it runs the same self-skill install logic (extract + symlink to detected tools) without entering the full install wizard flow.
- **`install.sh --with-skill` flag**: After binary installation completes, if `--with-skill` is passed, the script runs `skillsync self-skill install --yes` automatically. Documents both install variants in output messaging.
- **README update**: Add the `--with-skill` variant alongside the existing one-liner install command.
- **Idempotency**: All paths check if the self-skill is already installed and up-to-date (byte comparison, same as current logic in `main.go`). If current, skip silently (or print "already up to date").
- **Tests**: Unit tests for the new `cmdSelfSkillInstall` logic (extract + idempotency check) using temp directories. Table-driven, stdlib only.

## Scope OUT

- No changes to the `skillasset` package itself — it already provides `ExtractTo()`, `Content()`, and `SkillName`.
- No changes to the `installer` package API.
- No project-scope self-skill install (self-skill is always global — it's a CLI reference, not project-specific).
- No removal/uninstall subcommand for the self-skill (out of scope, can use `skillsync uninstall skillsync --global`).
- No changes to the existing post-wizard self-skill offer (keep it as a fallback for users who go through the full wizard and don't have it yet).

## Approach

### 1. CLI subcommand: `skillsync self-skill install [--yes]`

In `cmd/skillsync/main.go`:

- Add `"self-skill"` case in the subcommand switch (line ~94). It dispatches to a new `cmdSelfSkill()` function that expects `os.Args[2] == "install"`.
- `cmdSelfSkillInstall(cfg *config.Config, yesFlag bool) error`:
  1. Resolve `registryPath` from config.
  2. Check if `registryPath/skillsync/SKILL.md` exists and matches `skillasset.Content()` — if so, print "already up to date" and return nil.
  3. Call `skillasset.ExtractTo(registryPath)`.
  4. If `!yesFlag`, call `tui.ConfirmSelfSkillInstall()` before extracting. If declined, return nil with message.
  5. If `yesFlag`, extract directly (no prompt).
  6. After extraction, create symlinks via `installer.Install()` to all detected tools at `ScopeGlobal`. Print results via `tui.PrintResults()`.
- Add `"self-skill"` to `printUsage()`.
- This command needs config (for registry path + tools) but does NOT need registry discovery, so it can short-circuit before `reg.Discover()`. Move the `self-skill` case handling BEFORE the registry scan block (similar to how `init` is handled early), or load config but skip registry.

### 2. Wizard menu option

In `internal/tui/wizard.go`:

- Add `huh.NewOption("Install skillsync skill", "self-skill")` to `askWizardMode()` options list.
- Add `case "self-skill"` in `RunWizard`'s mode switch. This returns a sentinel or calls back into main. Since `tui` cannot import `skillasset` (would create a dependency — `tui` already imports many internal packages but NOT `skillasset`), the approach is:
  - `RunWizard` returns `nil, nil` for mode `"self-skill"` (same pattern as `"add-remote"`).
  - `main.go` checks `mode` via a new return field or a sentinel. Simplest: add a `Mode string` field to `WizardResult`. When mode is `"self-skill"`, main.go handles the self-skill install logic directly (same as `cmdSelfSkillInstall` but always interactive).
  - Alternative (cleaner): `RunWizard` returns a new `WizardResult` with a `SelfSkillRequested bool` field set to true. Main checks this flag and runs the self-skill install.

**Chosen approach**: Add `SelfSkillRequested bool` to `WizardResult`. When wizard mode is `"self-skill"`, return `&WizardResult{SelfSkillRequested: true}, nil`. In `main.go`, after `RunWizard` returns, check `result.SelfSkillRequested` and call `cmdSelfSkillInstall(cfg, false)`. This keeps `tui` free of `skillasset` imports.

### 3. install.sh `--with-skill` flag

- Parse args at top of script: iterate `$@`, set `WITH_SKILL=false`, flip to `true` on `--with-skill`.
- After the final "installed successfully" message, if `WITH_SKILL=true`:
  ```sh
  echo "Installing skillsync skill..."
  "${INSTALL_DIR}/${BINARY}" self-skill install --yes
  ```
- If `skillsync self-skill install` fails, print a warning but do NOT exit 1 (binary install already succeeded).

### 4. README documentation

- Add a second install variant: `curl -fsSL .../install.sh | sh -s -- --with-skill`
- Brief explanation of what `--with-skill` does.

### 5. Refactoring the existing self-skill logic in main.go

The current self-skill offer block (lines 157-183) duplicates logic that will now live in `cmdSelfSkillInstall`. Refactor: extract the "check + extract + symlink" logic into a shared helper (`installSelfSkill(cfg *config.Config, tools []config.Tool, interactive bool) error`) called from both the post-wizard block and the new subcommand. This eliminates duplication.

## Risks

- **`tui` importing `skillasset`**: Avoided by using the `SelfSkillRequested` flag pattern. The coordinator (`main.go`) handles the actual install. Risk: low.
- **`self-skill` subcommand needs config but not registry**: The current `main.go` flow loads config then immediately scans the registry. The `self-skill` command must short-circuit before registry scan. Risk: low — same pattern as `init` command. Mitigation: place the `self-skill` case right after config load, before `reg.Discover()`.
- **install.sh running binary immediately after install**: The binary might not be in PATH yet (e.g., installed to `~/.local/bin`). Mitigation: use the full `${INSTALL_DIR}/${BINARY}` path, not just `skillsync`.
- **No config file on first run**: `self-skill install --yes` must work even without a config file (fresh install scenario). Mitigation: fall back to defaults (same as `init` does).

## Open Questions

- Should `self-skill install` also support `--project` scope, or is global-only the right default? Current proposal: global-only, since the self-skill is a generic CLI reference. Can be added later if needed.
