# Spec: add-skills-to-upload

## Domain: tap-model

### Requirement: Tap Registration
MUST add/list/remove taps. Persisted in skillsync.yaml under `taps`. Unique name + valid git URL required.
Scenarios: register success, duplicate name error, list, remove, remove nonexistent.

### Requirement: Skill Upload
MUST clone tap → copy skill → commit → push (atomic via temp dir). Skill placed at `skills/<name>/`. Conflict = error unless --force.
Scenarios: upload success + receiver instructions, unregistered tap error, skill not found, already exists in tap, push auth failure + no residue.

### Requirement: Tap Wizard Mode
MUST have "Share a skill (tap)" wizard mode. Select skill → select/register tap → confirm → upload → show receiver instructions.
Scenarios: happy path, no tap registered (inline registration).

## Domain: export-import

### Requirement: Skill Export
MUST produce tar.gz from local skill. Archive root = skill dir. MUST validate SKILL.md exists before export.
Scenarios: export success, custom --output path, nonexistent skill, missing SKILL.md.

### Requirement: Skill Import
MUST install from tar.gz to ~/.agents/skills/. MUST validate: one top-level dir, SKILL.md present, no path traversal. Conflict = error unless --force.
Scenarios: import success, conflict error, missing SKILL.md, path traversal rejection, nonexistent file.

### Requirement: Export/Import Wizard Modes
MUST have "Export skill" and "Import skill" wizard modes. Both confirm before writing to disk.
Scenarios: export wizard happy path, import wizard shows skill preview before confirm.
