# Specification: non-interactive-install

**Change**: `non-interactive-install`
**Status**: spec
**Date**: 2026-05-01

---

## 1. Requirements

### REQ-1 — Skill.Bundle field
The `Skill` struct SHALL gain a `Bundle string` field.
- `Discover()` SHALL set `Bundle` to the bundle directory name when scanning `_remote/{bundleName}/`.
- `Discover()` SHALL leave `Bundle` as `""` for local skills.
- All existing code using named struct literals remains valid (zero value is correct).

### REQ-2 — FindByBundleAndName
`Registry` SHALL expose:
```go
func (r *Registry) FindByBundleAndName(bundle, name string) (Skill, bool)
```
Returns first skill where `Bundle == bundle` AND `Name == name`. Returns `(Skill{}, false)` if not found. Case-sensitive and exact.

### REQ-3 — FindByBundle
`Registry` SHALL expose:
```go
func (r *Registry) FindByBundle(bundle string) []Skill
```
Returns all skills where `Bundle == bundle`. Returns `nil` when none match.

### REQ-4 — install subcommand routing
`skillsync install` SHALL be added to the subcommand switch AFTER registry discovery.

### REQ-5 — --skill flag
Accepts `--skill <value>` zero or more times (repeatable). Each value is either:
- Plain name: `my-skill`
- Qualified: `bundle:skill` (split on first colon only)

### REQ-6 — --bundle flag
Accepts `--bundle <name>`. Installs ALL skills in that bundle. Combinable with `--skill`. Result is deduplicated.

### REQ-7 — --tool flag
Accepts `--tool <name>` zero or more times. When provided: only those tools targeted. When omitted: all tools from `tui.DetectInstalledTools(cfg.Tools)`. Unknown tool name → error.

### REQ-8 — --scope flag
Accepts `--scope global|project`. Default: `global`. Other values → error.

### REQ-9 — --yes flag
Accepted as no-op (forward compatibility). No behavior change.

### REQ-10 — flag parsing style
Manual `os.Args` iteration. `flag` stdlib package SHALL NOT be introduced.

### REQ-11 — qualified syntax: skill not found
`--skill bundle:skill` where skill not in bundle → error: `skill "X" not found in bundle "Y"`

### REQ-12 — plain name: not found
`--skill name` where name not in registry → error: `skill "X" not found in registry`

### REQ-13 — plain name: single match
`--skill name` where exactly one skill matches → use it directly.

### REQ-14 — plain name: ambiguous
`--skill name` where 2+ skills match across different bundles → error listing all bundles and instructing `bundle:skill` syntax.

### REQ-15 — deduplication
Skill list deduplicated by `Skill.Path` before calling `installer.Install()`.

### REQ-16 — --bundle not configured
`--bundle X` where X not in `cfg.Bundles` → error: `bundle "X" not configured`

### REQ-17 — --bundle already synced
`_remote/{name}/` exists → no sync, `FindByBundle` directly.

### REQ-18 — --bundle not synced (auto-sync)
`_remote/{name}/` does NOT exist and bundle has remote `Source` → auto-sync using private helper in `main.go`, re-run `reg.Discover()`, then `FindByBundle`. On failure: wrap error with `"auto-sync failed for bundle %q: %w"`.

### REQ-19 — --bundle no Source (local-only)
No `Source` field → no sync attempt, `FindByBundle` directly.

### REQ-20 — missing required flags
No `--skill` and no `--bundle` → usage error with command signature.

### REQ-21 — invalid tool name
→ error: `unknown tool "X"`

### REQ-22 — invalid scope value
→ error: `invalid scope "X": must be "global" or "project"`

### REQ-23 — output
After install: call `tui.PrintResults(results)`. No new output format.

### REQ-24 — printUsage
Updated to include `skillsync install` with brief description.

**Validation order**: missing flags → invalid scope → invalid tool → unknown bundle → skill resolution errors

---

## 2. Scenarios

### Registry — Bundle field

**S-1**: Local skill → Bundle == ""
**S-2**: `_remote/frontend-cen/react-patterns/` → Bundle == "frontend-cen"
**S-3**: Multiple remote bundles → each skill gets its own bundle name

### FindByBundleAndName

**S-4**: Exact match → (skill, true)
**S-5**: Bundle mismatch → (Skill{}, false)
**S-6**: Name mismatch → (Skill{}, false)
**S-7**: Empty registry → (Skill{}, false)

### FindByBundle

**S-8**: 2 skills in "frontend-cen", 1 in "backend-cen" → FindByBundle("frontend-cen") returns 2
**S-9**: No match → nil
**S-10**: FindByBundle("") returns local skills

### Flag parsing

**S-11**: No flags → usage error
**S-12**: Only --yes → usage error
**S-13**: --scope staging → invalid scope error
**S-14**: --tool ghost-editor → unknown tool error

### Qualified syntax

**S-15**: bundle:skill found → Install called with that skill
**S-16**: bundle synced, skill absent → error "not found in bundle"

### Plain name

**S-17**: Not found → error
**S-18**: Single match → install without qualification
**S-19**: Ambiguous → error listing both bundles + bundle:skill instruction

### Bundle flag

**S-20**: Not configured → error
**S-21**: Already synced → no sync, installs 2 skills
**S-22**: Not synced → auto-sync, re-discover, install
**S-23**: Auto-sync fails → error "auto-sync failed for bundle X"

### Tool filtering

**S-24**: --tool omitted → all detected tools
**S-25**: --tool claude → only claude
**S-26**: Multiple --tool → all listed

### Scope

**S-27**: --scope global → ScopeGlobal
**S-28**: omitted → ScopeGlobal (default)
**S-29**: --scope project → ScopeProject

### Dedup + Output

**S-30**: --bundle + --skill same skill → installed once
**S-31**: Output uses tui.PrintResults()
