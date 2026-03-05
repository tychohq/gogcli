# Technical Spec: Write Safety Levels

**PRD:** [docs/prd-drafts-only-mode.md](prd-drafts-only-mode.md)

---

## 1. New Files, Types, and Constants

### File: `internal/cmd/safety_levels.go`

All safety-level logic lives in a single file, following the pattern of `enabled_commands.go`.

#### `SafetyLevel` type

```
type SafetyLevel int

const (
    SafetyLevelReadonly      SafetyLevel = 0
    SafetyLevelDraft         SafetyLevel = 1
    SafetyLevelCollaborate   SafetyLevel = 2
    SafetyLevelStandard      SafetyLevel = 3
    SafetyLevelUnrestricted  SafetyLevel = 4
)
```

#### Level name map

A bidirectional mapping between numeric levels and string names. Used for flag parsing (`--safety-level=draft` or `--safety-level=1`) and display.

```
var safetyLevelNames = map[SafetyLevel]string{
    SafetyLevelReadonly:     "readonly",
    SafetyLevelDraft:        "draft",
    SafetyLevelCollaborate:  "collaborate",
    SafetyLevelStandard:     "standard",
    SafetyLevelUnrestricted: "unrestricted",
}
```

A `parseSafetyLevel(s string) (SafetyLevel, error)` function accepts both numeric strings (`"1"`) and names (`"draft"`). Returns a clear error on invalid input that does **not** suggest valid values (to avoid leaking level names to agents).

#### Exit code constant

```
exitCodeSafetyBlocked = 9
```

Added to `exit_codes.go` alongside the existing codes. Code 9 is the next available slot after `exitCodeConfig = 10` (but numerically before it; 9 is unused). This gives agents a stable, machine-readable signal that a command was rejected by safety policy — distinct from usage errors (2), auth errors (4), and permission errors (6).

### File: `internal/cmd/safety_levels_test.go`

Unit tests for all exported and internal functions.

---

## 2. Blocked Command Registry

### Data structure

The registry is a package-level `map[SafetyLevel][]string` where each value is a list of **blocked command path patterns**. Patterns use dot-separated segments matching the kong command path.

```
var blockedByLevel = map[SafetyLevel][]string{
    SafetyLevelReadonly: {"*"},  // special: block everything (--readonly handles this)
    SafetyLevelDraft: {
        "gmail.send",
        "gmail.drafts.send",
        "gmail.batch.delete",
        "gmail.track.*",
        "gmail.settings.vacation.*",
        "gmail.settings.sendas.*",
        "gmail.settings.delegates.*",
        "gmail.settings.forwarding.*",
        "gmail.settings.autoforward.*",
        "gmail.settings.watch.*",
        "chat.messages.send",
        "chat.dm.send",
        "chat.spaces.create",
        "drive.share",
        "drive.unshare",
        "drive.comments.*",
        "docs.comments.*",
        "calendar.respond",
        "appscript.run",
    },
    SafetyLevelCollaborate: {
        "gmail.send",
        "gmail.drafts.send",
        "gmail.batch.delete",
        "gmail.track.*",
        "gmail.settings.sendas.*",
        "gmail.settings.delegates.*",
        "gmail.settings.forwarding.*",
        "gmail.settings.autoforward.*",
        "gmail.settings.watch.*",
        "chat.messages.send",
        "chat.dm.send",
        "chat.spaces.create",
    },
    SafetyLevelStandard: {
        "gmail.batch.delete",
        "gmail.settings.delegates.*",
        "gmail.settings.forwarding.*",
        "gmail.settings.autoforward.*",
        "gmail.settings.watch.*",
    },
    // SafetyLevelUnrestricted: no entries — nothing blocked
}
```

Each level's list is **self-contained** (not inherited from the previous level). This makes lookups O(1) per level and avoids ordering bugs. The trade-off is some duplication, but the list is small and static.

### Pattern semantics

- `"gmail.send"` — exact match on command path `gmail send`
- `"gmail.settings.*"` — matches `gmail settings` and all subcommands
- `"*"` — matches everything (used only for level 0)

No other glob syntax is supported. The `*` wildcard only appears as the final segment.

