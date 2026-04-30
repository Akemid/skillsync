# Tap Model Specification

## Purpose

Defines behavior for registering writable git repositories (taps) and uploading local skills to them.

## Requirements

### Requirement: Tap Registration

The system MUST allow users to register, list, and remove writable git repositories as taps.
A tap MUST have a unique name and a valid git URL. Taps are persisted in `skillsync.yaml` under `taps`.

#### Scenario: Register a new tap

- GIVEN no tap named "my-skills" exists in config
- WHEN the user runs `skillsync tap add my-skills git@github.com:user/my-skills.git`
- THEN the tap is saved to config with name "my-skills", url, and branch "main"
- AND the CLI prints a confirmation message

#### Scenario: Register tap with duplicate name

- GIVEN a tap named "my-skills" already exists in config
- WHEN the user runs `skillsync tap add my-skills <url>`
- THEN the CLI returns an error: tap "my-skills" already registered
- AND config is not modified

#### Scenario: List taps

- GIVEN one or more taps exist in config
- WHEN the user runs `skillsync tap list`
- THEN each tap is printed with its name and URL

#### Scenario: Remove a tap

- GIVEN a tap named "my-skills" exists in config
- WHEN the user runs `skillsync tap remove my-skills`
- THEN the tap is removed from config
- AND the CLI prints a confirmation message

#### Scenario: Remove nonexistent tap

- GIVEN no tap named "ghost" exists in config
- WHEN the user runs `skillsync tap remove ghost`
- THEN the CLI returns an error: tap "ghost" not found

---

### Requirement: Skill Upload

The system MUST upload a local skill to a registered tap by cloning the tap repo, copying the skill directory, committing, and pushing. The operation MUST be atomic via a temporary directory — push failure MUST leave no local residue.

Uploaded skill MUST be placed at `skills/<skill-name>/` inside the tap repo. If the skill already exists in the tap, the CLI MUST return an error unless `--force` is passed.

#### Scenario: Upload succeeds

- GIVEN skill "find-skills" exists in the local registry
- AND tap "my-skills" is registered and push-accessible
- WHEN the user runs `skillsync upload find-skills --to my-skills`
- THEN the skill is pushed to `skills/find-skills/` in the tap repo
- AND the CLI prints receiver instructions (`remote add` + `sync`)

#### Scenario: Upload to unregistered tap

- GIVEN no tap named "ghost" exists in config
- WHEN the user runs `skillsync upload find-skills --to ghost`
- THEN the CLI returns an error: tap "ghost" not found

#### Scenario: Skill not in local registry

- GIVEN no skill named "nonexistent" exists in `~/.agents/skills/`
- WHEN the user runs `skillsync upload nonexistent --to my-skills`
- THEN the CLI returns an error: skill "nonexistent" not found

#### Scenario: Skill already exists in tap

- GIVEN "find-skills" already exists at `skills/find-skills/` in the tap repo
- WHEN the user runs `skillsync upload find-skills --to my-skills` (no --force)
- THEN the CLI returns an error: skill already exists in tap; use --force to overwrite

#### Scenario: Push fails (auth error)

- GIVEN tap "my-skills" is registered but push is rejected by the remote
- WHEN the user runs `skillsync upload find-skills --to my-skills`
- THEN the CLI returns an error with the git error message
- AND no temp directory residue is left on disk

---

### Requirement: Tap Wizard Mode

The TUI wizard MUST include a "Share a skill (tap)" mode. It MUST guide the user through selecting a skill, selecting or registering a tap, and confirming the upload.

#### Scenario: Wizard upload happy path

- GIVEN at least one skill and one tap exist
- WHEN the user selects "Share a skill (tap)" in the wizard
- THEN the wizard prompts to select a skill, then select a tap
- AND on confirmation executes upload and shows receiver instructions

#### Scenario: No tap registered in wizard

- GIVEN no taps exist in config
- WHEN the user selects "Share a skill (tap)" in the wizard
- THEN the wizard prompts the user to register a new tap inline before proceeding
