# Tasks: Add Skills Upload & Sharing

Strict TDD active: RED (write failing test) → GREEN (make it pass) per task pair.

## Phase 1: Foundation — Config

- [x] 1.1 `internal/config/config.go` — Add `Tap` struct (`Name`, `URL`, `Branch string` with yaml tags) and `Taps []Tap \`yaml:"taps,omitempty"\`` field to `Config`
- [x] 1.2 `internal/config/config_test.go` — RED: `TestTap_RoundTrip` — marshal/unmarshal YAML with `taps` section; assert fields survive; assert omitempty on nil slice
- [x] 1.3 `internal/config/config.go` — GREEN: ensure `Load` passes with `taps` present and absent (omitempty handles it, no code change likely needed — verify test passes)

## Phase 2: Tap Package (TDD)

- [x] 2.1 `internal/tap/tap_test.go` — RED: `TestUpload_Success` — mock `execCommand`; assert clone→copy→commit→push sequence; assert temp dir cleaned up
- [x] 2.2 `internal/tap/tap_test.go` — RED: `TestUpload_SkillAlreadyExists_NoForce` — pre-populate clone with `skills/<name>/SKILL.md`; assert error contains "already exists"
- [x] 2.3 `internal/tap/tap_test.go` — RED: `TestUpload_SkillAlreadyExists_Force` — same setup with `force=true`; assert upload proceeds
- [x] 2.4 `internal/tap/tap_test.go` — RED: `TestUpload_PushFails_NoResidue` — mock push to return error; assert temp dir does not exist after call
- [x] 2.5 `internal/tap/tap.go` — GREEN: `Tapper` struct, `New`, `Upload`; private `validateGitURL` (duplicate from sync); private `copySkillToClone`; `execCommand` var for testability

## Phase 3: Archive Package (TDD)

- [x] 3.1 `internal/archive/archive_test.go` — RED: `TestExport_Success` — create skill dir with SKILL.md + subdir in `t.TempDir()`; assert tar.gz created; assert file tree matches
- [x] 3.2 `internal/archive/archive_test.go` — RED: `TestExport_MissingSkillMD` — dir without SKILL.md; assert error
- [x] 3.3 `internal/archive/archive_test.go` — RED: `TestImport_Success` — export then import; assert skill installed to registry; assert SKILL.md present
- [x] 3.4 `internal/archive/archive_test.go` — RED: `TestImport_PathTraversal` — craft tar entry with `../../etc/passwd`; assert error contains "unsafe path"
- [x] 3.5 `internal/archive/archive_test.go` — RED: `TestImport_MissingSkillMD` — archive without SKILL.md; assert error; assert no files written to registry
- [x] 3.6 `internal/archive/archive_test.go` — RED: `TestImport_Conflict_NoForce` — pre-install skill; import same name; assert error contains "already installed"
- [x] 3.7 `internal/archive/archive.go` — GREEN: `Export`, `Import`; path traversal guard; SKILL.md validation; force flag; atomic extract-then-rename

## Phase 4: Command Wiring

- [x] 4.1 `cmd/skillsync/main.go` — Add `cmdTapAdd`, `cmdTapList`, `cmdTapRemove` reading from `cfg.Taps`, mutating and saving config
- [x] 4.2 `cmd/skillsync/main.go` — Add `cmdUpload` — resolve skill from registry + tap from `cfg.Taps`; call `tap.Tapper.Upload`; print receiver instructions on success
- [x] 4.3 `cmd/skillsync/main.go` — Add `cmdExport` — resolve skill; call `archive.Export`; print output path + size
- [x] 4.4 `cmd/skillsync/main.go` — Add `cmdImport` — call `archive.Import`; print skill name + description on success
- [x] 4.5 `cmd/skillsync/main.go` — Wire `tap`, `upload`, `export`, `import` into `run()` switch; update `printUsage()`

## Phase 5: TUI Wizard Modes

- [x] 5.1 `internal/tui/wizard.go` — Add "Share a skill (tap)" mode: select skill → select tap (or inline register) → confirm → call `tap.Tapper.Upload` → print result
- [x] 5.2 `internal/tui/wizard.go` — Add "Export skill" mode: select skill → confirm output path → call `archive.Export` → print result
- [x] 5.3 `internal/tui/wizard.go` — Add "Import skill" mode: enter file path → preview skill name/description → confirm → call `archive.Import` → print result
- [x] 5.4 `internal/tui/wizard_test.go` — Smoke tests: each new wizard mode callable without panicking with minimal mock inputs
