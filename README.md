<div align="center">

# skillsync ⚡

---

[![Go 1.26+](https://img.shields.io/badge/go-1.26+-00ADD8.svg)](https://go.dev/dl/) [![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE) [![Agent Skills](https://img.shields.io/badge/standard-agentskills.io-blueviolet.svg)](https://agentskills.io/) [![Release](https://img.shields.io/github/v/release/Akemid/skillsync)](https://github.com/Akemid/skillsync/releases/latest) [![CI](https://github.com/Akemid/skillsync/actions/workflows/ci.yml/badge.svg)](https://github.com/Akemid/skillsync/actions/workflows/ci.yml)

<img src="assets/logo.png" alt="skillsync logo" width="300"/>
<br/><br/>

**AI Agent Skills Sync** — Synchronize [Agent Skills](https://agentskills.io/) across all your agentic coding tools.

</div>

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

### macOS / Linux

```sh
curl -fsSL https://raw.githubusercontent.com/Akemid/skillsync/main/install.sh | sh
```

### Windows (PowerShell)

```powershell
irm https://raw.githubusercontent.com/Akemid/skillsync/main/install.ps1 | iex
```

> **Note for Windows users**: `skillsync` uses symlinks. Enable Developer Mode in Settings → Privacy & Security → For developers, or run as Administrator.

### Build from source

```bash
go build -o skillsync ./cmd/skillsync
mv skillsync /usr/local/bin/
```

## Quick Start

```bash
# 1. Generate default config
skillsync init

# If config already exists and you only want migration-safe updates
skillsync upgrade-config

# 2. Edit config with your bundles
$EDITOR ~/.config/skillsync/skillsync.yaml

# 3. Run interactive wizard
skillsync
```

## Usage

```
skillsync                 Run interactive TUI wizard
skillsync list            List all skills in registry
skillsync status          Show installed skills per tool
skillsync sync            Fetch/update remote bundles from Git
skillsync uninstall       Remove a skill's symlinks
skillsync init            Generate default config
skillsync upgrade-config  Migrate existing config without destructive overwrite
skillsync help            Show help
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

## Remote Bundles

Remote bundles let you pull skills from any Git repository. This is useful for sharing skills across teams without a central registry.

### How it works

1. **Configure** — Add a bundle with `source.type: git` to your config
2. **Sync** — Run `skillsync sync` to clone/pull the repository
3. **Install** — Run `skillsync` (wizard) or `skillsync install` to symlink skills

Skills are cloned into `_remote/<bundle-name>/` inside your registry path:

```
~/.agents/skills/
├── sdd-init/           ← local skill (managed by you)
├── find-skills/        ← local skill (managed by you)
└── _remote/            ← managed by skillsync (do not edit)
    ├── my-org-skills/  ← cloned from Git
    └── team-frontend/  ← cloned from Git
```

> **`_remote/` is read-only** — never edit files inside it. Re-run `skillsync sync` to update.

### Bundle configuration reference

Every bundle (local or remote) supports these fields:

| Field | Required | Description |
|-------|----------|-------------|
| `name` | yes | Unique identifier used as the directory name under `_remote/` |
| `description` | no | Human-readable summary shown in the wizard |
| `company` | no | Organization label (displayed in the TUI) |
| `tags` | no | String list for filtering (e.g. `["cloud", "azure"]`) |
| `tech` | no | Technology stack hints (e.g. `["go", "python"]`) |
| `source` | no | Omit for local bundles; add for Git-backed bundles |
| `skills` | yes | List of `{ name: "skill-name" }` entries to install |

#### `source` block parameters

| Parameter | Required | Default | Description |
|-----------|----------|---------|-------------|
| `type` | yes | — | Only `"git"` is supported |
| `url` | yes | — | HTTPS or SSH remote URL |
| `branch` | no | `"main"` | Branch, tag, or commit SHA to track |
| `path` | no | repo root | Subdirectory **inside** the repo that contains the skill folders |

When `path` is set, skillsync descends into that subdirectory after cloning and treats it as the skill root. Use this when your skills live in a sub-folder of a larger monorepo.

### Configuration examples

**HTTPS — public repo, skills at repo root:**
```yaml
bundles:
  - name: "company-shared"
    description: "Shared skills from company Git repo"
    tags: ["company", "shared"]
    source:
      type: "git"
      url: "https://github.com/your-org/agent-skills.git"
      branch: "main"
    skills:
      - name: "onboarding"
      - name: "code-review"
```

**SSH — private repo, skills in a subdirectory:**
```yaml
bundles:
  - name: "team-frontend"
    description: "Frontend team skills"
    company: "Acme"
    tags: ["frontend", "team"]
    tech: ["typescript"]
    source:
      type: "git"
      url: "git@github.com:your-org/monorepo.git"
      branch: "stable"
      path: "tools/agent-skills"   # skills live inside tools/agent-skills/
    skills:
      - name: "design-system"
      - name: "accessibility"
```

**Pinned to a tag or commit SHA:**
```yaml
bundles:
  - name: "org-skills-pinned"
    description: "Pinned to a stable release"
    source:
      type: "git"
      url: "https://github.com/your-org/agent-skills.git"
      branch: "v1.2.0"   # tag or commit SHA both work
    skills:
      - name: "code-review"
```

### HTTPS vs SSH authentication

| Method | URL format | When to use |
|--------|-----------|-------------|
| HTTPS  | `https://github.com/org/repo.git` | Public repos, CI/CD with tokens |
| SSH    | `git@github.com:org/repo.git`     | Private repos with SSH key configured |

**HTTPS with token** (e.g., for private repos in CI):
```bash
# Set credential helper before running sync
git config --global credential.helper store
echo "https://token:YOUR_TOKEN@github.com" >> ~/.git-credentials
skillsync sync
```

**SSH** (recommended for developer machines):
```bash
# Ensure your SSH key is added
ssh-add ~/.ssh/id_ed25519
skillsync sync
```

### Running a sync

```bash
# Fetch/update all remote bundles
skillsync sync
```

Output example:
```
Syncing 2 remote bundle(s)...
  Syncing my-org-skills from https://github.com/your-org/agent-skills.git...
  ✓ my-org-skills synced
  Syncing team-frontend from git@github.com:your-org/frontend-skills.git...
  ✓ team-frontend synced
```

Exit code `1` is returned if any bundle fails — safe for use in CI pipelines.

### Troubleshooting

**Authentication failure**
```
git clone failed: exit status 128
```
- HTTPS: verify your credentials or token has `read` access to the repo
- SSH: run `ssh -T git@github.com` to verify your key is recognised

**Network timeout**
Each bundle has a 2-minute timeout. If your repo is large or your connection is slow:
- Use `--depth 1` (already the default — shallow clone)
- Check firewall/proxy settings: `git config --global http.proxy <proxy>`

**Local modifications detected**
```
bundle "my-org-skills" has local modifications — remove them or run: skillsync sync --clean
```
You (or another process) edited a file inside `_remote/`. Since `_remote/` is managed by skillsync, you should discard those changes:
```bash
cd ~/.agents/skills/_remote/my-org-skills
git checkout .
skillsync sync
```

### Rollback procedure

If a sync introduces a breaking change, roll back to the previous commit:

```bash
cd ~/.agents/skills/_remote/my-org-skills
git log --oneline -5            # find the good commit
git checkout <commit-hash>      # detach HEAD to that commit
```

To pin permanently, set `branch` in your config to a specific tag or commit SHA:
```yaml
source:
  url: "https://github.com/your-org/agent-skills.git"
  branch: "v1.2.0"   # pin to a tag
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
