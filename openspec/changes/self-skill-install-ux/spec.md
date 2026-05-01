# Spec: self-skill-install-ux

## Domain: CLI Subcommand

### Requirement: self-skill subcommand exists and is routed before registry scan
The command `skillsync self-skill install` must be recognised and dispatched in `main.go` after config load but before `reg.Discover()` is called. This keeps the command usable even when the registry is empty.

**Scenarios:**
- **Happy path — interactive**: Given no `--yes` flag, when the user runs `skillsync self-skill install`, then a confirmation prompt is displayed and, upon confirmation, the self-skill is extracted to the registry and symlinked to all detected tools at global scope; exit 0.
- **Happy path — non-interactive**: Given `--yes` is present, when the user runs `skillsync self-skill install --yes`, then no prompt is shown, extraction and symlinking proceed silently; exit 0.
- **Idempotency — already installed**: Given the self-skill SKILL.md already exists in the registry and its content matches the embedded asset, when the user runs the subcommand, then the command prints "skillsync skill already installed" and exits 0 without re-extracting or re-symlinking.
- **Idempotency — content differs**: Given a SKILL.md exists but its content differs from the embedded asset (stale version), when the user runs the subcommand, then extraction proceeds (overwrite), symlinking is run, and the updated version is confirmed.
- **Confirmation declined**: Given no `--yes` flag, when the user is prompted and answers no, then nothing is extracted or symlinked; exit 0.
- **Config absent on fresh install**: Given `~/.config/skillsync/skillsync.yaml` does not exist, when the subcommand is invoked, then the command falls back to default config (same defaults as `init`) and proceeds without error.
- **Extraction failure**: Given `skillasset.ExtractTo` returns an error, when the subcommand runs, then an error message is printed to stderr and the command exits non-zero.
- **Unknown sub-subcommand**: Given the user runs `skillsync self-skill foo`, then an "unknown command" message is printed, usage is shown, and exit 0 (consistent with existing unknown-command behaviour in `run()`).

### Requirement: shared helper deduplicates install logic
A function `installSelfSkill(cfg *config.Config, tools []config.Tool, interactive bool) error` must be the single implementation called from the subcommand, the wizard path, and the existing post-wizard block.

**Scenarios:**
- **Interactive=true**: Given `interactive` is true, when called, then `tui.ConfirmSelfSkillInstall()` is invoked and the install proceeds only on confirmation.
- **Interactive=false**: Given `interactive` is false, when called, then no prompt is shown and the install proceeds unconditionally.
- **Already-installed check**: Given the self-skill is already installed with matching content, when `installSelfSkill` is called, then it returns nil without re-extracting.
- **Partial symlink failure**: Given extraction succeeds but one tool symlink fails, when `installSelfSkill` is called, then the partial results are printed via `tui.PrintResults` and the function returns nil (non-fatal, consistent with `installer.Install` contract).

---

## Domain: Wizard Menu Option

### Requirement: main menu offers "Install skillsync skill" option
`askWizardMode()` in `internal/tui/wizard.go` must include a new option that returns the string `"self-skill"`. `RunWizard` routes this mode by returning a sentinel `WizardResult` that signals the coordinator to call `installSelfSkill`.

**Scenarios:**
- **User selects the option**: Given the wizard launches, when the user selects "Install skillsync skill", then `askWizardMode` returns `"self-skill"` without error.
- **Coordinator receives signal**: Given `RunWizard` returns a `WizardResult` with `SelfSkillRequested: true`, when `main.go` processes the result, then `installSelfSkill(cfg, selectedTools, true)` is called and the wizard exits normally.
- **No circular import introduced**: Given `tui` returns only `SelfSkillRequested: true` on `WizardResult`, when `main.go` handles the flag, then `tui` imports neither `skillasset` nor `installer` for this flow (verified by `go vet ./...` passing).
- **Self-skill already installed — wizard path**: Given the self-skill is already installed, when the user selects this menu option and confirms, then `installSelfSkill` reports "already installed" and the wizard exits cleanly.
- **Other modes unaffected**: Given the user selects any existing mode (install, add-remote, share-skill, export-skill, import-skill), when `RunWizard` returns, then `WizardResult.SelfSkillRequested` is false and existing behaviour is unchanged.