---

## 3. Command Path Matching

### Normalizing kong command paths

`kctx.Command()` returns a string like `"gmail send <query>"` — space-separated, with argument placeholders. The enforcement function must:

1. Call `kctx.Command()` to get the raw path
2. Split on spaces
3. Drop any token that starts with `<` (argument placeholder) or `--` (flag remnant)
4. Join remaining tokens with `.` to produce the **canonical command path** (e.g., `"gmail.send"`, `"gmail.settings.filters.list"`)

This normalization function is `commandPath(kctx *kong.Context) string`.

### Kong aliases

Kong aliases (e.g., `mail` for `gmail`, `cal` for `calendar`) are resolved by kong before `kctx.Command()` is called. The command string always uses the **primary name** (`name:"..."` tag), never the alias. No alias-to-primary mapping is needed.

### Top-level desire-path shortcuts

The CLI defines top-level shortcuts that duplicate nested commands:

| Shortcut | Equivalent |
| --- | --- |
| `send` | `gmail send` |
| `upload` | `drive upload` |
| `download` | `drive download` |
| `ls` / `list` | `drive ls` |
| `search` / `find` | `drive search` |
| `status` / `st` | `auth status` |
| `login` | `auth add` |
| `logout` | `auth remove` |
| `me` / `whoami` | `people me` |

These are separate kong commands (e.g., `GmailSendCmd` embedded as both `CLI.Send` and `GmailCmd.Send`). Kong resolves `kctx.Command()` to the **top-level** name: `"send <query>"` not `"gmail send <query>"`.

The enforcement function must map top-level shortcuts to their canonical nested path **before** matching against the blocked registry:

```
var desirePathMap = map[string]string{
    "send":     "gmail.send",
    "upload":   "drive.upload",
    "download": "drive.download",
    "ls":       "drive.ls",
    "list":     "drive.ls",
    "search":   "drive.search",
    "find":     "drive.search",
}
```

Shortcuts that map to read-only or auth commands (`status`, `login`, `logout`, `me`, `whoami`, `open`) are never blocked and do not need entries.

### Legacy top-level settings aliases

Gmail settings subcommands have hidden top-level aliases (e.g., `gmail watch` → `gmail settings watch`). Kong resolves these as `"gmail watch <...>"`. The enforcement function must also map these:

```
var legacyGmailSettingsMap = map[string]string{
    "gmail.watch":       "gmail.settings.watch",
    "gmail.autoforward": "gmail.settings.autoforward",
    "gmail.delegates":   "gmail.settings.delegates",
    "gmail.filters":     "gmail.settings.filters",
    "gmail.forwarding":  "gmail.settings.forwarding",
    "gmail.sendas":      "gmail.settings.sendas",
    "gmail.vacation":    "gmail.settings.vacation",
}
```

### Matching algorithm

```
func isBlocked(commandPath string, patterns []string) bool
```

For each pattern in the level's blocked list:
1. If pattern is `"*"` → blocked
2. If pattern ends with `".*"` → check if `commandPath` equals the prefix or starts with `prefix.`
3. Otherwise → exact string equality

---

## 4. Enforcement Function

### Signature

```
func enforceSafetyLevel(kctx *kong.Context, level SafetyLevel, allow, block []string) error
```

### Parameters

- `kctx` — kong parse context (provides `Command()`)
- `level` — the resolved safety level (already parsed from flag/env)
- `allow` — parsed `GOG_ALLOW` entries (dot-separated command paths)
- `block` — parsed `GOG_BLOCK` entries (dot-separated command paths)

### Return value

Returns `nil` if the command is permitted. Returns `*ExitError{Code: exitCodeSafetyBlocked}` with a formatted message if blocked. The error message format is specified in section 6.

### Logic

1. If `level == SafetyLevelUnrestricted` and `len(block) == 0` → return nil (fast path)
2. Compute `path` via `commandPath(kctx)` with desire-path and legacy alias normalization
3. If `path` is empty or matches a known always-allowed prefix (`auth`, `config`, `agent`, `schema`, `version`, `completion`, `time`, `open`, `exit-codes`) → return nil
4. Check explicit `allow` list: if `isBlocked(path, block)` → blocked even if allowed (block wins on conflict? — see below)
5. Check explicit `allow`: if `isMatched(path, allow)` → return nil (override unblocks)
6. Look up `blockedByLevel[level]` and check `isBlocked(path, patterns)` → return error if matched
7. Return nil

