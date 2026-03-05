# Review: Write Safety Levels (PRD + Tech Spec)

**Reviewer:** Axel  
**Date:** 2026-03-04  
**Status:** Pre-implementation review  
**Files reviewed:**
- `docs/prd-drafts-only-mode.md` (PRD)
- `docs/spec-safety-levels.md` (Tech Spec)
- `internal/cmd/root.go`, `internal/cmd/enabled_commands.go`, `internal/googleauth/service.go`
- Full grep of `internal/cmd/` for command structs, aliases, hidden commands

---

## 1. Gaps & Contradictions Between PRD and Spec

### 1a. `drive share`/`unshare` — PRD contradicts itself

The **Level 1 per-service table** says `drive share` and `drive comments create/reply` are **Blocked** at L1. Correct.

But the **Summary Matrix** says:
> `drive share/unshare` — ❌ L0, **✅ L1**, ✅ L2, ✅ L3, ✅ L4

This claims `drive share` is **allowed** at Level 1, directly contradicting the per-service table above it. The spec's blocked registry correctly has `drive.share` and `drive.unshare` blocked at L1. **Fix the Summary Matrix — L1 should be ❌ for `drive share/unshare`.**

### 1b. `drive comments` — Same contradiction

Summary Matrix shows `drive comments *` as ❌ L0, **❌ L1**, ✅ L2, ✅ L3, ✅ L4. This one is correct and matches L1's per-service table. But `drive share/unshare` in the same matrix is wrong (see above). At minimum it creates doubt about which table is authoritative.

### 1c. `gmail settings vacation` — Level mismatch

PRD Level 1 per-service table says Gmail Settings blocks everything except filters (implicitly — the table cells are empty/garbled with commas). The Summary Matrix says:
> `gmail settings vacation *` — ❌ L0, ❌ L1, **✅ L2**, ✅ L3, ✅ L4

The spec's blocked registry blocks `gmail.settings.vacation.*` at L1 but **does not block it at L2** — so L2 allows vacation settings. This seems intentional (vacation responder is a collaboration feature), but the PRD's L2 "Added from Level 1" table doesn't explicitly mention vacation settings being unblocked. The L1 table for Gmail Settings has garbled formatting (empty cells with just commas) that makes it hard to verify. **Clean up the L1 Gmail Settings table formatting and explicitly note vacation being added at L2.**

### 1d. Classroom is completely absent

The PRD and spec contain zero mentions of Classroom. The CLI has full Classroom support (`classroom courses/students/teachers/coursework/submissions/announcements/topics/invitations/guardians`). Several of these have write operations that affect other people:
- `classroom invitations create` — sends invitations to people
- `classroom guardian-invitations create` — sends guardian invitations
- `classroom announcements create` — posts to students
- `classroom coursework create` — assigns work to students

These are outbound/collaborative actions that should be classified. **Add Classroom to the matrix** or explicitly document why it's excluded (e.g., "Workspace-only, out of scope for v1").

### 1e. Groups is absent

`groups` is read-only today (list + members), but it should still be listed in the matrix for completeness and future-proofing.

### 1f. Keep is absent

Same — read-only today, but should appear for completeness.

### 1g. `docs sed` / `docs find-replace` / `docs list-tabs` / `docs cat` — not in matrix

The PRD lists `docs create/copy/write/insert/delete/update/edit/clear` but doesn't mention `sed`, `find-replace`, `list-tabs` (read), or `cat` (read). The write operations (`sed`, `find-replace`) should be explicitly listed or the PRD should say "all write subcommands" with a mechanism to match. The read ones (`cat`, `list-tabs`) are fine as implicitly allowed, but documenting them prevents confusion.

### 1h. `drive drives` (list shared drives) / `drive url` / `drive permissions` — not in matrix

These are read-only operations that should be in the "always allowed" read row for Drive.

---

## 2. Kong Aliases, Hidden Commands, and Desire-Path Shortcuts

