# Skill Registry

Auto-generated registry of available skills and conventions for this project.

## Available Skills

### go-testing
**Path**: `~/.claude/skills/go-testing/SKILL.md`
**Description**: Go testing patterns for Gentleman.Dots, including Bubbletea TUI testing. Trigger: When writing Go tests, using teatest, or adding test coverage.

### skill-creator
**Path**: `~/.claude/skills/skill-creator/SKILL.md`
**Description**: Creates new AI agent skills following the Agent Skills spec. Trigger: When user asks to create a new skill, add agent instructions, or document patterns for AI.

### context7-mcp
**Path**: `~/.claude/skills/context7-mcp/SKILL.md`
**Description**: Fetch current library/framework docs via Context7 MCP. Trigger: setup questions, code generation involving libraries, API references.

### judgment-day
**Path**: `~/.claude/skills/judgment-day/SKILL.md`
**Description**: Parallel adversarial review — two blind judge agents review simultaneously. Trigger: "judgment day", "review adversarial", "dual review".

### branch-pr
**Path**: `~/.claude/skills/branch-pr/SKILL.md`
**Description**: PR creation workflow following issue-first enforcement. Trigger: creating a pull request or preparing changes for review.

### issue-creation
**Path**: `~/.claude/skills/issue-creation/SKILL.md`
**Description**: Issue creation workflow following issue-first enforcement. Trigger: creating a GitHub issue, reporting a bug, requesting a feature.

## Compact Rules (for sub-agent injection)

### Go code (*.go)
- Coordinator pattern: main.go is the ONLY orchestrator; internal packages must NOT import each other (except config)
- Constructor + pointer receiver: `func New(...) *T` + `func (t *T) Method()`
- Error wrapping: `fmt.Errorf("context: %w", err)` — preserve original for errors.Is()
- Test style: table-driven, standard library only (`go test ./...`)
- Quality: `go fmt ./...` + `go vet ./...` before any commit

### Testing (*.go in *_test.go)
- Table-driven tests; no external test frameworks
- Prefer `t.TempDir()` for filesystem fixtures
- Use `testify` only if already present (it is NOT in this project)

## Project Conventions

### CLAUDE.md
**Path**: `CLAUDE.md`
**Type**: Project-level AI instructions
**Summary**: Development commands, architecture overview (coordinator pattern), design principles (Single Responsibility, No Circular Dependencies), skill format (Agent Skills standard), configuration, symlink mechanics, technology detection, common development tasks for skillsync CLI.

---

*Last updated: 2026-04-28*
