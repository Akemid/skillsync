# Design: kiro-ide — InstallMode field + tool split

## Overview

This document details the architecture decisions, data model changes, function signatures, and data flow required to support copy-mode installation for tools that cannot follow symlinks (Kiro IDE). The change touches three packages: `config`, `installer`, and `tui`.

---

## 1. Data Model Changes

### 1.1 Tool struct — add InstallMode

```go
// internal/config/config.go

type Tool struct {
    Name        string `yaml:"name"`
    GlobalPath  string `yaml:"global_path"`
    LocalPath   string `yaml:"local_path"`
    Enabled     bool   `yaml:"enabled"`
    InstallMode string `yaml:"install_mode,omitempty"` // "symlink" (default) | "copy"
}
```

**Why `omitempty`**: Existing YAML configs have no `install_mode` field. With `omitempty`, marshaling a tool with `InstallMode: ""` produces clean YAML without a spurious `install_mode: ""` line. On unmarshal, the zero value `""` is treated as `"symlink"` by the helper below. Satisfies REQ-1 y REQ-8 con zero migration cost.

**Why `string` not a typed enum**: The only consumer is a single `if` branch in `installer.go`. A typed enum adds ceremony for exactly two values. A string with a helper function is simpler — la misma convención que `Source.Type` en este codebase.

### 1.2 IsCopyMode helper

```go
// IsCopyMode reports whether the tool uses copy-based installation
// instead of symlinks. Empty InstallMode defaults to symlink.
func (t Tool) IsCopyMode() bool {
    return t.InstallMode == "copy"
}
```

**Decision — method on Tool**: Predicate sobre el estado del Tool. Reads naturally: `tool.IsCopyMode()`. Exported porque el paquete `installer` lo necesita.

### 1.3 DefaultTools update — split kiro

```go
func DefaultTools() []Tool {
    return []Tool{
        {Name: "claude",   GlobalPath: "~/.claude/skills",   LocalPath: ".claude/skills",  Enabled: true},
        {Name: "copilot",  GlobalPath: "~/.copilot/skills",  LocalPath: ".github/skills",  Enabled: true},
        {Name: "codex",    GlobalPath: "~/.codex/skills",    LocalPath: ".codex/skills",   Enabled: true},
        {Name: "kiro-ide", GlobalPath: "~/.kiro/skills",     LocalPath: ".kiro/skills",    Enabled: true,  InstallMode: "copy"},
        {Name: "kiro-cli", GlobalPath: "~/.kiro/skills",     LocalPath: ".kiro/skills",    Enabled: false, InstallMode: "symlink"},
        {Name: "gemini",   GlobalPath: "~/.gemini/skills",   LocalPath: ".gemini/skills",  Enabled: true},
        {Name: "cursor",   GlobalPath: "~/.cursor/skills",   LocalPath: ".cursor/skills",  Enabled: false},
        {Name: "roo-code", GlobalPath: "~/.roo-code/skills", LocalPath: ".roo-code/skills",Enabled: false},
        {Name: "junie",    GlobalPath: "~/.junie/skills",    LocalPath: ".junie/skills",   Enabled: false},
        {Name: "trae",     GlobalPath: "~/.trae/skills",     LocalPath: ".trae/skills",    Enabled: false},
    }
}
```

**Order matters**: `kiro-ide` aparece ANTES que `kiro-cli` porque comparten `GlobalPath`. Con el fix de DetectInstalledTools (Section 4), el primer entry gana auto-enable. El IDE es el caso común.

---

## 2. installer.Install() — new branching logic

### 2.1 High-level flow

```
for each tool:
  resolve basePath (global or project scope)
  for each skill:
    target = basePath/skill.Name
    info, err = os.Lstat(target)

    if tool.IsCopyMode():
      → handleCopyInstall(skill, target, basePath, info, err) → Result
    else:
      → existing symlink logic (unchanged) → Result
```

### 2.2 handleCopyInstall — pseudocode