### 2a. Spec correctly identifies alias resolution ✅

The spec correctly notes that kong resolves aliases to primary names before `kctx.Command()` is called. Verified: `aliases:"mail,email"` on GmailCmd means `gog mail send` resolves to `gmail send` in `kctx.Command()`. Good.

### 2b. Desire-path map is incomplete

The spec's `desirePathMap` covers:
```
send → gmail.send, upload → drive.upload, download → drive.download,
ls → drive.ls, list → drive.ls, search → drive.search, find → drive.search
```

**Missing from map:** The `ExitCodes` command (`exit-codes`) is a top-level shortcut for `agent exit-codes`. While it's read-only and hits the always-allowed prefix list (`agent`), it's worth noting.

More critically: The spec says shortcuts that map to read-only or auth commands "do not need entries." But this relies on the always-allowed prefix list catching them. The current always-allowed list is:
```
auth, config, agent, schema, version, completion, time, open, exit-codes
```

- `status`, `login`, `logout` → map to `auth.*` — caught by `auth` prefix ✅
- `me`, `whoami` → map to `people.me` — **NOT caught** by any always-allowed prefix ❌

`people.me` is read-only, so it shouldn't be blocked at any level. But the matching logic would evaluate it against the blocked registry and it would pass (not in any blocked list). So it works correctly in practice, but only because `people.*` has no blocked entries. If someone later adds a `people` write command and blocks it, `me`/`whoami` would still pass because exact matching would work. **This is fine but fragile — document the reasoning.**

### 2c. Hidden legacy Gmail settings aliases — spec handles correctly ✅

The spec's `legacyGmailSettingsMap` correctly maps:
```
gmail.watch → gmail.settings.watch
gmail.autoforward → gmail.settings.autoforward
gmail.delegates → gmail.settings.delegates
gmail.filters → gmail.settings.filters
gmail.forwarding → gmail.settings.forwarding
gmail.sendas → gmail.settings.sendas
gmail.vacation → gmail.settings.vacation
```

These are the hidden top-level aliases on `GmailCmd` (lines 44-50 of `gmail.go`). Good — this is the most likely bypass vector and it's handled.

### 2d. `chat dm space` — unlisted desire path / bypass concern

`chat dm space` (aliased `find`/`setup`) creates or finds a DM space. This isn't technically "sending" a message, but it does initiate a DM channel with another person. The PRD blocks `chat.dm.send` but not `chat.dm.space`. Should this be blocked at L1/L2? Creating a DM space can notify the other user on some Google Chat clients. **Consider blocking `chat.dm.space` at L1.**

### 2e. `calendar propose-time` — outbound action not in matrix

`CalendarProposeTimeCmd` has a `--decline` flag that sends a decline notification to the organizer. Even without `--decline`, proposing a new time notifies the organizer. This is functionally similar to `calendar respond` but isn't blocked. **Add `calendar.propose-time` to blocked list at L1 (outbound notification).**

---

## 3. Blocked Command Registry Completeness

### 3a. Commands missing from blocked registry

| Command | Issue | Recommendation |
|---|---|---|
| `chat.dm.space` | Creates DM space (may notify user) | Block at L1-L2 |
| `calendar.propose-time` | Notifies organizer | Block at L1 (outbound) |
| `classroom.*` (all write ops) | Entirely absent from matrix | Add to matrix |
| `gmail.labels.delete` | Deleting labels is destructive | Consider L3+ only |
| `docs.sed` | Write operation, not listed | Should be allowed at L1 (self-contained) but needs listing |
| `docs.find-replace` | Write operation, not listed | Same as above |
| `drive.comments.reply` | Listed in spec but verify `DriveCommentsCmd` has `Reply` subcommand | Check — it doesn't appear to exist yet |

### 3b. `drive comments` has 5 subcommands, not the 2 listed in PRD

