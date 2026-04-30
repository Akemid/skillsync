# Export/Import Specification

## Purpose

Defines behavior for exporting local skills to portable `.tar.gz` archives and importing archives into the local skill registry. No internet or git required.

## Requirements

### Requirement: Skill Export

The system MUST produce a `.tar.gz` archive from a local skill directory. The archive MUST contain the skill directory as its root (e.g., `find-skills/SKILL.md`). The system MUST validate that the skill contains a `SKILL.md` before exporting.

#### Scenario: Export succeeds

- GIVEN skill "find-skills" exists in `~/.agents/skills/`
- WHEN the user runs `skillsync export find-skills`
- THEN a file `find-skills.tar.gz` is created in the current working directory
- AND the CLI prints the output path and file size

#### Scenario: Export with custom output path

- GIVEN skill "find-skills" exists in the local registry
- WHEN the user runs `skillsync export find-skills --output /tmp/my-skill.tar.gz`
- THEN the archive is created at `/tmp/my-skill.tar.gz`

#### Scenario: Export nonexistent skill

- GIVEN no skill named "ghost" exists in `~/.agents/skills/`
- WHEN the user runs `skillsync export ghost`
- THEN the CLI returns an error: skill "ghost" not found

#### Scenario: Export skill missing SKILL.md

- GIVEN a directory named "broken" exists in `~/.agents/skills/` without a `SKILL.md`
- WHEN the user runs `skillsync export broken`
- THEN the CLI returns an error: skill directory has no SKILL.md

---

### Requirement: Skill Import

The system MUST install a skill from a `.tar.gz` archive into `~/.agents/skills/`. The system MUST validate that the archive contains exactly one top-level directory with a `SKILL.md`. All extracted paths MUST be validated as relative and confined within the target directory (path traversal prevention). If a skill with the same name already exists, the import MUST fail unless `--force` is passed.

#### Scenario: Import succeeds

- GIVEN a valid `find-skills.tar.gz` exists at the given path
- AND no skill named "find-skills" exists in `~/.agents/skills/`
- WHEN the user runs `skillsync import find-skills.tar.gz`
- THEN the skill is installed to `~/.agents/skills/find-skills/`
- AND the CLI prints the skill name and description from SKILL.md

#### Scenario: Import with conflicting skill name

- GIVEN skill "find-skills" already exists in `~/.agents/skills/`
- WHEN the user runs `skillsync import find-skills.tar.gz` (no --force)
- THEN the CLI returns an error: skill "find-skills" already installed; use --force to overwrite

#### Scenario: Import archive missing SKILL.md

- GIVEN an archive that does not contain a SKILL.md at the top-level skill directory
- WHEN the user runs `skillsync import bad-archive.tar.gz`
- THEN the CLI returns an error: invalid archive — no SKILL.md found
- AND no files are written to `~/.agents/skills/`

#### Scenario: Import archive with path traversal

- GIVEN an archive containing a path like `../../etc/passwd`
- WHEN the user runs `skillsync import malicious.tar.gz`
- THEN the CLI returns an error: invalid archive — unsafe path detected
- AND no files are written to disk

#### Scenario: Import nonexistent file

- GIVEN the path `/tmp/ghost.tar.gz` does not exist
- WHEN the user runs `skillsync import /tmp/ghost.tar.gz`
- THEN the CLI returns an error: file not found

---

### Requirement: Export/Import Wizard Modes

The TUI wizard MUST include "Export skill" and "Import skill" modes. Both MUST guide the user interactively and confirm before writing to disk.

#### Scenario: Export wizard happy path

- GIVEN at least one skill exists in the local registry
- WHEN the user selects "Export skill" in the wizard
- THEN the wizard prompts to select a skill and confirm the output path
- AND on confirmation runs export and prints the result

#### Scenario: Import wizard happy path

- GIVEN a valid `.tar.gz` archive path is provided
- WHEN the user selects "Import skill" in the wizard
- THEN the wizard shows the skill name and description from the archive
- AND prompts the user to confirm before installing