### Requirement: WizardResult carries SelfSkillRequested field
`WizardResult` struct must gain a `SelfSkillRequested bool` field. It must default to `false` for all existing paths.

**Scenarios:**
- **Field absent on normal install flow**: Given the user completes a normal skill install, when `RunWizard` returns, then `result.SelfSkillRequested == false`.
- **Field set on self-skill mode**: Given the user selects "Install skillsync skill", when `RunWizard` returns, then `result.SelfSkillRequested == true` and all other fields are zero-valued.

---

## Domain: install.sh Flag

### Requirement: install.sh accepts --with-skill flag
The script must parse its positional arguments for `--with-skill`. When present, after a successful binary install, it must run `${INSTALL_DIR}/${BINARY} self-skill install --yes`.

**Scenarios:**
- **Flag absent**: Given `--with-skill` is not passed, when the script runs, then only the binary is installed; no `self-skill install` is invoked; behaviour is identical to today.
- **Flag present — skill install succeeds**: Given `--with-skill` is passed and the binary runs successfully, when the script finishes, then it prints a confirmation that the skillsync skill was installed; exit 0.
- **Flag present — skill install fails**: Given `--with-skill` is passed but `self-skill install --yes` exits non-zero (e.g. extraction error), when the script handles this, then it prints a warning to stderr ("Warning: could not install skillsync skill: …") and exits 0 — the binary install is NOT rolled back and the script does NOT exit 1.
- **Binary not in PATH post-install**: Given the binary was installed to `${INSTALL_DIR}/${BINARY}` (full path), when `--with-skill` is active, then the script invokes it via the full path `${INSTALL_DIR}/${BINARY}`, not via `which skillsync`, avoiding PATH-not-updated issues.
- **set -e interaction**: Given the script has `set -e` at the top, when the self-skill install command fails, then the failure is captured explicitly (e.g. via `|| true` or a subshell) so `set -e` does not cause an unintended exit 1.

### Requirement: --with-skill flag is documented in the script's inline usage
The script must print the `--with-skill` flag when displaying help or usage notes, so users piping to sh can discover it.

**Scenarios:**
- **No explicit --help in script today**: This is a SHOULD, not a MUST — a one-line comment at the top of the curl-pipe invocation block is acceptable.

---

## Domain: README Update

### Requirement: README documents the --with-skill one-liner
The project README must include a code block showing the combined install + self-skill one-liner so users discover it at first glance.

**Scenarios:**
- **One-liner variant shown**: Given a user reads the README install section, when they look at the install commands, then they see a variant such as `curl -fsSL … | sh -s -- --with-skill` clearly labelled as "also installs the skillsync skill".
- **Existing one-liner preserved**: Given the current bare install one-liner exists in the README, when the update is applied, then it is kept as the primary example and the `--with-skill` variant is shown as an addendum, not a replacement.
- **Accuracy**: Given the README documents `--with-skill`, when a user copies and runs the command, then it produces the same result as the spec describes (flag is actually implemented in install.sh).

---

## Domain: Tests

### Requirement: installSelfSkill logic is covered by table-driven tests
Tests must live in `cmd/skillsync/` (or a dedicated `internal/selfskill/` package if extracted) and use only stdlib — no external test frameworks.

**Scenarios:**
- **Already installed, content matches**: Given a temp registry dir with a pre-written SKILL.md matching `skillasset.Content()`, when `installSelfSkill` is called with `interactive=false`, then no extraction is performed and the function returns nil.
- **Not installed**: Given a temp registry dir with no self-skill dir, when `installSelfSkill` is called with `interactive=false`, then the SKILL.md is written and `installer.Install` is called; function returns nil.
- **Stale content**: Given a temp registry dir with a SKILL.md whose content differs from the asset, when `installSelfSkill` is called with `interactive=false`, then the file is overwritten with the current asset content.
- **Extraction error propagation**: Given `skillasset.ExtractTo` returns a non-nil error (simulate via a read-only temp dir), when `installSelfSkill` is called, then the error is returned wrapped with context.
- **--yes flag sets interactive=false**: Given `os.Args` contains `self-skill install --yes`, when `run()` dispatches the subcommand, then `installSelfSkill` is called with `interactive=false`.