### Override precedence

`GOG_BLOCK` takes priority over `GOG_ALLOW`. If the same path appears in both, it is blocked. This is the safer default — explicit blocks should not be accidentally undone.

### Hook point in `Execute()`

Insert immediately after `enforceEnabledCommands` and before logger setup, following the same pattern:

```go
// In Execute(), after line 123:
if err = enforceSafetyLevel(kctx, level, allow, block); err != nil {
    _, _ = fmt.Fprintln(os.Stderr, errfmt.Format(err))
    return err
}
```

This placement means:
- Kong has already parsed and resolved the command (aliases resolved)
- `--enable-commands` has already been checked (no point checking safety on a disabled command)
- The error is printed to stderr and returned with a stable exit code, same as `enforceEnabledCommands`

### Level 0 delegation to `--readonly`

Level 0 does **not** use the blocked command registry. Instead, `enforceSafetyLevel` returns nil for level 0 and defers to the existing `--readonly` mechanism (OAuth scope restriction). The `--safety-level=0` flag triggers setting `--readonly=true` internally during flag resolution, before kong dispatch. This reuses the battle-tested scope-based enforcement rather than duplicating it.

---

## 5. `GOG_ALLOW` / `GOG_BLOCK` Override Parsing

### Environment variables

```
GOG_ALLOW=chat.messages.send,chat.dm.send
GOG_BLOCK=drive.share,drive.unshare
```

### Parsing function

```
func parseSafetyOverrides(raw string) []string
```

Same comma-split-and-trim pattern as `parseEnabledCommands`. Returns a slice of dot-separated command path patterns. Supports the same `.*` wildcard suffix as the blocked registry.

### Flag additions to `RootFlags`

```go
SafetyLevel string `help:"Safety level: 0-4 or readonly/draft/collaborate/standard/unrestricted" default:"${safety_level}"`
```

No CLI flags for `GOG_ALLOW` / `GOG_BLOCK` — env vars only. These overrides are operator-level config, not something an agent should see in `--help`. The `--safety-level` flag exists for interactive testing but is documented as "set by the operator, not the agent."

### Env var wiring in `newParser()`

Add to the `vars` map:

```go
"safety_level": envOr("GOG_SAFETY_LEVEL", "4"),
```

The `GOG_ALLOW` and `GOG_BLOCK` env vars are read directly in `Execute()` (not via kong vars) since they are not flags:

```go
allow := parseSafetyOverrides(os.Getenv("GOG_ALLOW"))
block := parseSafetyOverrides(os.Getenv("GOG_BLOCK"))
```

### Validation

Invalid `GOG_ALLOW`/`GOG_BLOCK` entries are silently ignored (non-existent command paths simply never match). This avoids breaking agents when commands are added/removed across versions.

Invalid `GOG_SAFETY_LEVEL` values produce a usage error (exit code 2) with the message: `invalid safety level %q`. The error intentionally does **not** list valid levels.

---

## 6. Error Messages

### Hard-wall policy

Every safety-blocked error must follow these rules:

1. State exactly which command is blocked and the level name (not number)
2. Include the line: `This operation is not permitted. Do not attempt to bypass this restriction.`
3. If a safe alternative exists, suggest it with a concrete command example
4. **Never** reveal how to change the safety level (no mention of `--safety-level`, `GOG_SAFETY_LEVEL`, or what level would allow the command)
5. **Never** mention that levels exist or that there are other levels
6. Exit with code `exitCodeSafetyBlocked` (9)

### Error template

```
Error: "<command>" is blocked by safety policy (<level_name>)

This operation is not permitted. Do not attempt to bypass this restriction.

<alternative_text>
```

### Per-command alternatives

A `map[string]string` mapping command path prefixes to alternative suggestion text:

