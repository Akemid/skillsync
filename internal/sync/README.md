# internal/sync

Package `sync` handles Git-based bundle synchronization for remote skills.

## What it does

- **Clones** remote Git repositories into `_remote/<bundle-name>/` (first run)
- **Pulls** updates from remote on subsequent runs (`--ff-only` fast-forward only)
- **Validates** Git URLs to reject insecure protocols (only `https://`, `git://`, `git@`, `file://`)
- **Checks** for local modifications before pulling to prevent data loss
- **Atomic operations**: clone to temp dir → rename to final location, so a failed clone never leaves partial data

## Directory layout

Skills are stored under your registry's `_remote/` subdirectory:

```
~/.agents/skills/
└── _remote/
    ├── my-org-skills/   ← cloned with `git clone --depth 1`
    │   └── .git/        ← managed by skillsync, do not edit
    └── team-frontend/
        └── .git/
```

`_remote/` is **read-only** — files inside are managed entirely by this package.

## Go concepts used

### Variable injection for testability

Rather than calling `exec.CommandContext` and `exec.LookPath` directly, the package stores them in package-level variables:

```go
var execCommand = exec.CommandContext
var execLookPath = exec.LookPath
```

Tests swap these out to control what `git` commands do without spawning real processes. This is the standard Go pattern for mocking OS-level calls — no interfaces or DI framework needed.

### Atomic rename

Cloning directly to the final directory would leave a partial directory if the network drops mid-clone. Instead:

```go
tempDir, _ := os.MkdirTemp(s.remoteBaseDir, ".tmp-*")
defer os.RemoveAll(tempDir)  // cleanup if anything fails

// ... git clone into tempDir ...

os.Rename(tempDir, targetDir)  // atomic on same filesystem
```

`os.Rename` is atomic on the same filesystem (POSIX rename(2)), so the final directory either exists fully or not at all.

### Error wrapping with `%w`

All errors are wrapped with context using `fmt.Errorf("doing X: %w", err)`:

```go
return fmt.Errorf("git clone failed: %w\nOutput: %s", err, output)
```

This preserves the original error so callers can use `errors.Is()` or `errors.As()` to inspect it.

## API

```go
// New creates a Syncer. remoteBaseDir is typically {registry_path}/_remote
syncer, err := sync.New("~/.agents/skills/_remote")

// SyncBundle clones or pulls a bundle. branch defaults to "main" if empty.
err = syncer.SyncBundle(ctx, "my-org-skills", "https://github.com/org/skills.git", "main")

// CleanBundle removes a bundle completely (used for --clean flag)
err = syncer.CleanBundle("my-org-skills")
```

## Tests

The package has two test files:

| File | What it tests |
|------|--------------|
| `sync_test.go` | Unit tests with mocked `exec` — no real git needed |
| `integration_test.go` | Real git operations using local `file://` repos — skipped if git not installed |

Run only unit tests:
```bash
go test -run 'TestNew|TestSyncBundle|TestValidate|TestClean' ./internal/sync/
```

Run integration tests too:
```bash
go test -v ./internal/sync/
```