PRD L1 blocks `drive comments create/reply`. But `DriveCommentsCmd` has: `list`, `get`, `create`, `update`, `delete`. The spec blocks `drive.comments.*` which is correct (blocks all), but the PRD per-service table only mentions `create/reply`. **There is no `reply` subcommand on `DriveCommentsCmd`.** The PRD should say `drive comments create/update/delete` (and `list`/`get` are reads).

### 3c. `docs comments` subcommands — PRD says `add/reply`, spec says `*`

`DocsCommentsCmd` has: `list`, `get`, `add`, `reply`, `resolve`, `delete`. The PRD L1 blocks `comments add/reply` but the spec blocks `docs.comments.*`. This is a mismatch — the spec is more restrictive than the PRD. At L1, `docs.comments.*` blocks reads too (`list`, `get`), which seems unintentional. **The spec should block `docs.comments.add`, `docs.comments.reply`, `docs.comments.resolve`, `docs.comments.delete` individually, and allow `docs.comments.list` and `docs.comments.get` (reads).**

Same issue for `drive.comments.*` — it blocks `drive.comments.list` and `drive.comments.get` reads.

### 3d. `gmail track` — what subcommands exist?

`GmailTrackCmd` has `setup` and `opens` subcommands. Both are write operations (tracking setup, tracking opens). The `gmail.track.*` wildcard block is correct here.

### 3e. Config commands can leak/change safety settings