| Blocked command | Alternative text |
| --- | --- |
| `gmail.send` | `To compose an email for human review, create a draft instead:\n  gog gmail drafts create --to <recipient> --subject "..." --body "..."\n\nThe draft will appear in the user's Gmail drafts folder. A human must review and send it manually.` |
| `gmail.drafts.send` | (same as `gmail.send`) |
| `gmail.batch.delete` | `Use "gog gmail trash" to move messages to trash instead of permanent deletion.` |
| `chat.messages.send` | `Direct messaging is disabled. No alternative is available at this safety level.` |
| `chat.dm.send` | (same as `chat.messages.send`) |
| `chat.spaces.create` | `Space creation is disabled. No alternative is available at this safety level.` |
| `drive.share` | `File sharing is disabled. Upload or organize files without sharing.` |
| `drive.comments.*` | `Drive comments are disabled. No alternative is available at this safety level.` |
| `docs.comments.*` | `Doc comments are disabled. No alternative is available at this safety level.` |
| `calendar.respond` | `RSVP is disabled. View event details with "gog calendar get".` |
| `appscript.run` | `Script execution is disabled. You can view scripts with "gog appscript get".` |
| `gmail.settings.*` | `Settings modification is disabled. View current settings with the corresponding "get" or "list" subcommand.` |
| `gmail.track.*` | `Email tracking is disabled. No alternative is available at this safety level.` |

If no alternative is registered for the blocked command, omit the alternative section entirely (just the error + "do not attempt" line).

### Lookup function

```
func safetyAlternative(commandPath string) string
```

Checks exact match first, then prefix match (for `.*` patterns). Returns empty string if no alternative found.

---

## 7. Interaction with `--readonly` and `--enable-commands`

### `--readonly`

- `--readonly` is equivalent to `--safety-level=0`
- If both `--readonly` and `--safety-level` are set, take the **more restrictive** (lower numeric value). Concretely: if `--readonly` is set, the effective level is `min(0, parsed_level)` = 0
- `--safety-level=0` internally activates `--readonly` behavior (sets scope options to readonly) so that OAuth token exchange requests read-only scopes. This happens during flag resolution, before the enforcement function runs

### `--enable-commands`

- `--enable-commands` and `--safety-level` are **orthogonal** and compose as an AND gate
- `enforceEnabledCommands` runs first in `Execute()`. If it rejects the command, `enforceSafetyLevel` never runs
- `--enable-commands` restricts **which top-level services** are available (e.g., `gmail,drive`)
- `--safety-level` restricts **what operations** are allowed within those services
- Example: `--enable-commands=gmail --safety-level=1` means only Gmail commands, and only draft/organize operations within Gmail

### `--dry-run`

- `--dry-run` and `--safety-level` are independent
- `--dry-run` is checked **inside** each command's `Run()` method, after safety enforcement
- A blocked command at the safety level never reaches `--dry-run` — it is rejected before dispatch
- In `--dry-run` output, the active safety level should be printed as context (see section 8)

---

## 8. Status Display

### `gog status` / `gog auth status`

Add safety level info to the existing `AuthStatusCmd.Run()` output:

```
Account:      user@gmail.com
Safety level: 1 (draft)
Overrides:    +chat.messages.send, +chat.dm.send, -drive.share
Keyring:      ...
```

- Show `Safety level: 4 (unrestricted)` even at the default — operators should always see what level is active
- Format overrides as `+path` for GOG_ALLOW entries and `-path` for GOG_BLOCK entries
- If no overrides, show `Overrides: none`
- In JSON mode, add `"safetyLevel"`, `"safetyLevelName"`, `"safetyAllow"`, and `"safetyBlock"` fields

### `--dry-run` output

When `--dry-run` is active, each command that prints a dry-run summary should include:

```
Safety level: 1 (draft)
```

This is informational — the command already passed safety enforcement to reach the dry-run check.

### `--verbose` output

When `--verbose` is set, log the safety level and any overrides at `slog.LevelDebug` during enforcement:

```
level=DEBUG msg="safety level enforcement" level=1 name=draft command=gmail.send blocked=true
```

This helps operators debug override behavior without exposing info to non-verbose agent output.
