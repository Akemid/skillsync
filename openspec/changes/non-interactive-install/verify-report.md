# Verify Report: non-interactive-install

**Change**: `non-interactive-install`
**Date**: 2026-05-01
**Verifier**: sdd-verify sub-agent (re-run)

---

## Verdict

PASS

---

## Test Results

- Total: 187 tests across 10 packages — all PASS
- go vet: clean

---

## Prior Findings Resolution

- **C1**: RESOLVED — `run()` now sets `isInstall` flag and skips the empty-registry guard when subcommand is `install` (`main.go:104–110`). cmdInstall is always reached regardless of initial registry state.
- **W1**: RESOLVED — Validation order in `cmdInstall` now matches spec exactly: missing flags → invalid scope → invalid tool → [sync+discover] → unknown bundle → skill resolution.
- **W2**: RESOLVED — `TestDiscover_MultipleRemoteBundles` now asserts `Bundle` values per skill (lines 334–338 in `registry_test.go`), verifying `skill-1.Bundle == "bundle-1"` and `skill-2.Bundle == "bundle-2"`.
- **W3**: RESOLVED — `TestCmdInstall_ErrorPaths` added in `main_test.go` (lines 876–938) with 5 table-driven cases: no flags, only --yes, invalid scope, unknown tool, bundle not configured.

---

## Findings

### CRITICAL (blocks merge)

(none)

---

### WARNING (should fix)

(none)

---

### SUGGESTION (optional)

**S1 — auto-sync error omits "Run manually" hint**

The design (`design.md`) specifies the error format as:
```
"auto-sync failed for bundle %q: %w\nRun manually: skillsync sync"
```
The implementation (`main.go:1041`) uses only:
```
"auto-sync failed for bundle %q: %w"
```
REQ-18 only requires the prefix match, so this is not a spec violation. The design's UX recovery hint is absent but non-blocking.

**S2 — Redundant Discover() call on install path**

`run()` calls `reg.Discover()` at line 100 for all subcommands, then `cmdInstall` calls it again at line 906 after any auto-sync. The first call is wasted on the install path. No correctness impact.

---

## Task Completion Matrix

| Task | Status | Evidence |
|------|--------|---------|
| T01 RED — TestDiscover_BundleField | DONE | Test exists and passes (`registry_test.go:457`) |
| T02 GREEN — Bundle field + Discover | DONE | `registry.go:25`, `scanSkillDir:107` |
| T03 RED — TestFindByBundleAndName | DONE | 4-row table test passes (`registry_test.go:343`) |
| T04 GREEN — FindByBundleAndName | DONE | `registry.go:129–136` |
| T05 RED — TestFindByBundle | DONE | 3-row table test, nil-check passes (`registry_test.go:403`) |
| T06 GREEN — FindByBundle returns nil | DONE | `registry.go:140–148` |
| T07 RED — TestParseSkillRef | DONE | 4-row table test passes (`main_test.go:803`) |
| T08 GREEN — parseSkillRef | DONE | `main.go:1055–1061` |
| T09 GREEN — syncRemoteBundlesIfNeeded | DONE | `main.go:1000–1048` |
| T10 GREEN — cmdInstall | DONE | `main.go:823–994` |
| T11 GREEN — Wire install + printUsage | DONE | `main.go:129`, `main.go:1074` |

---

## REQ Coverage Matrix

| REQ | Status | Notes |
|-----|--------|-------|
| REQ-1 Bundle field | PASS | struct + Discover populates correctly |
| REQ-2 FindByBundleAndName | PASS | exact match, case-sensitive |
| REQ-3 FindByBundle | PASS | nil on no match verified |
| REQ-4 install routing after discovery | PASS | `main.go:129` in switch |
| REQ-5 --skill flag | PASS | repeatable, qualified/plain |
| REQ-6 --bundle flag | PASS | combinable, deduped by Path |
| REQ-7 --tool flag | PASS | filters or detects installed tools |
| REQ-8 --scope flag | PASS | default global |
| REQ-9 --yes no-op | PASS | `main.go:854` |
| REQ-10 manual os.Args | PASS | no flag stdlib |
| REQ-11 qualified not found | PASS | error "not found in bundle" |
| REQ-12 plain not found | PASS | error "not found in registry" |
| REQ-13 plain single match | PASS | uses it directly |
| REQ-14 plain ambiguous | PASS | lists bundles, instructs bundle:skill |
| REQ-15 deduplication by Path | PASS | seenPaths map (`main.go:911`) |
| REQ-16 bundle not configured | PASS | error "not configured" |
| REQ-17 bundle already synced | PASS | os.Stat check skips re-sync |
| REQ-18 auto-sync on missing | PASS | C1 resolved — guard bypassed for install |
| REQ-19 no Source → no sync | PASS | bundleByName map skips local bundles |
| REQ-20 missing flags usage error | PASS | `main.go:859` |
| REQ-21 invalid tool | PASS | error "unknown tool" |
| REQ-22 invalid scope | PASS | error "invalid scope ... must be global or project" |
| REQ-23 tui.PrintResults | PASS | `main.go:991` |
| REQ-24 printUsage updated | PASS | "Install skills non-interactively" present |
