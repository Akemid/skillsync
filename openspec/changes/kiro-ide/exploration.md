## Exploration: upgrade-config command and re-init warning

### Current State
The command skillsync init is handled early in run() and always writes a full default config, replacing the existing file without merge semantics. Existing users keep legacy tool entries (for example kiro) because config.Load() only applies DefaultTools() when tools is empty. There is currently no migration command and no warning path when init is executed against an already existing config.

### Affected Areas
- cmd/skillsync/main.go — command routing and cmdInit() behavior need warning and upgrade-config subcommand wiring.
- internal/config/config.go — migration logic should live here to keep merge rules testable and reusable.
- cmd/skillsync/main_test.go — needs coverage for upgrade-config flow and init warning behavior.
- internal/config/config_test.go — needs coverage for non-destructive migration rules and legacy kiro split behavior.
- README.md — CLI docs should explain when to use init vs upgrade-config.

### Approaches
1. Dedicated upgrade-config command + warning-only init
   - Pros: safest UX, no silent mutations, explicit operator intent, preserves backward compatibility.
   - Cons: adds one new command and migration logic surface.
   - Effort: Medium

2. Merge behavior directly into init (implicit upgrade)
   - Pros: fewer commands, users learn one flow.
   - Cons: ambiguous behavior, surprising side effects, harder to preserve current init semantics.
   - Effort: Medium

3. Automatic migration during config.Load()
   - Pros: transparent for users, no extra command invocation.
   - Cons: hidden write side effects unless write-back is implemented, harder to reason about and test safely.
   - Effort: High

### Recommendation
Use approach 1: add skillsync upgrade-config and keep init as a generator with a clear warning when config already exists. This keeps behavior explicit, avoids destructive overwrites by default, and solves the kiro to kiro-ide/kiro-cli migration problem for existing users without breaking current scripts.

### Risks
- Migration rule drift if DefaultTools() evolves and upgrade logic is not updated with tests.
- User confusion between init and upgrade-config if help text and README are not updated together.

### Ready for Proposal
Yes. Tell the user the change is well-scoped and should include: explicit subcommand, deterministic merge rules (especially legacy kiro split), dry output summary, and tests for idempotency and no-data-loss guarantees.
