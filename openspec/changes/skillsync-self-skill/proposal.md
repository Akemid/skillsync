# Proposal: skillsync-self-skill

## Intent

Ship a built-in "skillsync" agent skill that teaches AI agents how to use the skillsync CLI. When a user runs the install wizard, they get offered the option to also install this self-describing skill. The skill content is embedded in the binary via Go `embed`, so it works offline with zero external dependencies.

**Why**: New users install skillsync but their AI agent has no idea the tool exists. The agent cannot help with `skillsync sync`, `skillsync tap add`, etc. unless it has a skill describing those commands. Shipping the skill inside the binary closes this bootstrap gap.

## Scope

### IN

- New `internal/skillasset/` package that embeds `internal/skillasset/skill/skillsync/SKILL.md` and exposes `ExtractTo(destDir string) (registry.Skill, error)`
- New `internal/skillasset/skill/skillsync/SKILL.md` — full CLI reference (Agent Skills standard frontmatter + body)
- Wizard modification: one confirm step after `tui.PrintResults()` — "Install the skillsync skill? It teaches your AI agent how to use this CLI."
- If yes: extract to `~/.agents/skills/skillsync/` then symlink via existing `installer.Install()` (global scope only)
- Tests for `internal/skillasset/` (extract, overwrite, content verification)

### OUT

- No new CLI subcommand (wizard-only)
- No per-project scope for the self-skill (always global)
- No auto-install without user consent
- No config schema changes

## Approach: Go embed

Embed `SKILL.md` in binary at compile time. `ExtractTo` writes the file to the registry directory and returns a `registry.Skill` struct for the caller to pass to `installer.Install()`.

**Rationale**: Zero runtime deps, atomic versioning with the binary, fully testable via `embed.FS`.

**Alternatives rejected**:
- Download from GitHub at runtime: network dependency, failure modes
- Ship as separate file: distribution burden, version mismatch risk

## Key Design Decisions

### Package structure

`internal/skillasset/skillasset.go` — `//go:embed` directive + `ExtractTo()` logic
`internal/skillasset/skill/skillsync/SKILL.md` — the actual skill (embed path constraint: must be under the package dir)

### Integration point

`cmd/skillsync/main.go` after `tui.PrintResults(results)` — calls `tui.AskSelfSkillInstall(cfg, selectedTools, registryPath)`

### AskSelfSkillInstall

Lives in `internal/tui/wizard.go`. Checks if skill already installed (skip prompt if identical content). Shows `huh.Confirm`. If yes: calls `skillasset.ExtractTo()` + `installer.Install()` with global scope.

### ExtractTo behavior

- Creates `destDir/skillsync/` if absent
- Writes `SKILL.md`
- Idempotent: no-op if content matches; overwrite if newer binary version

## Files

| File | Action |
|------|--------|
| `internal/skillasset/skill/skillsync/SKILL.md` | CREATE |
| `internal/skillasset/skillasset.go` | CREATE |
| `internal/skillasset/skillasset_test.go` | CREATE |
| `cmd/skillsync/main.go` | MODIFY — post-install hook |
| `internal/tui/wizard.go` | MODIFY — `AskSelfSkillInstall()` |

## Risks

1. Embed path constraint: file MUST be under `internal/skillasset/`
2. Skill content drift if CLI commands change without updating SKILL.md
3. Registry collision with existing third-party "skillsync" skill
