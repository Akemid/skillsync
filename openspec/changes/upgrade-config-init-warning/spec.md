# Spec: upgrade-config-init-warning

**Change**: upgrade-config-init-warning
**Status**: draft
**Date**: 2026-04-24

---

## Summary

Agregar `skillsync upgrade-config` como subcomando de migración no destructiva de configuración, y emitir un warning explícito cuando `skillsync init` se ejecuta sobre un archivo de config ya existente.

---

## Requirements

### REQ-1: init warning when config exists

`skillsync init` MUST emit a warning to stderr when the target config file already exists, before writing anything.

The warning MUST include the config file path and instruct the user to run `upgrade-config` instead if they want to migrate.

**Rationale**: El comportamiento actual sobreescribe silenciosamente; el usuario puede perder personalizaciones sin darse cuenta.

#### Scenario: init on existing config shows warning

- GIVEN a config file exists at the default path
- WHEN the user runs `skillsync init`
- THEN a warning is printed to stderr mentioning the existing path and suggesting `upgrade-config`
- AND the command still writes the new default config (opt-in overwrite)

#### Scenario: init on missing config proceeds silently

- GIVEN no config file exists
- WHEN the user runs `skillsync init`
- THEN the default config is written with no warning

---

### REQ-2: upgrade-config subcommand exists

`skillsync upgrade-config` MUST be a valid subcommand routed in the CLI.

It MUST load the existing config, apply migration rules, save the result, and print a summary of changes.

#### Scenario: upgrade-config with existing config

- GIVEN a config file exists
- WHEN the user runs `skillsync upgrade-config`
- THEN the config is loaded, migration rules are applied, and the result is saved
- AND a summary of applied changes is printed to stdout

#### Scenario: upgrade-config with no config exits with error

- GIVEN no config file exists
- WHEN the user runs `skillsync upgrade-config`
- THEN the command exits with a clear error message indicating no config to upgrade

---

### REQ-3: merge preserves user customizations

`upgrade-config` MUST NOT remove or modify tool entries that do not match a known legacy pattern.

Bundles, `registry_path`, and unknown tool entries MUST be preserved as-is.

#### Scenario: custom tool entry is preserved

- GIVEN the config contains a tool entry with a name not in DefaultTools
- WHEN upgrade-config is run
- THEN the custom entry is present in the saved config unchanged

#### Scenario: registry_path is preserved

- GIVEN the config has a non-default registry_path value
- WHEN upgrade-config is run
- THEN the saved config retains the original registry_path

---

### REQ-4: legacy kiro entry is migrated

If a tool entry named `kiro` exists in the config (regardless of InstallMode), `upgrade-config` MUST replace it with two entries: `kiro-ide` (InstallMode: copy, Enabled: true) and `kiro-cli` (InstallMode: symlink, Enabled: false), using the same GlobalPath and LocalPath as the original entry.

#### Scenario: legacy kiro entry is replaced

- GIVEN the config contains a tool named `kiro`
- WHEN upgrade-config is run
- THEN the resulting config contains `kiro-ide` and `kiro-cli`
- AND the original `kiro` entry is removed
- AND GlobalPath and LocalPath are inherited from the original `kiro` entry

#### Scenario: kiro-ide already present is not duplicated

- GIVEN the config already contains `kiro-ide` and `kiro-cli`
- WHEN upgrade-config is run
- THEN no duplicate entries are added
- AND the existing entries are preserved unchanged

---

### REQ-5: upgrade-config is idempotent

Running `upgrade-config` multiple times on the same config MUST produce identical results.

#### Scenario: idempotent on already-migrated config

- GIVEN upgrade-config was run once on a config and the result was saved
- WHEN upgrade-config is run again on the same file
- THEN the output config is byte-for-byte equivalent to the previous result
- AND no changes are reported in the summary

---

### REQ-6: migration summary output

`upgrade-config` MUST print a human-readable summary listing each change applied (e.g. "migrated kiro → kiro-ide + kiro-cli") and each section preserved unchanged.

#### Scenario: summary lists each migration

- GIVEN the config has a legacy kiro entry
- WHEN upgrade-config is run
- THEN stdout includes a line indicating the kiro migration was applied

#### Scenario: summary indicates no changes when already current

- GIVEN the config has no legacy entries
- WHEN upgrade-config is run
- THEN stdout indicates no changes were necessary

---

## Files Affected

| File | Change |
|------|--------|
| `cmd/skillsync/main.go` | Route `upgrade-config`; add warning in `cmdInit` |
| `internal/config/config.go` | `MigrateTools()` function |
| `cmd/skillsync/main_test.go` | Tests for warning and upgrade-config command |
| `internal/config/config_test.go` | Tests for MigrateTools: merge rules, legacy kiro, idempotency |
| `README.md` | Document upgrade-config and when to use it vs init |
