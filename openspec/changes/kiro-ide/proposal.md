# Proposal: kiro-ide — InstallMode field + tool split

## Intent

Kiro IDE no sigue symlinks cuando resuelve directorios de skills. Kiro CLI sí. Hoy skillsync tiene un único entry `kiro` que siempre usa symlinks, lo que hace que los skills instalados sean invisibles para Kiro IDE. El objetivo es soportar instalación por **copia directa** para herramientas que no pueden seguir symlinks, arrancando con Kiro IDE.

## Scope

### In Scope
- Agregar `InstallMode string` a `config.Tool` (`""` / `"symlink"` = default, `"copy"` = copia recursiva)
- Split del entry `kiro` en `kiro-ide` (copy, enabled=true) y `kiro-cli` (symlink, enabled=false)
- `installer.Install()` brancha en `InstallMode`: symlink OR copia recursiva de directorio
- `installer.Uninstall()` maneja dirs reales para copy-mode (safety: verificar SKILL.md antes de RemoveAll)
- Fix `DetectInstalledTools` para no auto-enable ambas entradas que comparten el mismo `GlobalPath`

### Out of Scope
- File watching / auto-sync cuando cambia registry (copies son snapshots)
- Comando `skillsync refresh` para copies stale
- Cambios en el flujo del TUI wizard
- Cambios en `status` command output

## Approach

### 1. config.Tool — add InstallMode (`internal/config/config.go`)

```go
type Tool struct {
    Name        string `yaml:"name"`
    GlobalPath  string `yaml:"global_path"`
    LocalPath   string `yaml:"local_path"`
    Enabled     bool   `yaml:"enabled"`
    InstallMode string `yaml:"install_mode,omitempty"` // "symlink" (default) or "copy"
}
```

Zero-value safe — configs existentes sin este campo siguen funcionando (empty = symlink).

### 2. DefaultTools() — split kiro (`internal/config/config.go`)

```go
{Name: "kiro-ide", GlobalPath: "~/.kiro/skills", LocalPath: ".kiro/skills", Enabled: true,  InstallMode: "copy"},
{Name: "kiro-cli", GlobalPath: "~/.kiro/skills", LocalPath: ".kiro/skills", Enabled: false, InstallMode: "symlink"},
```

`kiro-ide` enabled por defecto (producto primario). `kiro-cli` disabled para evitar conflictos de path.

### 3. installer.Install() — branch on InstallMode (`internal/installer/installer.go`)

- **Symlink path** (existente): sin cambios.
- **Copy path** (nuevo): si target ya existe con `SKILL.md` → idempotent skip. Sino, copia recursiva via `filepath.WalkDir` + `os.MkdirAll` + file copy.
- **Conflicto**: si existe symlink donde se espera dir (o vice versa) → `Result.Error` con mensaje claro.

### 4. installer.Uninstall() — safe dir removal (`internal/installer/installer.go`)

Para copy-mode tools:
1. Verificar que target es dir real (no symlink)
2. Verificar que contiene `SKILL.md` (sentinel check — confirma que es skill gestionado)
3. Solo entonces → `os.RemoveAll(target)`
4. Sin SKILL.md → refuse: `"directory exists but doesn't look like a managed skill, skipping for safety"`

### 5. DetectInstalledTools — fix dual-enable (`internal/tui/wizard.go`)

No auto-enable un tool si otro tool con el mismo `GlobalPath` ya está enabled. Prevents que tanto `kiro-ide` como `kiro-cli` se activen cuando existe `~/.kiro`.

## Impact

| File | Cambio |
|------|--------|
| `internal/config/config.go` | `InstallMode` field en `Tool`; split kiro en `DefaultTools()` |
| `internal/config/config_test.go` | Tests para `InstallMode` YAML parsing; `DefaultTools()` retorna ambos kiro entries |
| `internal/installer/installer.go` | `copySkillDir()` helper; branch en `Install()`; safe dir removal en `Uninstall()` |
| `internal/installer/installer_test.go` | Tests copy-mode install/uninstall, safety checks, conflicto symlink/dir |
| `internal/tui/wizard.go` | Fix `DetectInstalledTools` para shared-path tools |

No hay cambios en `registry`, `detector`, `sync`, ni `main.go`.

## Open Questions

1. **Update mechanism**: copies son snapshots. Cuando cambia un skill en `~/.agents/skills/`, la copia en `~/.kiro/skills/` queda stale. ¿Agregar `skillsync refresh`? → **Diferir**. Documentar limitación. Users pueden re-correr el wizard.

2. **Conflicto mixed mode**: Si user habilita manualmente ambos `kiro-ide` + `kiro-cli`, ambos apuntan al mismo path. → **Detectar en `Install()`**: si target existe con tipo incorrecto (symlink donde se espera dir, o viceversa), retornar error claro.

3. **Config migration**: Configs viejas con `kiro` siguen funcionando (backward compatible). Empty `InstallMode` = symlink. **No se necesita migration**.