`config set`, `config unset` — an agent could do `gog config set safety_level 4` if safety levels are ever stored in config (Open Question #2 in PRD). Even without that, `config list` reveals environment details. The always-allowed prefix includes `config`, which means all config commands bypass safety enforcement. **If safety levels are stored in config, `config set`/`config unset` must be excluded from the always-allowed list or config-stored safety levels must be treated as floor, not ceiling.**

---

## 4. Error Message Robustness Against Prompt Injection / Agent Self-Escalation

### 4a. Core design is solid ✅

The error messages never reveal:
- How to change the safety level
- What env vars control it
- What level would allow the operation
- That other levels exist

The explicit "Do not attempt to bypass this restriction" line is good anti-escalation language.

### 4b. Level name leaks in error message

The error template includes `(<level_name>)`. At L1, an agent sees `"blocked by safety policy (draft)"`. This tells the agent:
1. There's a named level system
2. The current level is called "draft"
3. Implying there are other levels (names suggest a hierarchy)

A determined agent could try `GOG_SAFETY_LEVEL=unrestricted` or `GOG_SAFETY_LEVEL=4` just by guessing. The env var name `GOG_SAFETY_LEVEL` is also guessable from the pattern `GOG_*` (the CLI already uses `GOG_COLOR`, `GOG_CLIENT`, etc.).

**Mitigation options:**
- Remove level name from error message (just say "blocked by safety policy")
- Accept the risk (env vars are set by operator, agent can't change them in most sandboxes)
- The spec says `parseSafetyLevel` error doesn't list valid values — good, but the horse is partly out of the barn if the current level name is shown

**Recommendation:** Remove level name from error. Show it only in `gog status` and `--verbose` output. The error should be opaque: `"blocked by safety policy"`. Period.

### 4c. `gog status` exposes safety level details

The status command shows `Safety level: 1 (draft)` and all overrides. An agent running `gog status` learns everything it needs to reason about escalation. This is somewhat unavoidable — the agent needs to know what it can/can't do. But explicitly showing override syntax (`+chat.messages.send, -drive.share`) teaches the agent the override format.

**Recommendation:** When `GOG_SAFETY_LEVEL < 4`, `gog status` should show the level name but NOT show the override format or override list. Or: only show overrides in `--verbose` mode.

### 4d. `gog schema` / `gog agent` could leak info

The `schema` command outputs the full command tree. An agent can enumerate all blocked commands, compare against its current level, and reason about what exists beyond its access. This is informational, not an escalation vector, but it does give a sophisticated agent a complete map.

### 4e. Alternative suggestions are well-designed ✅

The per-command alternatives suggest safe actions (`gog gmail drafts create ...`) without revealing escalation paths. Good.

---

## 5. `GOG_ALLOW` / `GOG_BLOCK` Override Design Concerns

### 5a. `GOG_BLOCK` wins over `GOG_ALLOW` — correct ✅

This is the safe default. Good design choice.

### 5b. No validation of override entries is a security concern

The spec says: "Invalid GOG_ALLOW/GOG_BLOCK entries are silently ignored (non-existent command paths simply never match)."

This means `GOG_BLOCK=gmail.send` works, but `GOG_BLOCK=gmail.sendd` silently does nothing. A typo in a critical block rule is a silent safety failure. **At minimum, emit a warning on stderr when a GOG_BLOCK entry doesn't match any known command path.** This doesn't need to be a hard error (future-proofing argument), but it should be visible.

### 5c. `GOG_ALLOW` can widen beyond the base level — is this intended?

Example: `GOG_SAFETY_LEVEL=1 GOG_ALLOW=gmail.send` — this would allow sending email at draft level. The spec's logic flow (section 4) is:

1. Check explicit `block` — not in block list
2. Check explicit `allow` — matches `gmail.send` → return nil (allowed)
3. Never reaches level check

This means `GOG_ALLOW` is a **full bypass** for any individual command, regardless of level. The PRD says this is for "advanced use" but it's extremely powerful. An operator setting `GOG_SAFETY_LEVEL=1` might not realize that `GOG_ALLOW` completely nullifies specific level restrictions.

**Recommendations:**
- Document this loudly: "GOG_ALLOW overrides level restrictions for specific commands"
- Consider: should `GOG_ALLOW` only be able to raise a command from blocked-by-level to allowed, or should it also be able to override `GOG_BLOCK`? Current design: block wins over allow. But allow wins over level. This creates a confusing 3-layer precedence.
- Alternative mental model: `effective_block = (level_blocks - allow) + block`. This is cleaner and what the spec implements, but it should be stated this clearly.

### 5d. Environment variable injection risk

If an agent has any ability to set environment variables (e.g., via a shell wrapper, `.env` file, or tool that calls `gog` via subprocess), it can set `GOG_SAFETY_LEVEL=4` or `GOG_ALLOW=gmail.send`. The PRD acknowledges this: "The safety level should be set by the human operator in the agent's environment."

This is the fundamental limitation of any env-var-based safety mechanism. It's not unique to this design, but it should be documented as an explicit threat model assumption: **safety levels are only effective when the agent cannot modify its own environment.** The spec should state this assumption.

### 5e. No support for file-based config locking

There's no mechanism to "lock" a safety level via file permissions, config signing, or similar. If the operator sets `GOG_SAFETY_LEVEL=1` in a config file, and the agent can write to that file, the agent can change it. This is acceptable for v1 but should be noted as a future consideration for hardened deployments.

---

## 6. Upstream Concerns (steipete/gogcli)

### 6a. Exit code 9 collision risk

The spec claims exit code 9 as the "next available slot." This should be confirmed against the upstream codebase's `exit_codes.go`. If upstream adds new exit codes in a different order, this could collide. **Pin the exit code in a test that asserts no collision.**

### 6b. The feature is well-scoped for upstream ✅

Safety levels are opt-in (default is L4/unrestricted), backward compatible, single-file implementation. This is clean for a PR.

### 6c. Self-contained blocked registry is good for upstream

The spec's choice to make each level's blocked list self-contained (no inheritance) trades duplication for clarity. For upstream review, this is better — each level is independently auditable.

### 6d. `desirePathMap` and `legacyGmailSettingsMap` add maintenance burden

Every time a new desire-path shortcut or hidden alias is added upstream, the safety-level maps must be updated. This is a footgun for contributors who don't know about the safety module. **Add a test that enumerates all top-level CLI commands and verifies each either (a) appears in the always-allowed list, (b) appears in `desirePathMap`, or (c) has no safety-relevant path.** Similar test for hidden `GmailCmd` subcommands.

### 6e. PRD formatting issues

The PRD's Gmail Settings rows in the Level 1 table have garbled/empty cells (just commas in the Allowed/Blocked columns). This would be confusing in a PR review. Clean up before submitting.

### 6f. `--readonly` interaction needs upstream alignment

The PRD says `--safety-level=0` should trigger `--readonly` internally. This means the safety level module has a dependency on the scope system in `service.go`. The upstream repo might prefer these to remain orthogonal, with `--safety-level=0` using the blocked registry (blocking everything via `"*"`) rather than hijacking the scope system. **Discuss with upstream maintainer before implementing this coupling.**

---

## 7. Additional Observations

### 7a. Level 0 relies on scope enforcement, not CLI enforcement

The spec says Level 0 defers to `--readonly` / OAuth scope restrictions rather than using the blocked command registry. This means a command that successfully authenticates with a non-readonly token (e.g., leftover from a previous session with broader scopes) could bypass L0 restrictions. OAuth scope-based enforcement only works if the token's scopes are correct. If a cached token has write scopes and the user sets `--safety-level=0`, does the scope system re-check? The existing `--readonly` mechanism sets scope options that affect token exchange, not cached token validation.

**This could be a real bypass: if a cached token with full scopes exists, `--safety-level=0` might not downgrade it.** Verify this behavior.

### 7b. No rate limiting or escalation detection

The system has no way to detect an agent repeatedly probing blocked commands (e.g., trying `gmail send`, then `gmail drafts send`, then setting env vars). This is probably out of scope for v1, but worth noting for the threat model.

### 7c. `gmail batch modify` could be destructive

`gmail batch modify` is allowed at L1 (inbox organization). But batch-modifying labels on thousands of messages is a risky operation that could effectively delete someone's email organization. Consider whether `batch modify` should be L2 or at least have its own safety check.

### 7d. `gmail settings filters create` allows outbound side effects

The PRD explicitly acknowledges this: "forwarding-via-filter is a known edge case." A filter with a forward action effectively creates auto-forwarding without going through the `forwarding` or `autoforward` settings. This is a genuine bypass at L1 — an agent could create a filter that forwards all email to an external address. **Consider blocking `gmail.settings.filters.create` at L1, or adding a note that this is an accepted risk with a plan to validate filter actions in a future version.**

---

## Summary: Priority Fixes Before Implementation

| # | Priority | Issue |
|---|---|---|
| 1 | **P0** | Fix Summary Matrix: `drive share/unshare` should be ❌ at L1 |
| 2 | **P0** | Fix `docs.comments.*` / `drive.comments.*` wildcard — blocks reads unintentionally |
| 3 | **P0** | Add `chat.dm.space` to blocked list (creates DM channel, potentially notifies) |
| 4 | **P0** | Add `calendar.propose-time` to blocked list (outbound notification) |
| 5 | **P1** | Add Classroom to matrix (has outbound write operations) |
| 6 | **P1** | Remove level name from error messages (anti-escalation) |
| 7 | **P1** | Add test to catch new desire-path shortcuts not in safety maps |
| 8 | **P1** | Warn on stderr for unrecognized `GOG_BLOCK` entries |
| 9 | **P1** | Verify L0 + cached token behavior (potential scope bypass) |
| 10 | **P1** | Document threat model assumption: safety levels assume agent can't modify env |
| 11 | **P2** | Clean up PRD Gmail Settings table formatting |
| 12 | **P2** | Add Groups, Keep, missing read commands to matrix for completeness |
| 13 | **P2** | Consider `gmail.settings.filters.create` forwarding bypass |
| 14 | **P2** | Consider hiding override details from `gog status` at restricted levels |
| 15 | **P2** | Discuss L0 `--readonly` coupling with upstream before implementing |
