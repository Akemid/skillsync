# Spec: kiro-ide — InstallMode field + tool split

**Change**: kiro-ide
**Status**: draft
**Date**: 2026-04-24

---

## Summary

Add an `InstallMode string` field to `config.Tool` para control per-tool sobre si los skills se instalan via symlink o copia recursiva. Split del entry `kiro` en `DefaultTools()` en dos: `kiro-ide` (copy mode, enabled=true) y `kiro-cli` (symlink mode, enabled=false). Actualizar `installer.Install()` y `installer.Uninstall()` para branchear en este campo. Fix `DetectInstalledTools` para prevenir dual-enable de entries que comparten el mismo `GlobalPath`.

---

## Requirements

### REQ-1: InstallMode field backward compatibility

`config.Tool.InstallMode` MUST default to symlink behavior when the field is absent or empty.

**Rationale**: Configs escritas antes de este cambio no tienen `install_mode`; el campo debe ser aditivo y no-breaking.

---

### REQ-2: kiro-ide usa copy mode

`DefaultTools()` MUST include a `kiro-ide` entry con `InstallMode: "copy"` y `Enabled: true`.

**Rationale**: Kiro IDE no sigue symlinks — los skills deben copiarse físicamente para ser visibles.

---

### REQ-3: kiro-cli usa symlink mode (disabled por defecto)

`DefaultTools()` MUST include a `kiro-cli` entry con `InstallMode: "symlink"` (o empty) y `Enabled: false`.

**Rationale**: Kiro CLI sí sigue symlinks. Disabled by default para evitar conflicto de path con kiro-ide.

---

### REQ-4: Install copy-mode behavior

Cuando `tool.InstallMode == "copy"`, `installer.Install()` MUST copiar recursivamente el directorio fuente al destino en lugar de crear un symlink, y MUST ser idempotente: si ya existe un dir real con `SKILL.md` en el target, la operación MUST skipear sin error.

**Rationale**: Copia recursiva es el único mecanismo que hace accesibles los skills a tools que no traversan symlinks. Idempotency permite correr Install() repetidamente sin side effects.

---

### REQ-5: Install conflict detection

`installer.Install()` MUST retornar error para un par (tool, skill) cuando detecta mismatch de modo en el target path:
- Existe symlink donde se espera dir (copy mode).
- Existe dir real donde se espera symlink (symlink mode).

El mensaje de error MUST indicar claramente el tipo de conflicto.

**Rationale**: Mismatches silenciosos producen fallas no obvias en runtime.

---

### REQ-6: Uninstall copy-mode safety

`installer.Uninstall()` MUST verificar que existe `SKILL.md` directamente dentro del directorio target antes de llamar `os.RemoveAll`. Si `SKILL.md` está ausente, la eliminación MUST ser rechazada con error descriptivo.

**Rationale**: `os.RemoveAll` en un directorio arbitrario es destructivo e irreversible. La presencia de `SKILL.md` es la señal de que el dir fue creado por skillsync.

---

### REQ-7: DetectInstalledTools shared-path conflict prevention

Cuando dos o más tool entries resuelven al mismo `GlobalPath`, `DetectInstalledTools` MUST habilitar a lo sumo uno. El primer entry por índice en el slice MUST ganar; todos los siguientes que compartan ese path MUST quedar disabled.

**Rationale**: `kiro-ide` y `kiro-cli` resuelven ambos a `~/.kiro/skills`. Habilitar ambos produciría conflicto de mixed-mode en install time (REQ-5).

---

### REQ-8: Backward compatibility con configs que tienen el viejo entry `kiro`

Un config que contenga un entry `kiro` (sin `install_mode`) MUST instalar skills via symlink sin requerir ninguna migración, warning, ni acción del usuario.

**Rationale**: Usuarios existentes no deben romperse. El viejo `kiro` era implícitamente symlink, y empty `InstallMode` mapea a symlink (REQ-1).

---

## Scenarios

### SCENARIO-1: Install kiro-ide skill (copia creada, no symlink)

```
Given: kiro-ide tool entry con InstallMode:"copy", GlobalPath:"~/.kiro/skills"
       skill "find-skills" existe en ~/.agents/skills/find-skills/ con SKILL.md
       ~/.kiro/skills/find-skills/ NO existe
When:  installer.Install() es llamado con kiro-ide y el skill find-skills
Then:  ~/.kiro/skills/find-skills/ es un directorio real (no symlink)
       contiene los mismos archivos que ~/.agents/skills/find-skills/ (copia recursiva)
       Result.Created == true
       Result.Error == nil
```

