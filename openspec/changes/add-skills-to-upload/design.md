# Design: add-skills-to-upload

## Key Decisions
- New `internal/tap` package (mirrors internal/sync, inbound/outbound separation)
- New `internal/archive` package (stdlib only: archive/tar + compress/gzip)
- validateGitURL duplicated in tap (8 lines, avoids coupling)
- Upload: clone-to-temp → copy → commit → push; defer RemoveAll (same as cloneBundle)
- Wizard new modes return nil,nil from RunWizard (self-contained, existing contract)
- Path traversal: filepath.Clean + reject .. prefix + reject absolute paths

## New Files
- internal/tap/tap.go — Tapper struct, New, Upload
- internal/tap/tap_test.go
- internal/archive/archive.go — Export, Import
- internal/archive/archive_test.go

## Modified Files
- internal/config/config.go — Tap struct + Taps []Tap `yaml:"taps,omitempty"`
- internal/tui/wizard.go — 3 new wizard modes
- cmd/skillsync/main.go — tap/upload/export/import routing

## Interfaces
```go
type Tap struct { Name, URL, Branch string }
func (t *Tapper) Upload(ctx, tap config.Tap, skillPath, skillName string, force bool) error
func Export(skillPath, outputPath string) error
func Import(archivePath, registryPath string, force bool) (string, error)
```

## Open Questions
- tap add: validate remote reachable at registration or defer? (defer recommended)
- import wizard: file browser or text input? (text input for v1)
