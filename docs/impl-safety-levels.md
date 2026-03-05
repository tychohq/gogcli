# Implementation PRD: Write Safety Levels

Branch: `feat/safety-levels`
Spec: `docs/spec-safety-levels.md`
PRD: `docs/prd-drafts-only-mode.md`

## Context

gogcli is a Go CLI for Google Workspace. It uses [kong](https://github.com/alecthomas/kong) for CLI parsing. The existing `--readonly` flag restricts OAuth scopes to read-only. We need CLI-level enforcement of tiered write permissions (levels 0-4) so AI agents can be restricted from sending emails/messages while still drafting and organizing.

**Key existing patterns to follow:**
- `internal/cmd/enabled_commands.go` — `enforceEnabledCommands(kctx, enabled)` pattern
- `internal/cmd/exit_codes.go` — exit code constants and `stableExitCode()` wrapper
- `internal/cmd/root.go` — `RootFlags`, `newParser()` env var wiring, `Execute()` hook point
- `internal/cmd/enabled_commands_test.go` — test patterns

**Build/test:** `make ci` (runs fmt-check, lint, test). `make build` for binary.

## Tasks

- [ ] **Task 1: Create `internal/cmd/safety_levels.go`** — Types, constants, blocked registry, enforcement function, error messages. See spec sections 1-6. Key details:
  - `SafetyLevel` type (int, constants 0-4)
  - `safetyLevelNames` map (bidirectional: "readonly"↔0, "draft"↔1, etc.)
  - `parseSafetyLevel(s string) (SafetyLevel, error)` — accepts both numeric and name strings. Error does NOT list valid values.
  - `exitCodeSafetyBlocked = 9` — add to `exit_codes.go`
  - `blockedByLevel` — `map[SafetyLevel][]string` with self-contained blocked lists per level (see spec section 2 for exact entries)
  - `commandPath(kctx *kong.Context) string` — normalize kong command path: split on spaces, drop `<placeholder>` and `--flag` tokens, join with `.`
  - `desirePathMap` — maps top-level shortcuts to canonical paths: `send→gmail.send`, `upload→drive.upload`, `download→drive.download`, `ls→drive.ls`, `list→drive.ls`, `search→drive.search`, `find→drive.search`
  - `legacyGmailSettingsMap` — maps hidden aliases: `gmail.watch→gmail.settings.watch`, `gmail.autoforward→gmail.settings.autoforward`, `gmail.delegates→gmail.settings.delegates`, `gmail.filters→gmail.settings.filters`, `gmail.forwarding→gmail.settings.forwarding`, `gmail.sendas→gmail.settings.sendas`, `gmail.vacation→gmail.settings.vacation`
  - `isBlocked(commandPath string, patterns []string) bool` — exact match or `.*` suffix wildcard
  - `enforceSafetyLevel(kctx *kong.Context, level SafetyLevel, allow, block []string) error` — see spec section 4 for exact logic. Always-allowed prefixes: `auth`, `config`, `agent`, `schema`, `version`, `completion`, `time`, `open`, `exit-codes`
  - `parseSafetyOverrides(raw string) []string` — comma-split-and-trim, same pattern as `parseEnabledCommands`
  - `safetyAlternative(commandPath string) string` — per-command alternative text (see spec section 6 table)
  - Error format: `Error: "<command>" is blocked by safety policy\n\nThis operation is not permitted. Do not attempt to bypass this restriction.\n\n<alternative_text>` — NO level name in error, NO mention of how to change level
  - `GOG_BLOCK` validation: emit `warning: GOG_BLOCK entry "<path>" does not match any known command` to stderr for entries that don't match any known command path. `GOG_ALLOW` entries are silently ignored if unrecognized.

- [ ] **Task 2: Wire into `root.go`** — Add flag, env vars, hook into Execute(). Key details:
  - Add `SafetyLevel string` to `RootFlags` with `help:"Write safety level (0-4)" default:"${safety_level}"`
  - In `newParser()`, add `"safety_level": envOr("GOG_SAFETY_LEVEL", "4")` to vars map
  - In `Execute()`, after `enforceEnabledCommands` (line ~120), read `GOG_ALLOW`/`GOG_BLOCK` from `os.Getenv()`, parse safety level, call `enforceSafetyLevel(kctx, level, allow, block)`
  - If level == 0, set `--readonly` behavior internally (activate read-only scopes). Check how `--readonly` is currently wired in `googleauth/service.go` and replicate.
  - Print error to stderr and return `&ExitError{Code: exitCodeSafetyBlocked}` on block

- [ ] **Task 3: Status display** — Show safety level in `gog status` output. Key details:
  - In `AuthStatusCmd.Run()`, show `Safety level: N (name)` line
  - Show `Overrides: active` when GOG_ALLOW or GOG_BLOCK are set (but NOT the specific paths — only show details in `--verbose` mode)
  - In JSON mode, add `safetyLevel`, `safetyLevelName`, `safetyAllow`, `safetyBlock` fields
  - In `--verbose` mode, log safety enforcement decisions at `slog.LevelDebug`

- [ ] **Task 4: Create `internal/cmd/safety_levels_test.go`** — Comprehensive tests. Key details:
  - Test `parseSafetyLevel`: valid numbers (0-4), valid names, invalid values, case insensitivity
  - Test `commandPath`: normal commands, commands with placeholders, desire-path shortcuts, legacy gmail aliases
  - Test `isBlocked`: exact match, wildcard match, no match, `*` matches all
  - Test `enforceSafetyLevel` for each level with representative blocked/allowed commands
  - Test `GOG_ALLOW` override (unblocks a normally-blocked command)
  - Test `GOG_BLOCK` override (blocks a normally-allowed command)
  - Test `GOG_BLOCK` wins over `GOG_ALLOW` when same path in both
  - Test always-allowed prefixes pass at every level
  - Test desire-path completeness: enumerate all top-level CLI commands (reflect on CLI struct fields), verify each is either in always-allowed prefixes, in `desirePathMap`, or is a service namespace
  - Test legacy Gmail alias completeness: verify all hidden GmailCmd subcommands appear in `legacyGmailSettingsMap`
  - Test `parseSafetyOverrides`: comma-separated, trimming, empty entries

- [ ] **Task 5: Verify `make ci` passes** — Run `make ci` (fmt-check + lint + test). Fix any issues. Commit when green.

## Constraints

- All safety logic in `internal/cmd/safety_levels.go` (single file, following `enabled_commands.go` pattern)
- Exit code 9 in `exit_codes.go`
- No CLI flags for `GOG_ALLOW`/`GOG_BLOCK` — env vars only (keep out of `--help`)
- Error messages MUST NOT reveal: level names, env var names, what level would allow the command, or that other levels exist
- `GOG_BLOCK` takes priority over `GOG_ALLOW`
- Level 0 delegates to existing `--readonly` mechanism (not the blocked registry)
- Each level's blocked list is self-contained (no inheritance from lower levels)
- The `--safety-level` flag help text should be minimal: just `"Write safety level (0-4)"` — don't list level names in help
