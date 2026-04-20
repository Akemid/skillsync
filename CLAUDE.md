# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**skillsync** is a Go CLI tool that manages Agent Skills across multiple agentic coding tools (Claude Code, Copilot, Cursor, etc.) by creating symlinks from a central registry (`~/.agents/skills/`) to each tool's skill directory. It follows the [Agent Skills](https://agentskills.io/) open standard.

## Development Commands

### Build and Run
```bash
# Build the binary
go build -o skillsync ./cmd/skillsync

# Run without building
go run cmd/skillsync/main.go

# Run with specific commands
go run cmd/skillsync/main.go list
go run cmd/skillsync/main.go status
go run cmd/skillsync/main.go init
```

### Testing
```bash
# Test all packages
go test ./...

# Test specific package
go test ./internal/config
go test ./internal/registry

# Test with verbose output
go test -v ./...
```

### Code Quality
```bash
# Format all Go files
go fmt ./...

# Run Go vet (static analysis)
go vet ./...

# Update dependencies
go mod tidy
```

## Architecture Overview

This project follows a **coordinator pattern** where `cmd/skillsync/main.go` orchestrates five independent internal packages:

```
main.go (COORDINATOR)
  ├─→ config      Load YAML configuration
  ├─→ registry    Scan and discover skills from ~/.agents/skills/
  ├─→ detector    Auto-detect project technologies (Go, Python, etc.)
  ├─→ tui         Run interactive wizard (Charm Huh forms)
  └─→ installer   Create/remove symlinks to tool directories
```

### Key Design Principles

1. **Single Responsibility**: Each package has ONE reason to change
   - `config` only changes if YAML format changes
   - `registry` only changes if SKILL.md format changes
   - `installer` only changes if symlink logic changes

2. **No Circular Dependencies**
   - Internal packages don't import each other (except `config`)
   - `main.go` is the ONLY orchestrator
   - This keeps the codebase maintainable and testable

3. **Constructor + Pointer Receiver Pattern**
   ```go
   func New(basePath string) *Registry { ... }
   func (r *Registry) Discover() error { ... }
   ```
   - All packages follow this consistent pattern

4. **Error Wrapping with %w**
   - Errors are wrapped with context: `fmt.Errorf("loading config: %w", err)`
   - Preserves original error for `errors.Is()` checks

## Skill Format (Agent Skills Standard)

Skills are discovered by scanning directories in the registry. Each skill directory must contain:

```
skill-name/
├── SKILL.md          # Required: YAML frontmatter + instructions
├── scripts/          # Optional: executable code
├── references/       # Optional: documentation
└── assets/           # Optional: templates
```

**SKILL.md frontmatter format:**
```yaml
---
name: my-skill
description: What this skill does and when to use it.
---

# Skill instructions here...
```

The registry parser (`internal/registry/registry.go:parseFrontmatter`) extracts this YAML frontmatter to populate skill metadata.

## Configuration

**Default config path**: `~/.config/skillsync/skillsync.yaml`

Override with:
- `--config <path>` flag
- `SKILLSYNC_CONFIG` environment variable

The config defines:
- `registry_path`: Central skill registry location (default: `~/.agents/skills`)
- `bundles`: Pre-configured skill groups (e.g., "cen-core", "shared-tools")
- `tools`: Supported agentic tools with their global/local paths

## How Symlinks Work

The installer creates symlinks from tool directories to the central registry:

```
~/.agents/skills/find-skills/          ← Source (registry)
~/.claude/skills/find-skills           ← Symlink (target)
~/.copilot/skills/find-skills          ← Symlink (target)
```

**Scopes:**
- **Global**: Symlinks created in home directory (e.g., `~/.claude/skills/`)
- **Project**: Symlinks created in current directory (e.g., `.claude/skills/`)

## Technology Detection

The detector (`internal/detector/detector.go`) auto-detects project technologies by looking for indicator files:

- `go.mod` → Go
- `package.json` → TypeScript/JavaScript/Node
- `pyproject.toml` or `requirements.txt` → Python
- `Cargo.toml` → Rust
- `pom.xml` or `build.gradle` → Java

This enables bundle filtering (e.g., only show Python bundles in Python projects).

## Important File Paths

- `cmd/skillsync/main.go` - Entry point, argument parsing, command routing
- `internal/config/config.go` - YAML parsing, config structs
- `internal/registry/registry.go` - Skill discovery, frontmatter parsing
- `internal/detector/detector.go` - Technology detection
- `internal/installer/installer.go` - Symlink creation/removal
- `internal/tui/wizard.go` - Interactive form UI (Charm Huh)

## Common Development Tasks

### Adding a New Tool
1. Add to `config.DefaultTools()` in `internal/config/config.go`
2. Add detection logic in `internal/tui/wizard.go:DetectInstalledTools()`

### Adding Technology Detection
Add indicator to `indicators` map in `internal/detector/detector.go:Detect()`

### Modifying Wizard UI
Edit forms in `internal/tui/wizard.go` using the Charm Huh API

## Documentation Structure

This project is heavily documented to help people learn Go. Each internal package has its own README explaining:
- What the package does
- Go concepts used (with examples)
- How the code works
- Common patterns

**Start here for learning:**
- `docs/GO_FUNDAMENTALS.md` - Quick reference for Go concepts
- `internal/README.md` - Architecture overview and patterns
- `docs/INDEX.md` - Complete learning roadmap

## Dependencies

- `github.com/charmbracelet/huh` - Terminal forms and wizard UI
- `github.com/charmbracelet/lipgloss` - Terminal styling
- `gopkg.in/yaml.v3` - YAML parsing

All UI styling uses Charm's Lipgloss for consistent terminal aesthetics.
