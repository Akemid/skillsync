# Tasks: kiro-ide — InstallMode field + tool split

## Phase 1: Foundation (config)

- [x] 1.1 `internal/config/config.go: Tool struct` — Add `InstallMode string \`yaml:"install_mode,omitempty"\`` field after `Enabled bool`
- [x] 1.2 `internal/config/config.go: Tool.IsCopyMode()` — Add exported method `func (t Tool) IsCopyMode() bool { return t.InstallMode == "copy" }` after the Tool struct definition
- [x] 1.3 `internal/config/config.go: DefaultTools()` — Replace el entry `kiro` con dos: `kiro-ide` (InstallMode:"copy", Enabled:true) y `kiro-cli` (InstallMode:"symlink", Enabled:false), mismos paths `~/.kiro/skills` / `.kiro/skills`

## Phase 2: Installer helpers

- [x] 2.1 `internal/installer/installer.go: copyFile(src, dst string) error` — Helper privado: abre src, crea dst heredando permisos de src stat, stream via `io.Copy`; errores wrapeados con `fmt.Errorf("copyFile %s→%s: %w", ...)`
- [x] 2.2 `internal/installer/installer.go: copySkillDir(src, dst string) error` — Helper privado: `os.MkdirAll(dst, 0755)` + `filepath.WalkDir(src, fn)` donde fn recrea estructura de subdirs, skippea symlinks (`d.Type()&fs.ModeSymlink != 0`), llama `copyFile` para archivos regulares

## Phase 3: Installer core

- [x] 3.1 `internal/installer/installer.go: Install() — copy-mode Existed path` — En el bloque Lstat-succeeded (línea 52): si `tool.IsCopyMode()` y el entry NO es symlink, hacer stat de `filepath.Join(target, "SKILL.md")`; si existe → `res.Existed = true; continue`
- [x] 3.2 `internal/installer/installer.go: Install() — copy-mode conflict guard` — En el mismo bloque: si `tool.IsCopyMode()` y el entry ES symlink → `res.Error = fmt.Errorf("conflict: symlink found where directory expected for copy-mode tool %s", tool.Name); continue`
- [x] 3.3 `internal/installer/installer.go: Install() — copy-mode Created path` — Después de `os.MkdirAll(basePath)`: si `tool.IsCopyMode()` → llamar `copySkillDir(skill.Path, target)`; éxito → `res.Created = true`; error → `res.Error`; `continue` para saltear el bloque de symlink
- [x] 3.4 `internal/installer/installer.go: Uninstall() — copy-mode branch` — Después del `os.Lstat(target)` (línea 116): si `tool.IsCopyMode()` y target es dir real → stat `SKILL.md`; si existe → `os.RemoveAll(target)` + `res.Created = true`; si no existe → `res.Error = fmt.Errorf("refusing to remove %s: no SKILL.md sentinel", target)` + `continue`; si target es symlink → fallthrough al `os.Remove` existente

## Phase 4: TUI fix

- [x] 4.1 `internal/tui/wizard.go: DetectInstalledTools()` — Agregar `seen := make(map[string]bool)` antes del loop (línea 614); dentro del loop, después de computar `globalPath`, agregar: `if seen[globalPath] { continue }`; después de `updated[i].Enabled = true` agregar: `seen[globalPath] = true`

## Phase 5: Tests

- [x] 5.1 `internal/config/config_test.go: TestToolIsCopyMode` — Table-driven: `"copy"→true`, `""→false`, `"symlink"→false`
- [x] 5.2 `internal/config/config_test.go: TestDefaultTools_KiroEntries` — Llamar `DefaultTools()`; assert `kiro-ide` existe con `InstallMode=="copy"` y `Enabled==true`; assert `kiro-cli` existe con `InstallMode!="copy"`; assert NO existe entry `kiro`
- [x] 5.3 `internal/config/config_test.go: TestLoad_InstallModeRoundTrip` — (a) YAML con `install_mode: copy` → Load → field sobrevive; (b) YAML sin `install_mode` → Load → field es `""`
- [x] 5.4 `internal/installer/installer_test.go` — Nuevo archivo, package `installer_test`; agregar helper `makeSkillDir(t, base, name)` que crea `base/name/SKILL.md` y `base/name/script.sh`, retorna el path
- [x] 5.5 `internal/installer/installer_test.go: TestInstall_CopyMode_Created` — Tool kiro-ide copy-mode; skill con makeSkillDir; llamar Install(); assert `Result.Created==true`, `Error==nil`; verificar que SKILL.md y script.sh son archivos reales (no symlinks via Lstat+ModeSymlink)
- [x] 5.6 `internal/installer/installer_test.go: TestInstall_CopyMode_Idempotent` — Mismo setup; llamar Install() dos veces; assert segunda llamada retorna `Existed==true`, `Error==nil`
- [x] 5.7 `internal/installer/installer_test.go: TestInstall_CopyMode_ConflictSymlink` — Pre-crear target como symlink; llamar Install() con copy-mode; assert `Error!=nil` y mensaje contiene `"conflict"`
- [x] 5.8 `internal/installer/installer_test.go: TestInstall_SymlinkMode_Unchanged` — Tool sin InstallMode; llamar Install(); assert target es symlink via `info.Mode()&os.ModeSymlink != 0`
- [x] 5.9 `internal/installer/installer_test.go: TestUninstall_CopyMode_WithSentinel` — Crear dir real con SKILL.md; llamar Uninstall() con copy-mode; assert `Result.Created==true`; assert dir ya no existe
- [x] 5.10 `internal/installer/installer_test.go: TestUninstall_CopyMode_NoSentinel` — Crear dir real SIN SKILL.md; llamar Uninstall(); assert `Error!=nil` y contiene `"refusing"`; assert dir sigue existiendo
