# Archive Report: non-interactive-install

**Status**: ARCHIVED
**Date**: 2026-05-01
**Verdict**: PASS

## Summary

Added non-interactive `skillsync install` subcommand with support for qualified `bundle:skill` syntax, enabling AI agents and CI pipelines to install skills programmatically. Introduced `Bundle` field to `Skill` struct for disambiguation and two new Registry methods (`FindByBundleAndName`, `FindByBundle`) to support bundle-scoped lookups. Auto-syncs remote bundles when needed via a private helper in main.go.

## What Was Built

- **Non-interactive `install` subcommand** with flags `--skill` (repeatable), `--bundle`, `--tool` (repeatable), `--scope`, `--yes`
- **Bundle field on Skill struct** populated during `Discover()` to track source bundle for remote skills
- **FindByBundleAndName(bundle, name)** — resolve qualified `bundle:skill` syntax
- **FindByBundle(bundle)** — retrieve all skills in a given bundle
- **parseSkillRef()** — parse `bundle:skill` and plain `skill` names
- **syncRemoteBundlesIfNeeded()** — auto-sync remote bundles if not yet present, with 2-minute timeout
- **cmdInstall()** — full algorithm: flag parsing, validation (scope, tools, bundles), skill resolution with ambiguity detection, deduplication, and result printing
- **11 strictly-TDD tasks** (RED/GREEN) validated across registry and main subcommand layers
- **Comprehensive test coverage** — 187 tests across 10 packages, all PASS

## Files Changed

| File | Change |
|------|--------|
| `internal/registry/registry.go` | Added `Bundle string` field to Skill; updated Discover() to populate Bundle for remote skills; added FindByBundleAndName() and FindByBundle() methods |
| `internal/registry/registry_test.go` | Added TestDiscover_BundleField, TestFindByBundleAndName, TestFindByBundle, and TestDiscover_MultipleRemoteBundles to verify Bundle population and lookup behavior |
| `cmd/skillsync/main.go` | Added case "install" in run() switch; implemented cmdInstall() (172 lines), parseSkillRef() (7 lines), syncRemoteBundlesIfNeeded() (49 lines); updated printUsage() with install command docs |
| `cmd/skillsync/main_test.go` | Added TestParseSkillRef and TestCmdInstall_ErrorPaths to validate flag parsing and error handling |

## Test Results

- **Total**: 187 tests across 10 packages — all PASS
- **go vet**: clean
- **Strict TDD Mode**: enabled — all 11 tasks followed RED/GREEN discipline
- **Coverage**: Bundle field population, both registry lookup methods, flag parsing, error cases (missing flags, invalid scope, unknown tools, unconfigured bundles)

## Deviations

**None** — specification and design fully implemented. Two suggestions from verify phase (optional):
- S1: Auto-sync error message omits "Run manually" hint (non-blocking; REQ-18 requires prefix only)
- S2: Redundant Discover() call on install path (no correctness impact)

## Implementation Highlights

- **Manual flag parsing** maintains consistency with existing commands; `flag` stdlib not introduced
- **Auto-sync location**: private `syncRemoteBundlesIfNeeded()` in main.go (not exported from tui), preserves no-circular-dependency invariant
- **Deduplication by Path**: seenPaths map prevents duplicate installs when `--bundle` and `--skill` overlap
- **Ambiguity detection**: plain skill names matching 2+ bundles trigger error listing all bundles and instructing `bundle:skill` syntax
- **All detected tools by default**: when `--tool` omitted, all tools detected via `tui.DetectInstalledTools()` are targeted

## Recommendations for Next Session

- Consider merging both SUGGESTION items (auto-sync hint + redundant Discover call) if UX polish desired
- Feature complete; ready for production use