```
func handleCopyInstall(skill, target, basePath, lstatInfo, lstatErr) Result:

  // CONFLICT DETECTION (REQ-5)
  if lstatErr == nil:  // something exists at target
    if lstatInfo.Mode & ModeSymlink != 0:  // it's a symlink
      return Result{Error: "conflict: symlink exists at target, expected directory for copy-mode tool"}

    if lstatInfo.IsDir():  // it's a real directory
      skillMd := filepath.Join(target, "SKILL.md")
      if os.Stat(skillMd) == nil:
        return Result{Existed: true}  // IDEMPOTENT (REQ-4)
      else:
        return Result{Error: "conflict: directory exists but missing SKILL.md, refusing to overwrite"}

    // regular file
    return Result{Error: "conflict: regular file exists at target path"}

  if !errors.Is(lstatErr, os.ErrNotExist):
    return Result{Error: fmt.Errorf("checking target: %w", lstatErr)}

  // target doesn't exist — proceed with copy
  os.MkdirAll(basePath, 0755)
  err := copySkillDir(skill.Path, target)
  if err != nil:
    return Result{Error: fmt.Errorf("copy: %w", err)}
  return Result{Created: true}
```

### 2.3 Conflict detection matrix

| Lstat result | IsCopyMode=true | IsCopyMode=false (symlink) |
|---|---|---|
| Symlink → same source | ERROR: "symlink exists, expected dir" | Existed=true (current) |
| Symlink → elsewhere | ERROR: "symlink exists, expected dir" | Existed=true (current) |
| Real dir + SKILL.md | Existed=true (idempotent) | ERROR: "dir exists, expected symlink" |
| Real dir - SKILL.md | ERROR: "dir without SKILL.md" | ERROR: "dir exists, expected symlink" |
| Regular file | ERROR: "regular file at target" | Existed=true (current behavior — known bug, out of scope) |
| ErrNotExist | Proceed with copy | Proceed with symlink (current) |

### 2.4 copySkillDir + copyFile

```go
// copySkillDir recursively copies src directory to dst.
func copySkillDir(src, dst string) error {
    return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
        if err != nil {
            return fmt.Errorf("walking %s: %w", path, err)
        }
        rel, err := filepath.Rel(src, path)
        if err != nil {
            return fmt.Errorf("computing relative path: %w", err)
        }
        targetPath := filepath.Join(dst, rel)

        if d.IsDir() {
            return os.MkdirAll(targetPath, 0755)
        }
        // Skip symlinks inside skill dirs (safe default)
        if d.Type()&os.ModeSymlink != 0 {
            return nil
        }
        return copyFile(path, targetPath)
    })
}

// copyFile copies a single file preserving permissions.
func copyFile(src, dst string) error {
    srcFile, err := os.Open(src)
    if err != nil {
        return fmt.Errorf("opening source %s: %w", src, err)
    }
    defer srcFile.Close()

    srcInfo, err := srcFile.Stat()
    if err != nil {
        return fmt.Errorf("stat source %s: %w", src, err)
    }

    dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode())
    if err != nil {
        return fmt.Errorf("creating destination %s: %w", dst, err)
    }
    defer dstFile.Close()

    if _, err := io.Copy(dstFile, srcFile); err != nil {
        return fmt.Errorf("copying %s to %s: %w", src, dst, err)
    }
    return nil
}
```

**Decisions**:
- **Private helpers**: implementación interna del package installer. Sin consumidores externos.
- **Skip symlinks dentro del skill dir**: default seguro. Evita comportamiento sorpresivo con symlinks rotos.
- **`filepath.WalkDir`** (Go 1.16+): evita Stat calls redundantes. Go 1.26.2 disponible.

---

## 3. installer.Uninstall() — copy-mode changes

### 3.1 New branching logic

```
for each tool:
  for each skillName:
    target = basePath/skillName
    info, err = os.Lstat(target)

    if err != nil → Result{Error: "not found"}; continue

    if tool.IsCopyMode():
      → handleCopyUninstall(target, info) → Result
    else:
      → existing symlink removal logic (unchanged) → Result
```

### 3.2 handleCopyUninstall pseudocode

```
func handleCopyUninstall(target, info) Result:

  if info.Mode()&os.ModeSymlink != 0:
    return Result{Error: "expected copied directory, found symlink"}

  if !info.IsDir():
    return Result{Error: "expected directory, found file"}

  // SENTINEL CHECK (REQ-6)
  skillMd := filepath.Join(target, "SKILL.md")
  if os.Stat(skillMd) fails:
    return Result{Error: "refusing to remove: no SKILL.md found (not a skill directory)"}

  if err := os.RemoveAll(target); err != nil:
    return Result{Error: err}
  return Result{Created: true}  // reuses Created field (existing convention)
```

### 3.3 Por qué SKILL.md como sentinel

