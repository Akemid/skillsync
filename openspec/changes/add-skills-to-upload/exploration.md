# Exploration: add-skills-to-upload

## Current State

skillsync manages skills in **one direction today: inbound only**.

- `internal/registry/registry.go` — scans `~/.agents/skills/` locally. Read-only.
- `internal/sync/sync.go` — `git clone --depth 1` or `git pull --ff-only`. No push, no write to remotes.
- `internal/config/config.go` — `Bundle.Source` has `Type` (only "git"), `URL`, `Branch`, `Path`. Read-only sources.
- `internal/installer/installer.go` — symlinks or copies to tool paths. Pure local.
- `cmd/skillsync/main.go` — commands: `list`, `status`, `sync`, `remote add/list`, `uninstall`, `init`, `upgrade-config`. No upload/publish/share.

The receive side (`remote add` + `sync`) **already works today**. The gap is purely the publish/upload side.

## Affected Areas

- `cmd/skillsync/main.go` — new `upload` and optional `tap` command routing + `cmdUpload()` impl
- `internal/config/config.go` — `UploadTarget` struct, tap registry config
- `internal/publisher/publisher.go` — NEW package for outbound git ops (clone, copy, commit, push)
- `internal/tui/wizard.go` — optional "Share a skill" wizard mode
- `go.mod` — only if GitHub API approach chosen (not recommended)

## Approaches

### 1. Git Tap Model (Homebrew-inspired) ⭐ RECOMMENDED

User creates a "tap" repo with a known layout: `skills/<skill-name>/SKILL.md`.

- `skillsync tap add <user/repo>` — registers as a remote bundle (thin alias over existing `remote add`)
- `skillsync upload <skill> --to <tap-name>` — clones tap, copies skill dir, commits, pushes
- Receiver consumes via `skillsync remote add <url>` + `skillsync sync` — **already works, zero new code**

**Pros**: Receiver side fully implemented. Builds directly on `Bundle`/`Source` infrastructure. No server. Opinionated convention enables discoverability.
**Cons**: User needs a git repo and push credentials (SSH key or HTTPS token).
**Effort**: Medium — new `internal/publisher` package + `upload` + optional `tap` commands.

### 2. Git Repository (user-configured, no convention)

- User adds `upload_target` to config pointing to any git repo they own
- `skillsync upload <skill> --to <remote>` copies and pushes

**Pros**: Flexible.
**Cons**: No discoverability without convention. Functionally weaker than Option 1.
**Effort**: Medium (same impl work, worse UX).

### 3. GitHub Gist

- `skillsync upload <skill>` creates/updates a Gist
- `skillsync install --gist <url>` imports

**Pros**: Simple sharing URL.
**Cons**: Gists are flat — skill dirs have nested structure. Requires GitHub token. Breaks existing sync mechanism (Gists ≠ git remotes). New importer needed from scratch.
**Effort**: High. Custom API client, dir serialization, new importer, new dependency.

### 4. Local Export/Import (tar.gz) ⭐ RECOMMENDED SECONDARY

- `skillsync export <skill-name> [--output skill.tar.gz]`
- `skillsync import skill.tar.gz`

**Pros**: Zero external deps. No internet. Works air-gapped (email, Slack). Stdlib only.
**Cons**: No versioning. Manual distribution. Doesn't compose with remote bundle system.
**Effort**: Low — two commands in `main.go`, `archive/tar` + `compress/gzip`. No new package needed.

### 5. Dedicated Registry Server (agentskills.io)

- Central hosted backend, npm-like UX
- **Effort**: Very High — requires building + operating infrastructure. Out of scope.

## Recommendation

**Primary: Option 1 (Git tap model) + Secondary: Option 4 (local export/import)**

The tap model is right because the receiver side is already done. `upload` only implements the publisher side — a new `internal/publisher` package that mirrors `internal/sync` structure (clone → copy → commit → push, with temp-dir cleanup). Option 4 is additive, orthogonal, stdlib-only, and handles the "share via Slack" case.

## Risks

- **Git credentials**: push requires pre-configured SSH/HTTPS. Need clear error messages on auth failure.
- **Conflict on existing skill in tap**: needs `--force` flag or explicit error, not silent overwrite.
- **Tap layout validation**: must validate expected layout on `tap add` to avoid confusing empty syncs.
- **Publisher not atomic**: clone→copy→commit→push sequence; push rejection leaves stale temp clone. Mirror `cloneBundle` temp-dir cleanup pattern from `internal/sync`.
- **TUI cross-import pre-existing violation**: `wizard.go` already imports `sync`, `installer`, `registry`, `detector`. Not a new risk, but worth noting.

## Ready for Proposal

Yes. Affected areas are bounded, recommendation is clear, receiver side is already shipped.
