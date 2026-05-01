# Archive: self-skill-install-ux

## Status: ARCHIVED
**Date**: 2026-04-30
**Verdict**: PASS
**Tests**: 155 → 164 (+9)

## What Was Built
Added three new entry points for installing the skillsync self-skill (embedded SKILL.md): a dedicated `skillsync self-skill install [--yes]` subcommand, a wizard menu option, and an `install.sh --with-skill` flag. The subcommand works before registry scan (fresh install scenario), supports both interactive and non-interactive modes, and idempotency via content comparison. The wizard menu option returns a `SelfSkillRequested` boolean signal, keeping `tui` free of `skillasset`/`installer` imports. The install script variant auto-runs the self-skill install after binary placement. All logic consolidated in a single `installSelfSkill()` helper to eliminate duplication.

## Files Changed
- `cmd/skillsync/main.go` — added `installSelfSkill()` and `cmdSelfSkillInstall()` helpers, added `self-skill` subcommand routing before `reg.Discover()`, wired `WizardResult.SelfSkillRequested` handler, replaced inline post-wizard block with helper call, updated `printUsage()`
- `cmd/skillsync/main_test.go` — added 6 new tests (4 for `installSelfSkill`, 2 for `cmdSelfSkillInstall`)
- `internal/tui/wizard.go` — added `SelfSkillRequested bool` field to `WizardResult`, added `"self-skill"` case to `RunWizard` switch, added menu option to `askWizardMode()`
- `internal/tui/wizard_test.go` — added 2 new tests for `SelfSkillRequested` field
- `install.sh` — added `--with-skill` flag parsing and non-fatal post-install call
- `README.md` — added `--with-skill` one-liner variant

## Deviations
None. All 12 tasks (T01–T12) completed as specified. T08 and T05 RED phases were combined (single file compilation error); T09 implementation paired with T06 to unblock build. Both deviations were implementation optimizations, not spec deviations.

## Test Summary
**Total**: 164 passed, 0 failed
**New tests**: 9 (T05, T08 groups)
**Verification**: go vet clean, all spec requirements met

## Artifact References
- Proposal: `openspec/changes/self-skill-install-ux/proposal.md`
- Spec: `openspec/changes/self-skill-install-ux/spec.md`
- Design: `openspec/changes/self-skill-install-ux/design.md`
- Tasks: `openspec/changes/self-skill-install-ux/tasks.md`
- Verify Report: `openspec/changes/self-skill-install-ux/verify-report.md`
