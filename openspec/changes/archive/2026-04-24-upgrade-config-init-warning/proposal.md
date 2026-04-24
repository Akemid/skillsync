# Proposal: upgrade-config-init-warning

## Intent

Agregar un flujo explícito de migración de configuración para usuarios existentes, evitando sobrescritura accidental del archivo de configuración cuando ejecutan init.

## Problem

- El comando init actualmente escribe configuración default completa y sobrescribe la existente.
- Usuarios con configuración previa pueden perder personalizaciones de bundles y tools.
- Cambios de modelo de tools (por ejemplo el split de kiro a kiro-ide y kiro-cli) no se aplican automáticamente en configs existentes.

## Scope

### In Scope
- Nuevo subcomando upgrade-config para migración no destructiva de configuración.
- Reglas de merge para tools que preserven personalizaciones del usuario.
- Manejo explícito del legacy tool kiro hacia kiro-ide y kiro-cli.
- Warning en init cuando el archivo de config ya existe.
- Mensajes claros de salida sobre cambios aplicados o no aplicados.
- Tests de idempotencia y no-pérdida de datos.

### Out of Scope
- Migración automática silenciosa durante Load.
- Refactor del flujo del wizard.
- Cambios de formato del YAML fuera de migración de tools.

## Approach

### 1. CLI
- Agregar subcomando upgrade-config en el routing principal.
- Mantener init como generador de config, pero mostrar warning cuando el archivo ya existe.

### 2. Config migration logic
- Incorporar helper de migración en internal/config para facilitar testing.
- Aplicar merge no destructivo:
  - Preservar bundles y registry_path existentes.
  - Preservar tools personalizados.
  - Migrar entrada legacy kiro a esquema nuevo solo cuando corresponda.

### 3. UX and safety
- Emitir resumen de migración: cambios aplicados, preservados y omitidos.
- Garantizar idempotencia de upgrade-config.

### 4. Tests
- Tests de cmd para warning de init y ejecución de upgrade-config.
- Tests de config para merge y migración legacy kiro sin pérdida de datos.

## Impacted Areas

- cmd/skillsync/main.go
- cmd/skillsync/main_test.go
- internal/config/config.go
- internal/config/config_test.go
- README.md

## Risks

- Ambigüedad de merge en casos de configuración altamente personalizada.
- Drift futuro si cambian DefaultTools sin actualizar reglas de migración.

## Success Criteria

- upgrade-config migra sin destruir personalizaciones.
- init no sobreescribe en silencio un config existente.
- El flujo es idempotente y está cubierto por tests.
