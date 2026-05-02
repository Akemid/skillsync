# Technical Design: non-interactive-install

**Change**: `non-interactive-install`
**Date**: 2026-05-01

---

## 1. Key Decisions

| Decision | Chosen | Rejected | Reason |
|---|---|---|---|
| Bundle tracking | `Bundle string` field on `Skill` struct | Separate `map[string][]Skill` | Consistent with existing pattern, no extra indirection |
| `parseSkillRef` location | Private in `main.go` | New `internal/skillref` package | YAGNI — two-line pure function, coordinator owns flag parsing |
| Auto-sync location | Private `syncRemoteBundlesIfNeeded` in `main.go` | Export from `tui` | `tui` must not be imported for side-effect logic; preserves no-circular-dep invariant |
| Default tools | All detected tools when `--tool` omitted | Require `--tool` always | Least-surprise path for automation |
| `--yes` | Accepted, no-op | Prompt for confirmation | Non-interactive never prompts; forward-compatible affordance |
| Flag parsing | Manual `os.Args` | `flag` stdlib | Consistent with all other commands in main.go |

---

## 2. Data Structures

### Skill struct (internal/registry/registry.go)

```go
type Skill struct {
    Name        string
    Description string
    Path        string   // absolute path to skill folder
    Files       []string
    Bundle      string   // "" for local skills; bundle name for remote
}
```

No other struct changes (`installer.Result`, `config.Bundle`, `config.Tool` unchanged).

---

## 3. Function Signatures

```go
// internal/registry/registry.go

func (r *Registry) FindByBundleAndName(bundle, name string) (Skill, bool)
func (r *Registry) FindByBundle(bundle string) []Skill

// cmd/skillsync/main.go

func cmdInstall(cfg *config.Config, reg *registry.Registry, projectDir string) error
func parseSkillRef(s string) (bundle, name string)
func syncRemoteBundlesIfNeeded(cfg *config.Config, bundleNames []string) error
```

---

## 4. cmdInstall Algorithm

```
1. Parse flags from os.Args[2:]:
   skillFlags []string  (repeatable --skill)
   bundleFlag string    (single --bundle)
   toolFlags  []string  (repeatable --tool)
   scopeFlag  string    (default "global")
   --yes               (no-op)

2. Validate: len(skillFlags)==0 && bundleFlag=="" → usage error

3. Collect bundle names needing potential sync:
   - For each s in skillFlags: bundle, _ = parseSkillRef(s); if bundle != "": add
   - If bundleFlag != "": add

4. syncRemoteBundlesIfNeeded(cfg, bundleNames)

5. reg.Discover(cfg.Bundles...)  // re-discover after any sync

6. Resolve skills (deduplicate by Path):
   For each --skill value:
     bundle, name = parseSkillRef(s)
     if bundle != "": FindByBundleAndName → error if not found
     else: scan all with Name==name
       0 matches → error "not found in registry"
       1 match   → use it
       2+ matches → error listing bundles, instruct bundle:skill

   If --bundle: validate configured, FindByBundle → add (deduplicated)

7. Resolve tools:
   --tool given: filter cfg.Tools by name → error if unknown
   omitted: tui.DetectInstalledTools(cfg.Tools)

8. Resolve scope:
   "project" → ScopeProject; else → ScopeGlobal

9. installer.Install(resolved, tools, scope, projectDir)

10. tui.PrintResults(results)
```

---

## 5. parseSkillRef

```go
func parseSkillRef(s string) (bundle, name string) {
    parts := strings.SplitN(s, ":", 2)
    if len(parts) == 2 {
        return parts[0], parts[1]
    }
    return "", parts[0]
}
```

Edge cases:
- `"skill"` → `("", "skill")`
- `"bundle:skill"` → `("bundle", "skill")`
- `"a:b:c"` → `("a", "b:c")` — safe, skill names don't contain colons

---

## 6. syncRemoteBundlesIfNeeded

```
1. Build bundleByName map for bundles where Source != nil
2. registryAbs = expand cfg.RegistryPath; remoteBase = registryAbs/_remote
3. For each name in bundleNames:
   - If not in bundleByName: skip (local bundle)
   - remoteBundleDir = remoteBase/name (+ Source.Path if set)
   - If dir exists: continue
   - Print "Syncing name from URL..."
   - syncer = sync.New(remoteBase)
   - syncer.SyncBundle(ctx, name, url, branch)  [2min timeout]
   - On error: return fmt.Errorf("auto-sync failed for bundle %q: %w\nRun manually: skillsync sync", name, err)
   - Print "✓ name synced"
4. return nil
```

Note: `reg.Discover()` is called once AFTER all syncs complete (not per-bundle).

---

## 7. Flag Parsing Pattern

```go
args := os.Args[2:]
var skillFlags []string
var bundleFlag  string
var toolFlags   []string
scopeFlag := "global"

for i := 0; i < len(args); i++ {
    switch args[i] {
    case "--skill":
        if i+1 < len(args) { i++; skillFlags = append(skillFlags, args[i]) }
    case "--bundle":
        if i+1 < len(args) { i++; bundleFlag = args[i] }
    case "--tool":
        if i+1 < len(args) { i++; toolFlags = append(toolFlags, args[i]) }
    case "--scope":
        if i+1 < len(args) { i++; scopeFlag = args[i] }
    case "--yes":
        // no-op
    }
}
```

---

## 8. Test Strategy

### internal/registry/registry_test.go

**FindByBundleAndName** (table-driven, 4 cases):
- found in bundle → (skill, true)
- wrong bundle → (Skill{}, false)
- name match, empty bundle ("") → (local skill, true)
- not found → (Skill{}, false)

**FindByBundle** (table-driven, 3 cases):
- bundle with 2 skills → returns 2
- "" → returns local skills
- missing bundle → nil

**Discover — Bundle population** (fake fs with os.MkdirTemp):
- `basePath/local-skill/SKILL.md` → Bundle == ""
- `basePath/_remote/acme/remote-skill/SKILL.md` → Bundle == "acme"

### cmd/skillsync/main_test.go

**parseSkillRef** (table-driven, 4 cases — pure function, no I/O):
- "my-skill" → ("", "my-skill")
- "acme:go-testing" → ("acme", "go-testing")
- "a:b:c" → ("a", "b:c")
- "" → ("", "")

### Skipped
- `syncRemoteBundlesIfNeeded` — requires git network
- `cmdInstall` e2e — requires full CLI + real registry

---

## 9. File Map

| File | Change | Reason |
|---|---|---|
| `internal/registry/registry.go` | Modify | `Bundle` field + Discover remote loop + FindByBundleAndName + FindByBundle |
| `cmd/skillsync/main.go` | Modify | `case "install"` + cmdInstall + parseSkillRef + syncRemoteBundlesIfNeeded + printUsage |
| `internal/registry/registry_test.go` | Modify | Tests for Bundle population + new methods |
| `cmd/skillsync/main_test.go` | Modify | Tests for parseSkillRef |

**Not modified**: `internal/installer/installer.go`, `internal/tui/wizard.go`
