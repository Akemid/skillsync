# Technical Design: skillsync-self-skill

## 1. Package Structure

```
internal/skillasset/
  skillasset.go           # embed directive, ExtractTo(), Content()
  skillasset_test.go      # table-driven tests using t.TempDir()
  skill/
    skillsync/
      SKILL.md            # the embedded skill content
```

## 2. Embed Decision: []byte over embed.FS

Single file — `//go:embed skill/skillsync/SKILL.md` into `var skillMD []byte`.

| Factor | `[]byte` | `embed.FS` |
|--------|----------|------------|
| Single file | Direct access | Requires `fs.ReadFile` |
| Content comparison | `bytes.Equal(skillMD, existing)` | Read from FS first |
| Future multi-file | Migration needed | Already supports it |

YAGNI. One file = `[]byte`.

## 3. Exported API Surface

```go
package skillasset

//go:embed skill/skillsync/SKILL.md
var skillMD []byte

// SkillName is the canonical name of the embedded skill.
const SkillName = "skillsync"

// Content returns the raw embedded SKILL.md bytes.
func Content() []byte

// ExtractTo writes the embedded SKILL.md to destDir/skillsync/SKILL.md.
// Creates the skillsync/ subdirectory. Always overwrites (O_TRUNC).
// Returns error if destDir does not exist.
func ExtractTo(destDir string) error
```

**Note**: `ExtractTo` returns only `error` (not `registry.Skill`). Avoids `skillasset` importing `registry`. The caller (`main.go`) constructs `registry.Skill` from the known path and `SkillName` constant.

## 4. tui function

```go
// ConfirmSelfSkillInstall prompts the user to install the skillsync skill.
// Returns true if user accepts, false otherwise (including TUI failures).
// Pure UI — no side effects, no imports of skillasset or installer.
func ConfirmSelfSkillInstall() bool
```

## 5. main.go Integration

After `tui.PrintResults(results)` in `run()`:

```go
// Self-skill install offer
registryPath := config.ExpandPath(cfg.RegistryPath)
selfSkillDir := filepath.Join(registryPath, skillasset.SkillName)
selfSkillMD := filepath.Join(selfSkillDir, "SKILL.md")

alreadyInstalled := false
if existing, err := os.ReadFile(selfSkillMD); err == nil {
    if bytes.Equal(existing, skillasset.Content()) {
        alreadyInstalled = true
    }
}

if !alreadyInstalled {
    if tui.ConfirmSelfSkillInstall() {
        if err := skillasset.ExtractTo(registryPath); err != nil {
            fmt.Fprintf(os.Stderr, "Warning: could not extract skillsync skill: %v\n", err)
        } else {
            selfSkill := registry.Skill{Name: skillasset.SkillName, Path: selfSkillDir}
            selfResults := installer.Install(
                []registry.Skill{selfSkill},
                selectedTools,
                installer.ScopeGlobal,
                "",
            )
            tui.PrintResults(selfResults)
        }
    }
}
```

Imports added to `main.go`: `bytes`, `github.com/Akemid/skillsync/internal/skillasset`

## 6. Dependency Graph

```
main.go
  ├── skillasset  (new — extract embedded skill)
  ├── tui         (confirm prompt)
  ├── installer   (create symlinks)
  ├── registry    (Skill struct)
  └── config      (ExpandPath, tools)

skillasset → stdlib only (embed, fmt, os, path/filepath)
tui        → NO skillasset import
registry   → NO skillasset import
```

Zero circular dependencies.

## 7. Testing Approach

### skillasset_test.go

```go
func TestExtractTo(t *testing.T) {
    tests := []struct {
        name    string
        setup   func(t *testing.T) string
        wantErr bool
    }{
        {name: "happy path creates SKILL.md", setup: func(t *testing.T) string { return t.TempDir() }},
        {name: "idempotent overwrites existing", setup: func(t *testing.T) string {
            dir := t.TempDir()
            skillDir := filepath.Join(dir, "skillsync")
            os.MkdirAll(skillDir, 0755)
            os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("old"), 0644)
            return dir
        }},
        {name: "error when destDir absent", setup: func(t *testing.T) string {
            return "/nonexistent/path/" + t.Name()
        }, wantErr: true},
    }
    // assert: file exists, content == Content(), no error
}

func TestContent_NotEmpty(t *testing.T) {
    c := Content()
    if len(c) == 0 { t.Fatal("empty") }
    if !bytes.HasPrefix(c, []byte("---\n")) { t.Error("missing frontmatter") }
}
```

### ConfirmSelfSkillInstall

Smoke test only (non-TTY → returns false, no panic). Same pattern as existing `TestRunImportWizard_NoPanic`.

## 8. SKILL.md Sections

```
---
name: skillsync
description: ...
---

# skillsync CLI
## What is skillsync
## When to use this skill
## Commands Reference
  ### skillsync (wizard)
  ### init / list / status / sync / upgrade-config
  ### remote add / remote list
  ### uninstall
  ### tap add / tap list / tap remove
  ### upload / export / import
## Configuration
## Common Workflows
```