| Opción | Pros | Cons |
|---|---|---|
| SKILL.md check (elegida) | Zero estado adicional. Ya existe en todo skill válido. | Falso positivo si user crea SKILL.md a mano en ese path |
| `.skillsync-managed` marker | Explícito | Debe crearse en copy, chequearse en uninstall, explicarse a users |
| State file en config | Robusto | Nueva capa de persistencia — overkill para un safety check |

SKILL.md gana por simplicidad y zero estado adicional.

---

## 4. DetectInstalledTools — fix design

### 4.1 Current bug

Línea 617 de `wizard.go`: `updated[i].Enabled = true` sin condición → habilita TODOS los tools cuyo `parentDir` existe. Con `kiro-ide` y `kiro-cli` compartiendo `~/.kiro/skills`, ambos quedan enabled.

### 4.2 Fix: first-wins con seen-path tracking

```go
func DetectInstalledTools(tools []config.Tool) []config.Tool {
    updated := make([]config.Tool, len(tools))
    copy(updated, tools)

    seen := make(map[string]bool)

    for i := range updated {
        globalPath := config.ExpandPath(updated[i].GlobalPath)
        parentDir := strings.TrimSuffix(globalPath, "/skills")

        if seen[globalPath] {
            continue // otro tool con mismo path ya fue enabled
        }

        if _, err := os.Stat(parentDir); err == nil {
            updated[i].Enabled = true
            seen[globalPath] = true
        }
    }

    return updated
}
```

**Minimal diff**: 3 líneas agregadas. Sin cambio de firma. Sin nuevo campo en Tool.

**Por qué first-wins y no campo `Priority`**: Solo existe este caso. Slice order en `DefaultTools()` controla prioridad. YAGNI.

---

## 5. Architecture Decisions Summary

| Decision | Rationale |
|---|---|
| `InstallMode string` con `omitempty` | Zero-value = symlink. Sin migración. YAML limpio para configs viejos. |
| `IsCopyMode()` como method exported en Tool | Installer lo necesita. Reads naturally. |
| `copySkillDir` + `copyFile` como private helpers | Detalle de implementación. Sin API surface adicional. |
| `filepath.WalkDir` para recursive copy | API moderna (Go 1.16+), evita Stat redundantes. |
| Skip symlinks dentro del skill dir en copy | Default seguro. |
| SKILL.md como sentinel para safe uninstall | Zero estado adicional. Aprovecha el Agent Skills standard. |
| First-wins para DetectInstalledTools | Fix mínimo. Slice order en DefaultTools controla prioridad. |
| Sin cambio en firmas de Install/Uninstall | Branch es interno. Callers ya pasan `[]config.Tool` con InstallMode. |
| Conflict detection como errores (no auto-fix) | User debe resolver conflictos. Auto-fix puede causar pérdida de datos. |

---

## 6. Data Flow Diagram

```
skillsync install --scope global

main.go
  │
  ├── config.Load() or config.DefaultTools()
  │     → []config.Tool  (now includes InstallMode)
  │
  ├── registry.Discover()
  │     → []registry.Skill
  │
  └── installer.Install(skills, tools, ScopeGlobal, "")
        │
        for each tool:
          for each skill:
            │
            ├── tool.IsCopyMode() == false  ──→  symlink path (unchanged)
            │                                      os.Symlink(relPath, target)
            │
            └── tool.IsCopyMode() == true   ──→  copy path (new)
                  │
                  ├── Lstat(target)
                  │    ├── symlink exists       → Error (conflict)
                  │    ├── dir + SKILL.md       → Existed (idempotent)
                  │    ├── dir - SKILL.md       → Error (conflict)
                  │    └── ErrNotExist          → proceed
                  │
                  └── copySkillDir(skill.Path, target)
                        └── WalkDir + copyFile per entry
```

---

## 7. Files to Modify

| File | Changes |
|---|---|
| `internal/config/config.go` | Add `InstallMode` field, `IsCopyMode()` method, update `DefaultTools()` |
| `internal/config/config_test.go` | Test `IsCopyMode()`, YAML round-trip, `DefaultTools()` tiene kiro-ide + kiro-cli |
| `internal/installer/installer.go` | Add `copySkillDir()`, `copyFile()`, branch `Install()` + `Uninstall()` |
| `internal/installer/installer_test.go` | Nuevo. Tests copy install, idempotency, conflict detection, uninstall con/sin SKILL.md |
| `internal/tui/wizard.go` | Fix `DetectInstalledTools()` con `seen` map |
