# Design: upgrade-config-init-warning

## Technical Approach

Se implementa un flujo explícito de migración por comando (`upgrade-config`) en lugar de migración implícita en `Load()`. Esto mantiene `Load()` simple y backward-compatible, y evita side effects durante operaciones normales del CLI.

El cambio se divide en dos piezas:
1. Warning de seguridad en `init` si ya existe config.
2. Migración no destructiva en `internal/config` invocada desde `cmdUpgradeConfig`.

Este enfoque mapea directo a REQ-1..REQ-6 del spec: warning, subcomando, merge preservando customizaciones, migración `kiro`, idempotencia y summary legible.

## Architecture Decisions

| Decision | Option | Tradeoff | Choice | Rationale |
|----------|--------|----------|--------|-----------|
| Punto de migración | `Load()` automático vs comando explícito | automático reduce pasos pero introduce efectos colaterales globales | Comando explícito `upgrade-config` | Más seguro y auditable; solo migra cuando el usuario lo pide. |
| Dónde vive la lógica | `cmd` vs `internal/config` | en `cmd` es rápido pero difícil de testear; en `config` es reusable/testable | `internal/config` | Permite tests unitarios puros y separa orquestación de reglas de dominio. |
| Estrategia de merge | sobrescribir con defaults vs merge conservador | sobrescribir simplifica pero rompe setups personalizados | Merge conservador | Prioriza no pérdida de datos y cumple REQ-3. |
| Migración `kiro` | ignorar legacy vs convertir a dos entries | ignorar deja drift; convertir agrega complejidad | Convertir a `kiro-ide` + `kiro-cli` | Alinea config existente con modelo actual sin requerir reinstall manual. |

## Data Flow

`skillsync init`

    run()
      -> cmdInit(cfg, configPath)
         -> os.Stat(configPath)
         -> [exists] print warning to stderr (path + suggest upgrade-config)
         -> config.Save(defaultCfg, configPath)

`skillsync upgrade-config`

    run()
      -> cmdUpgradeConfig(configPath)
         -> config.Load(configPath)
         -> config.MigrateTools(cfg.Tools)
         -> assign migrated tools to cfg.Tools
         -> config.Save(cfg, configPath)
         -> print migration summary

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `cmd/skillsync/main.go` | Modify | Agregar routing `upgrade-config`; agregar warning de existencia en `cmdInit`; imprimir summary de migración. |
| `cmd/skillsync/main_test.go` | Modify | Tests del subcomando y warning en init con config existente/no existente. |
| `internal/config/config.go` | Modify | Agregar función de migración no destructiva de tools + estructura de summary. |
| `internal/config/config_test.go` | Modify | Tests de migración legacy `kiro`, preservación de custom tools/registry_path, no duplicados e idempotencia. |
| `README.md` | Modify | Documentar cuándo usar `init` vs `upgrade-config`. |

## Interfaces / Contracts

```go
// MigrationSummary reports what changed during upgrade-config.
type MigrationSummary struct {
	Changed         bool
	MigratedLegacy  bool
	AddedTools      []string
	PreservedTools  []string
	Unchanged       bool
}

// MigrateTools applies non-destructive migration rules to tool entries.
// It never mutates input slice and returns deterministic order.
func MigrateTools(existing []Tool) ([]Tool, MigrationSummary)
```

Rules contract for `MigrateTools`:
- Preserve unknown/custom tools unchanged.
- Preserve existing `kiro-ide`/`kiro-cli` entries unchanged.
- If `kiro` exists, remove it and ensure `kiro-ide` + `kiro-cli` exist.
- `kiro-ide`/`kiro-cli` inherited paths come from legacy `kiro` when generated.
- Re-running over an already migrated config yields identical output.

## Testing Strategy

| Layer | What to Test | Approach |
|-------|-------------|----------|
| Unit (`internal/config`) | Merge conservador, migración legacy `kiro`, no duplicados, orden determinista, idempotencia | Table-driven tests para `MigrateTools` con fixtures YAML-like. |
| Integration-lite (`cmd/skillsync`) | Routing de `upgrade-config`, warning de `init`, summary output | Tests de comando llamando funciones `cmd*` y capturando stdout/stderr con temp config files. |
| Regression | Compatibilidad con config sin `install_mode` | Caso legacy `kiro` + fields mínimos, validando que no se pierdan propiedades de paths. |

## Migration / Rollout

No migration automática en runtime. Rollout manual y seguro:
1. Usuario actualiza binario.
2. Corre `skillsync upgrade-config` una vez.
3. Verifica summary; puede re-ejecutar sin impacto (idempotente).

## Open Questions

- [ ] ¿El summary debe incluir diff por campo (verbose) o solo acciones de alto nivel?
- [ ] ¿Queremos flag `--dry-run` en una iteración posterior para previsualizar cambios sin guardar?
