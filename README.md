# skillsync ⚡

**AI Agent Skills Installer** — Synchronize [Agent Skills](https://agentskills.io/) across all your agentic coding tools.

## What it does

`skillsync` manages a central registry of skills (at `~/.agents/skills/`) and creates **symlinks** into each tool's skill directory, so every tool sees the same skills without duplication.

```
~/.agents/skills/          ← Central registry (source of truth)
├── sdd-init/
├── find-skills/
├── fastapi/
└── ...

~/.claude/skills/find-skills  → ../../.agents/skills/find-skills   (symlink)
~/.copilot/skills/find-skills → ../../.agents/skills/find-skills   (symlink)
~/.codex/skills/find-skills   → ../../.agents/skills/find-skills   (symlink)
~/.kiro/skills/find-skills    → ../../.agents/skills/find-skills   (symlink)
~/.gemini/skills/find-skills  → ../../.agents/skills/find-skills   (symlink)
```

## Supported Tools

All tools following the [Agent Skills](https://agentskills.io/) open standard:

| Tool | Global Path | Project Path |
|------|------------|--------------|
| Claude Code | `~/.claude/skills/` | `.claude/skills/` |
| GitHub Copilot | `~/.copilot/skills/` | `.copilot/skills/` |
| Codex | `~/.codex/skills/` | `.codex/skills/` |
| Kiro | `~/.kiro/skills/` | `.kiro/skills/` |
| Gemini | `~/.gemini/skills/` | `.gemini/skills/` |
| Cursor | `~/.cursor/skills/` | `.cursor/skills/` |
| Roo Code | `~/.roo-code/skills/` | `.roo-code/skills/` |
| Junie | `~/.junie/skills/` | `.junie/skills/` |
| TRAE | `~/.trae/skills/` | `.trae/skills/` |

## Installation

```bash
# Build from source
go build -o skillsync ./cmd/skillsync

# Move to PATH
mv skillsync /usr/local/bin/
```

## Quick Start

```bash
# 1. Generate default config
skillsync init

# 2. Edit config with your bundles
$EDITOR ~/.config/skillsync/skillsync.yaml

# 3. Run interactive wizard
skillsync
```

## Usage

```
skillsync              Run interactive TUI wizard
skillsync list         List all skills in registry
skillsync status       Show installed skills per tool
skillsync uninstall    Remove a skill's symlinks
skillsync init         Generate default config
skillsync help         Show help
```

### Interactive Wizard

The wizard guides you through:

1. **Scope** — Global (home dir) or Project (current dir)
2. **Selection** — Choose a bundle or pick individual skills
3. **Tools** — Select which agentic tools to install into (auto-detected)
4. **Confirm** — Review and execute

### Config File

Default location: `~/.config/skillsync/skillsync.yaml`

Override with `--config <path>` or `SKILLSYNC_CONFIG` env var.

See [skillsync.example.yaml](skillsync.example.yaml) for a full example.

## How Skills Work

Skills follow the [Agent Skills](https://agentskills.io/) open standard:

```
my-skill/
├── SKILL.md          # Required: YAML frontmatter + instructions
├── scripts/          # Optional: executable code
├── references/       # Optional: documentation
└── assets/           # Optional: templates
```

## 📚 Documentation for Developers

This project is fully documented to help you learn Go and understand the codebase:

- **[Go Fundamentals](docs/GO_FUNDAMENTALS.md)** — Quick reference for Go concepts (slices, maps, pointers, structs, etc.)
- **[Architecture Overview](internal/README.md)** — How all the pieces fit together
- **Package Documentation:**
  - [cmd/skillsync](cmd/skillsync/README.md) — Entry point and main flow
  - [internal/config](internal/config/README.md) — Configuration management
  - [internal/registry](internal/registry/README.md) — Skill discovery and management
  - [internal/detector](internal/detector/README.md) — Technology detection
  - [internal/installer](internal/installer/README.md) — Symlink creation
  - [internal/tui](internal/tui/README.md) — Interactive terminal UI

Each README explains:
- What the package does
- Go concepts used (with examples)
- How the code works
- Common patterns and best practices

The `SKILL.md` frontmatter:

```yaml
---
name: my-skill
description: What this skill does and when to use it.
---

# Instructions here...
```

## License

MIT
