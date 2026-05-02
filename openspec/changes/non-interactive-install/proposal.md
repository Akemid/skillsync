# Proposal: non-interactive-install

## Intent

skillsync currently requires an interactive TUI wizard for all skill installations, making it impossible for AI agents, CI pipelines, or automation scripts to install skills programmatically. This change adds a `skillsync install` subcommand with flags for skill names, bundles, tools, and scope, enabling fully non-interactive installation. It also introduces `bundle:skill` qualified syntax to disambiguate skills that appear in multiple bundles — a problem the current flat registry design silently ignores.

## Scope IN

- `skillsync install` subcommand in `cmd/skillsync/main.go`
- `bundle:skill` qualified syntax for disambiguation
- `FindByBundleAndName()` and `FindByBundle()` methods on `Registry`
- `Bundle string` field on `Skill` struct (set during `Discover()`)
- Auto-sync remote bundles if not yet synced (private helper in main.go)
- Tool filtering by `--tool` flag; default: all detected tools
- Reuse `tui.PrintResults()` for output
- Update `printUsage()` with new command docs
- Flags: `--skill` (repeatable, supports `bundle:skill`), `--bundle`, `--tool`, `--scope global|project`, `--yes`
- Tests: table-driven, stdlib only

## Scope OUT

- No wizard changes
- No `installer.Install()` API changes
- No `--dry-run` flag
- No JSON/machine-readable output
- No project-local skill source via install command

## Approach

1. Add `Bundle string` to `Skill` struct; populate during `Discover()` when scanning `_remote/{bundleName}/`
2. Add `FindByBundleAndName(bundle, name string) (Skill, bool)` and `FindByBundle(bundle string) []Skill` to Registry
3. Add `install` case in main.go subcommand switch → `cmdInstall()` (~60-80 lines)
4. Auto-sync: private helper in main.go using sync package (NOT exported from tui)
5. Ambiguous plain name → clear error listing which bundles contain it
6. Call `installer.Install()` + `tui.PrintResults()` — no new output logic

## Key Decisions

| Decision | Chosen | Rejected | Reason |
|----------|--------|----------|--------|
| Bundle tracking | Skill struct field | Separate map | Simpler, negligible size |
| Auto-sync location | main.go private helper | Export from tui | Keeps tui as UI-only |
| Ambiguity | Error + disambiguation msg | First-match wins | Safety for automation |
| Flag parsing | Manual os.Args | flag stdlib pkg | Consistent with all other commands |
| --yes behavior | Present, no-op (forward compat) | Required | Future: add confirmation prompt |

## Risks

- `Bundle` field addition is non-breaking (all code uses named field literals)
- Flat `FindByNames()` silently returns wrong skill if collision — new code must detect and error
- `reg.Discover()` called twice on auto-sync — acceptable, fast operation
