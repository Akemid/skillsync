---
name: skillsync
description: >-
  Manages Agent Skills across agentic coding tools (Claude Code, Copilot,
  Cursor, Kiro, etc.) by creating symlinks from a central registry
  (~/.agents/skills/) to each tool's skill directory. Skill syncing, remote bundles, taps for sharing, and export/import functionality.
---

# skillsync CLI

## What is skillsync

skillsync is a Go CLI tool that implements the [Agent Skills](https://agentskills.io/) open standard. It manages a central skill registry at `~/.agents/skills/` and creates symlinks from that registry into each configured agentic coding tool's skill directory. This lets you install a skill once and have it available across all your tools simultaneously.

## When to use this skill

Use this skill when the user asks about installing, managing, syncing, or discovering agent skills with the `skillsync` CLI. This includes questions about:
- Installing skills into Claude Code, Copilot, Cursor, Kiro, or other tools
- Listing, inspecting, or removing installed skills
- Adding remote skill repositories (bundles)
- Uploading or sharing skills via taps
- Exporting or importing skill archives
- Configuring or initializing skillsync

## Commands Reference

### `skillsync` (no args) — Interactive Install Wizard

Launches the interactive TUI wizard. Guides you through:
1. Selecting a mode (install skills, add remote, share/export/import)
2. Choosing installation scope (global or project-local)
3. Selecting bundles and individual skills
4. Selecting target tools
5. Confirming and installing

```bash
skillsync
```

### `skillsync init` — Generate Default Config

Creates the default configuration file at `~/.config/skillsync/skillsync.yaml` with sensible defaults. Does not overwrite an existing config.

```bash
skillsync init
```

Use `skillsync upgrade-config` instead if a config already exists.

### `skillsync list` — List Skills in Registry

Displays all skills discovered in the central registry, with their descriptions.

```bash
skillsync list
```

### `skillsync status` — Show Installed Skills per Tool

Shows which skills are currently installed (symlinked) for each configured tool.

```bash
skillsync status
```

### `skillsync sync` — Fetch/Update Remote Bundles

Clones or pulls all configured remote bundles (from Git) into the local registry under `~/.agents/skills/_remote/`.

```bash
skillsync sync
```

### `skillsync upgrade-config` — Migrate Existing Config Safely

Migrates an existing config file to the latest format, adding any missing tools or fields without overwriting your customizations.

```bash
skillsync upgrade-config
```

### `skillsync remote list` — List Configured Remote Bundles

Shows all configured remote Git bundles with their URLs and branches.

```bash
skillsync remote list
```

### `skillsync remote add` — Add a Remote Bundle to Config

Registers a new remote Git repository as a bundle source. After adding, run `skillsync sync` to fetch it.

```bash
skillsync remote add <name> <url> [--branch <branch>] [--path <path>] [--company <company>]
```

Flags:
- `--branch <branch>` — Git branch to track (default: `main`)
- `--path <path>` — Subdirectory inside the repo containing skills
- `--company <company>` — Optional team or company label

### `skillsync uninstall` — Remove a Skill Symlink

Removes the symlink for a specific skill from tool directories.

```bash
skillsync uninstall <skill-name> [--global|--project]
```

Flags:
- `--global` — Remove from global (home) tool directories (default)
- `--project` — Remove from project-local tool directories

### `skillsync tap add` — Register a Writable Git Repo (Tap)

Registers a writable Git repository as a tap — a destination for uploading and sharing your own skills.

```bash
skillsync tap add <name> <url> [--branch <branch>]
```

Flags:
- `--branch <branch>` — Git branch to push to (default: `main`)

### `skillsync tap list` — List Registered Taps

Shows all registered taps with their URLs.

```bash
skillsync tap list
```

### `skillsync tap remove` — Remove a Registered Tap

Unregisters a tap from the config. Does not delete any remote repository.

```bash
skillsync tap remove <name>
```

### `skillsync upload` — Upload a Local Skill to a Tap

Uploads a local skill directory to a registered tap (writable Git repo).

```bash
skillsync upload <skill> --to <tap-name> [--force]
```

Flags:
- `--to <tap-name>` — Target tap name (required)
- `--force` — Overwrite the skill if it already exists in the tap

### `skillsync export` — Export a Skill to Archive

Packages a skill directory into a portable `.tar.gz` archive.

```bash
skillsync export <skill> [--output <path>]
```

Flags:
- `--output <path>` — Output archive path (default: `<skill>.tar.gz`)

### `skillsync import` — Import a Skill from Archive

Extracts a skill from a `.tar.gz` archive into the central registry.

```bash
skillsync import <file.tar.gz> [--force]
```

Flags:
- `--force` — Overwrite if the skill already exists in the registry

### `skillsync help` — Show Help

Prints the full command reference.

```bash
skillsync help
# or
skillsync --help
skillsync -h
```

## Configuration

### Default config path

```
~/.config/skillsync/skillsync.yaml
```

### Override via environment variable

```bash
export SKILLSYNC_CONFIG=/path/to/custom/config.yaml
skillsync list
```

### Override via flag

```bash
skillsync --config /path/to/custom/config.yaml list
```

### Config file structure

```yaml
registry_path: ~/.agents/skills

tools:
  - name: claude
    global_path: ~/.claude/skills
    local_path: .claude/skills
  - name: copilot
    global_path: ~/.copilot/skills
    local_path: .copilot/skills
  - name: cursor
    global_path: ~/.cursor/skills
    local_path: .cursor/skills

bundles: []

taps: []
```

## Registry

### Default location

```
~/.agents/skills/
```

Override with `registry_path` in the config file.

### Skill directory format

Each skill is a directory under the registry containing a `SKILL.md` file:

```
~/.agents/skills/
  my-skill/
    SKILL.md          # required — frontmatter + instructions
    scripts/          # optional
    references/       # optional
    assets/           # optional
```

### SKILL.md frontmatter format

```yaml
---
name: my-skill
description: One-sentence description of what this skill does and when to use it.
---

# Skill instructions here...
```

Both `name` and `description` fields are required and must be valid YAML.

### Installation scopes

- **Global** — symlinks created in `~/<tool-dir>/skills/` (available in all projects)
- **Project** — symlinks created in `./<tool-local-path>/` (project-specific, checked into source control)

## Common Workflows

### Install skills globally for the first time

```bash
skillsync          # launches wizard
# 1. Select "Install skills"
# 2. Choose "Global" scope
# 3. Pick bundles and skills
# 4. Select target tools
# 5. Confirm
```

### Add a remote skill bundle

```bash
skillsync remote add company-skills https://github.com/org/skills.git
skillsync sync
skillsync          # wizard now shows the synced bundle
```

### Share a skill with your team via a tap

```bash
# Register your team's writable repo as a tap
skillsync tap add my-tap https://github.com/org/skill-tap.git

# Upload a skill from your local registry
skillsync upload my-skill --to my-tap

# Teammates install it
skillsync remote add my-tap https://github.com/org/skill-tap.git
skillsync sync
skillsync          # pick the skill from the bundle
```

### Export and import a skill without a tap

```bash
# Export
skillsync export my-skill --output my-skill.tar.gz

# Share the file, then on another machine:
skillsync import my-skill.tar.gz
```

### Remove a skill from all tools

```bash
skillsync uninstall my-skill          # removes global symlinks
skillsync uninstall my-skill --project # removes project-local symlinks
```