---

### SCENARIO-2: Install kiro-cli skill (symlink creado, no copia)

```
Given: kiro-cli tool entry con InstallMode:"" (symlink), GlobalPath:"~/.kiro/skills"
       skill "find-skills" existe en ~/.agents/skills/find-skills/
       ~/.kiro/skills/find-skills NO existe
When:  installer.Install() es llamado con kiro-cli y el skill find-skills
Then:  ~/.kiro/skills/find-skills es un symlink apuntando a ~/.agents/skills/find-skills/
       NO es un directorio real
       Result.Created == true
       Result.Error == nil
```

---

### SCENARIO-3: Install kiro-ide skill que ya existe con SKILL.md (idempotent skip)

```
Given: ~/.kiro/skills/find-skills/ es un directorio real que contiene SKILL.md
       kiro-ide InstallMode:"copy"
When:  installer.Install() es llamado de nuevo para el mismo skill
Then:  no se modifican ni re-copian archivos
       Result.Existed == true
       Result.Created == false
       Result.Error == nil
```

---

### SCENARIO-4: Install kiro-ide cuando existe symlink en el target (conflict error)

```
Given: ~/.kiro/skills/find-skills es un symlink (remanente de install previo con symlinks)
       kiro-ide InstallMode:"copy"
When:  installer.Install() es llamado para find-skills
Then:  Result.Error es non-nil
       mensaje indica conflicto de modo ("symlink found, copy expected" o equivalente)
       no se modifican archivos
       el symlink existente queda intacto
```

---

### SCENARIO-5: Uninstall kiro-ide skill con SKILL.md presente (RemoveAll exitoso)

```
Given: ~/.kiro/skills/find-skills/ es un directorio real que contiene SKILL.md
       kiro-ide InstallMode:"copy"
When:  installer.Uninstall() es llamado para find-skills
Then:  ~/.kiro/skills/find-skills/ y todo su contenido son eliminados
       Result.Created == true  (campo reutilizado para indicar remoción exitosa)
       Result.Error == nil
```

---

### SCENARIO-6: Uninstall kiro-ide directorio sin SKILL.md (rechazado por seguridad)

```
Given: ~/.kiro/skills/find-skills/ es un directorio real
       NO contiene SKILL.md
       kiro-ide InstallMode:"copy"
When:  installer.Uninstall() es llamado para find-skills
Then:  el directorio NO es eliminado (sin llamada a RemoveAll)
       Result.Error es non-nil
       mensaje indica el rechazo de seguridad
       ("SKILL.md not found, refusing to remove directory" o equivalente)
```

---

### SCENARIO-7: DetectInstalledTools con ~/.kiro presente (solo kiro-ide enabled)

```
Given: slice de tools contiene kiro-ide en index 0, kiro-cli en index 1
       ambos tienen GlobalPath resolviendo a ~/.kiro/skills
       ~/.kiro/ existe en disco
When:  DetectInstalledTools(tools) es llamado
Then:  kiro-ide.Enabled == true   (primer match gana)
       kiro-cli.Enabled == false  (path duplicado suprimido)
```

---

### SCENARIO-8: Config viejo con entry `kiro` (funciona sin cambios, symlink behavior)

```
Given: config del usuario contiene:
         tools:
           - name: kiro
             global_path: ~/.kiro/skills
             enabled: true
       sin campo install_mode (Go zero value: empty string tras unmarshal)
When:  installer.Install() es llamado para cualquier skill usando este tool entry
Then:  se crea un symlink en ~/.kiro/skills/<skill-name>
       Result.Created == true
       Result.Error == nil
       no se emite ningún prompt de migración, warning, ni error
```

---

## Out of Scope

- File watching o auto-refresh cuando cambia el skill fuente después de una copy install
- Comando `skillsync refresh` para actualizar copies stale
- Cambios en el flujo del TUI wizard
- Cambios en output de `status` / `list`
- Detección de copies stale (diferido; documentado como limitación conocida)

## Known Limitation

Las copies no se actualizan automáticamente cuando cambia el skill fuente en `~/.agents/skills/`. Los usuarios deben desinstalar y reinstalar manualmente para propagar actualizaciones a targets de copy-mode.

---

## Files Affected

| File | Change |
|------|--------|
| `internal/config/config.go` | Add `InstallMode string` to `Tool` struct; update `DefaultTools()` |
| `internal/installer/installer.go` | Branch `Install()` y `Uninstall()` on `InstallMode` |
| `internal/tui/wizard.go` | Fix `DetectInstalledTools` shared-path dedup logic |
