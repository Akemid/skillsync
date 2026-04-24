# Apply Progress: upgrade-config-init-warning

## Completed Tasks

- [x] 1.1 Add `MigrationSummary` and `MigrateTools(existing []Tool)` in `internal/config/config.go` with deterministic output order.
- [x] 1.2 Implement legacy transform rule in `internal/config/config.go`: replace `kiro` with `kiro-ide` + `kiro-cli` inheriting paths.
- [x] 1.3 Implement merge-preserve rules in `internal/config/config.go`: keep unknown tools unchanged; do not duplicate `kiro-ide`/`kiro-cli`.
- [x] 1.4 Ensure `MigrateTools` never mutates input slice and sets summary flags/messages (`Changed`, `MigratedLegacy`, `Unchanged`).
- [x] 2.1 Add `upgrade-config` route in `run()` switch in `cmd/skillsync/main.go`.
- [x] 2.2 Create `cmdUpgradeConfig(configPath string) error` in `cmd/skillsync/main.go` to load config, call `config.MigrateTools`, save, and print summary.
- [x] 2.3 Handle missing config in `cmdUpgradeConfig` with clear error aligned to REQ-2 scenario.
- [x] 2.4 Add warning in `cmdInit` path in `cmd/skillsync/main.go`: if config exists, print stderr warning with path and recommendation to run `upgrade-config`.
- [x] 2.5 Keep `init` behavior unchanged after warning: still write default config via `config.Save`.
- [x] 3.1 Add unit tests in `internal/config/config_test.go` for legacy `kiro` migration (REQ-4), including path inheritance.
- [x] 3.2 Add unit tests in `internal/config/config_test.go` for preservation of custom tools and non-default behavior assumptions (REQ-3).
- [x] 3.3 Add unit tests in `internal/config/config_test.go` for idempotency (REQ-5).
- [x] 3.4 Add unit tests in `internal/config/config_test.go` for no-duplicate `kiro-ide`/`kiro-cli` behavior (REQ-4 edge).
- [x] 3.5 Add CLI test in `cmd/skillsync/main_test.go` validating `upgrade-config` success path and stdout summary (REQ-2, REQ-6).
- [x] 3.6 Add CLI test in `cmd/skillsync/main_test.go` validating `upgrade-config` missing-config error (REQ-2 error scenario).
- [x] 3.7 Add CLI tests in `cmd/skillsync/main_test.go` validating `init` warning on existing config and no warning when missing (REQ-1).
- [x] 4.1 Update `README.md` command docs.
- [x] 4.3 Run targeted tests for modified packages.

## Requirement Coverage Matrix

| Requirement | Covered By Tests | Status |
|-------------|------------------|--------|
| REQ-1 init warning when config exists | `TestCmdInit_WarnsWhenConfigAlreadyExists`, `TestCmdInit_NoWarningWhenConfigMissing` in `cmd/skillsync/main_test.go` | ✅ |
| REQ-2 upgrade-config subcommand flow + missing-config error | `TestCmdUpgradeConfig_MigratesLegacyKiro`, `TestCmdUpgradeConfig_MissingConfig` in `cmd/skillsync/main_test.go` | ✅ |
| REQ-3 preserve customizations | `TestCmdUpgradeConfig_MigratesLegacyKiro`, `TestMigrateTools_PreservesCustomAndNoDuplicateSplitEntries` | ✅ |
| REQ-4 migrate legacy `kiro` to `kiro-ide` + `kiro-cli` | `TestMigrateTools_LegacyKiroMigratesWithInheritedPaths`, `TestMigrateTools_PreservesCustomAndNoDuplicateSplitEntries` | ✅ |
| REQ-5 idempotent upgrade-config behavior | `TestMigrateTools_Idempotent` | ✅ |
| REQ-6 human-readable summary output | `TestCmdUpgradeConfig_MigratesLegacyKiro` (asserts migration summary line) | ✅ |

## Notes

- No deviations from design.
- Non-related `.DS_Store` changes were intentionally ignored per user instruction.
