# Tasks: upgrade-config-init-warning

## Phase 1: Foundation (migration contract and helpers)

- [x] 1.1 Add `MigrationSummary` and `MigrateTools(existing []Tool)` in `internal/config/config.go` with deterministic output order.
- [x] 1.2 Implement legacy transform rule in `internal/config/config.go`: replace `kiro` with `kiro-ide` + `kiro-cli` inheriting paths.
- [x] 1.3 Implement merge-preserve rules in `internal/config/config.go`: keep unknown tools unchanged; do not duplicate `kiro-ide`/`kiro-cli`.
- [x] 1.4 Ensure `MigrateTools` never mutates input slice and sets summary flags/messages (`Changed`, `MigratedLegacy`, `Unchanged`).

## Phase 2: CLI implementation (init warning + new subcommand)

- [x] 2.1 Add `upgrade-config` route in `run()` switch in `cmd/skillsync/main.go`.
- [x] 2.2 Create `cmdUpgradeConfig(configPath string) error` in `cmd/skillsync/main.go` to load config, call `config.MigrateTools`, save, and print summary.
- [x] 2.3 Handle missing config in `cmdUpgradeConfig` with clear error aligned to REQ-2 scenario.
- [x] 2.4 Add warning in `cmdInit` path in `cmd/skillsync/main.go`: if config exists, print stderr warning with path and recommendation to run `upgrade-config`.
- [x] 2.5 Keep `init` behavior unchanged after warning: still write default config via `config.Save`.

## Phase 3: Tests (RED → GREEN → REFACTOR)

- [x] 3.1 Add unit tests in `internal/config/config_test.go` for legacy `kiro` migration (REQ-4), including path inheritance.
- [x] 3.2 Add unit tests in `internal/config/config_test.go` for preservation of custom tools and non-default `registry_path` behavior through upgrade flow assumptions (REQ-3).
- [x] 3.3 Add unit tests in `internal/config/config_test.go` for idempotency: second `MigrateTools` run returns byte-equivalent tools and `Unchanged` summary (REQ-5).
- [x] 3.4 Add unit tests in `internal/config/config_test.go` for no-duplicate behavior when `kiro-ide`/`kiro-cli` already exist (REQ-4 edge).
- [x] 3.5 Add CLI test in `cmd/skillsync/main_test.go` validating `upgrade-config` success path and stdout summary (REQ-2, REQ-6).
- [x] 3.6 Add CLI test in `cmd/skillsync/main_test.go` validating `upgrade-config` missing-config error (REQ-2 error scenario).
- [x] 3.7 Add CLI tests in `cmd/skillsync/main_test.go` validating `init` warning on existing config and no warning when missing (REQ-1).

## Phase 4: Documentation and verification

- [x] 4.1 Update `README.md` command docs: when to use `init` vs `upgrade-config` and expected outputs.
- [x] 4.2 Verify requirement coverage matrix manually in `openspec/changes/upgrade-config-init-warning/` before apply complete (REQ-1..REQ-6 mapped to tests).
- [x] 4.3 Run targeted tests for modified packages and confirm all new tests pass (`cmd/skillsync`, `internal/config`).
