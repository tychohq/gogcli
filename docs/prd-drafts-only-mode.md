# PRD: Write Safety Levels

## Problem

When gog is used by AI agents or automation, operators need granular control over what write operations are allowed. The existing `--readonly` flag is all-or-nothing: either full read-only (no writes at all) or full read-write (can send emails, delete files, modify settings). There's no middle ground.

Operators need tiered safety levels: "let the agent organize my inbox and draft emails, but never actually send one."

## Design: Safety Levels 0–4

Five levels from most restrictive to fully open. Each level is a **preset** that sets per-service write permissions. Operators can also override individual services.

### Level 0: Read-Only (existing `--readonly`)

No writes to any service. Already implemented via `--readonly` / `--gmail-scope=readonly` / `--drive-scope=readonly`.

### Level 1: Draft & Organize

Write to your own workspace. Create drafts. Organize existing items. **Nothing outbound — no messages reach other people.**

| Service                  | Allowed                                                                                                              | Blocked                                           | Rationale                                                                                                                  |
| ------------------------ | -------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------- |
| **Gmail**          | `drafts create/update/delete`, `labels *`, `archive`, `mark-read`, `unread`, `trash`, `batch modify`   | `send`, `drafts send`, `batch delete`       | Drafts + inbox management only                                                                                             |
| **Calendar**       | `create`, `update`, `delete`, `focus-time`, `ooo`, `working-location`                                    | `respond`                                       | Creating/editing your own events is fine; RSVP notifies the organizer                                                      |
| **Chat**           | —                                                                                                                   | `messages send`, `dm send`, `spaces create` | All chat writes are outbound                                                                                               |
| **Drive**          | `upload`, `mkdir`, `copy`, `move`, `rename`, `delete`, `share`, `unshare`                       | `comments create/update/delete`                                                                                                                                             | Manage your own files; sharing is allowed (grants access, no message sent). Comments notify collaborators. |
| **Docs**           | `create`, `copy`, `write`, `insert`, `delete`, `update`, `edit`, `clear`                             | `comments add/reply`                            | Edit your own docs; comments notify collaborators                                                                          |
| **Slides**         | `create`, `copy`, `create-from-markdown`, `add-slide`, `delete-slide`, `update-notes`, `replace-slide` | —                                                | All self-contained                                                                                                         |
| **Sheets**         | `create`, `copy`, `update`, `append`, `insert`, `clear`, `format`                                      | —                                                | All self-contained                                                                                                         |
| **Contacts**       | `create`, `update`, `delete`                                                                                   | —                                                | Address book is private                                                                                                    |
| **Tasks**          | `add`, `update`, `done`, `undo`, `delete`, `clear`, `lists create`                                     | —                                                | Personal todo list                                                                                                         |
| **Forms**          | `create`                                                                                                           | —                                                | Self-contained                                                                                                             |
| **AppScript**      | `create`                                                                                                           | `run`                                           | Creating is safe; running executes arbitrary code                                                                          |
| **Gmail Settings** | `filters create/delete` | `delegates add/remove`, `forwarding create/delete`, `autoforward update`, `sendas create/delete/update/verify`, `vacation update`, `watch *` | Filters are inbox organization; forwarding-via-filter is a known edge case. Other settings have outbound side effects. |

### Level 2: Draft & Collaborate

Everything in Level 1, **plus** collaborative actions within shared workspaces. Still no direct messaging.

| Service                  | Added from Level 1                                              | Still blocked |
| ------------------------ | --------------------------------------------------------------- | ------------- |
| **Calendar**       | `respond` (RSVP)                                              | —            |
| **Drive**          | `share`, `unshare`, `comments create/update/delete/reply` | —            |
| **Docs**           | `comments add/reply/resolve/delete`                           | —            |
| **Gmail Settings** | `vacation update` | `delegates add/remove`, `forwarding create/delete`, `autoforward update`, `sendas *`, `watch *` |
| **AppScript**      | `run`                                                         | —            |

### Level 3: Full Write (No Admin)

Everything in Level 2, **plus** direct messaging. The only things blocked are destructive admin/settings operations.

| Service                  | Added from Level 2                                | Still blocked                                                                               |
| ------------------------ | ------------------------------------------------- | ------------------------------------------------------------------------------------------- |
| **Gmail**          | `send`, `drafts send`                         | `batch delete` (permanent delete)                                                         |
| **Chat**           | `messages send`, `dm send`, `spaces create` | —                                                                                          |
| **Gmail Settings** | `sendas *`                                      | `delegates add/remove`, `forwarding create/delete`, `autoforward update`, `watch *` |

### Level 4: Unrestricted

All operations allowed. No CLI-level blocking. This is the current default behavior.

---

## Summary Matrix

| Command                                                                                       | L0 | L1 | L2 | L3 | L4 |
| --------------------------------------------------------------------------------------------- | :-: | :-: | :-: | :-: | :-: |
| **Gmail**                                                                               |    |    |    |    |    |
| `gmail search/get/messages/thread/attachment/url/history`                                   | ✅ | ✅ | ✅ | ✅ | ✅ |
| `gmail labels *`                                                                            | ❌ | ✅ | ✅ | ✅ | ✅ |
| `gmail archive/mark-read/unread/trash`                                                      | ❌ | ✅ | ✅ | ✅ | ✅ |
| `gmail batch modify`                                                                        | ❌ | ✅ | ✅ | ✅ | ✅ |
| `gmail drafts create/update/delete/list/get`                                                | ❌ | ✅ | ✅ | ✅ | ✅ |
| `gmail send` / `gmail drafts send` / `send`                                             | ❌ | ❌ | ❌ | ✅ | ✅ |
| `gmail batch delete`                                                                        | ❌ | ❌ | ❌ | ❌ | ✅ |
| `gmail track *`                                                                             | ❌ | ❌ | ❌ | ✅ | ✅ |
| `gmail settings filters *`                                                                  | ❌ | ✅ | ✅ | ✅ | ✅ |
| `gmail settings vacation *`                                                                 | ❌ | ❌ | ✅ | ✅ | ✅ |
| `gmail settings sendas *`                                                                   | ❌ | ❌ | ❌ | ✅ | ✅ |
| `gmail settings delegates *`                                                                | ❌ | ❌ | ❌ | ❌ | ✅ |
| `gmail settings forwarding *`                                                               | ❌ | ❌ | ❌ | ❌ | ✅ |
| `gmail settings autoforward *`                                                              | ❌ | ❌ | ❌ | ❌ | ✅ |
| `gmail settings watch *`                                                                    | ❌ | ❌ | ❌ | ❌ | ✅ |
| **Calendar**                                                                            |    |    |    |    |    |
| `calendar events/list/get/free-busy/calendars`                                              | ✅ | ✅ | ✅ | ✅ | ✅ |
| `calendar create/update/delete/focus-time/ooo/working-location`                             | ❌ | ✅ | ✅ | ✅ | ✅ |
| `calendar respond`                                                                          | ❌ | ❌ | ✅ | ✅ | ✅ |
| `calendar propose-time`                                                                     | ❌ | ❌ | ✅ | ✅ | ✅ |
| **Chat**                                                                                |    |    |    |    |    |
| `chat spaces list/get`, `chat messages list/get`, `chat dm list`                        | ✅ | ✅ | ✅ | ✅ | ✅ |
| `chat messages send`, `chat dm send`                                                      | ❌ | ❌ | ❌ | ✅ | ✅ |
| `chat dm space`                                                                             | ❌ | ❌ | ❌ | ✅ | ✅ |
| `chat spaces create`                                                                        | ❌ | ❌ | ❌ | ✅ | ✅ |
| **Drive**                                                                               |    |    |    |    |    |
| `drive ls/search/get/download/info/permissions`                                             | ✅ | ✅ | ✅ | ✅ | ✅ |
| `drive upload/mkdir/copy/move/rename/delete`                                                | ❌ | ✅ | ✅ | ✅ | ✅ |
| `drive share/unshare`                                                                       | ❌ | ✅ | ✅ | ✅ | ✅ |
| `drive comments *`                                                                          | ❌ | ❌ | ✅ | ✅ | ✅ |
| **Docs**                                                                                |    |    |    |    |    |
| `docs get/export/list`                                                                      | ✅ | ✅ | ✅ | ✅ | ✅ |
| `docs create/copy/write/insert/delete/update/edit/clear`                                    | ❌ | ✅ | ✅ | ✅ | ✅ |
| `docs comments *`                                                                           | ❌ | ❌ | ✅ | ✅ | ✅ |
| **Slides**                                                                              |    |    |    |    |    |
| `slides get/export/list`                                                                    | ✅ | ✅ | ✅ | ✅ | ✅ |
| `slides create/copy/add-slide/delete-slide/update-notes/replace-slide/create-from-markdown` | ❌ | ✅ | ✅ | ✅ | ✅ |
| **Sheets**                                                                              |    |    |    |    |    |
| `sheets get/export/list/read`                                                               | ✅ | ✅ | ✅ | ✅ | ✅ |
| `sheets create/copy/update/append/insert/clear/format`                                      | ❌ | ✅ | ✅ | ✅ | ✅ |
| **Contacts**                                                                            |    |    |    |    |    |
| `contacts list/get/search/directory/other`                                                  | ✅ | ✅ | ✅ | ✅ | ✅ |
| `contacts create/update/delete`                                                             | ❌ | ✅ | ✅ | ✅ | ✅ |
| **Tasks**                                                                               |    |    |    |    |    |
| `tasks list/get`                                                                            | ✅ | ✅ | ✅ | ✅ | ✅ |
| `tasks add/update/done/undo/delete/clear/lists create`                                      | ❌ | ✅ | ✅ | ✅ | ✅ |
| **Forms**                                                                               |    |    |    |    |    |
| `forms get/responses`                                                                       | ✅ | ✅ | ✅ | ✅ | ✅ |
| `forms create`                                                                              | ❌ | ✅ | ✅ | ✅ | ✅ |
| **AppScript**                                                                           |    |    |    |    |    |
| `appscript list/get`                                                                        | ✅ | ✅ | ✅ | ✅ | ✅ |
| `appscript create`                                                                          | ❌ | ✅ | ✅ | ✅ | ✅ |
| `appscript run`                                                                             | ❌ | ❌ | ✅ | ✅ | ✅ |

| **Classroom**                                                                           |    |    |    |    |    |
| `classroom courses/students/teachers/coursework list/get`                                   | ✅ | ✅ | ✅ | ✅ | ✅ |
| `classroom coursework create`, `classroom announcements create`                             | ❌ | ❌ | ✅ | ✅ | ✅ |
| `classroom invitations create`, `classroom guardian-invitations create`                      | ❌ | ❌ | ❌ | ✅ | ✅ |
| **Groups** (read-only today)                                                            |    |    |    |    |    |
| `groups list/members`                                                                       | ✅ | ✅ | ✅ | ✅ | ✅ |
| **Keep** (read-only today)                                                              |    |    |    |    |    |
| `keep list/get`                                                                             | ✅ | ✅ | ✅ | ✅ | ✅ |

## UX

### Flag

```bash
# Set via env var (recommended — set by the human operator, not the agent)
export GOG_SAFETY_LEVEL=1

# Or via flag (for testing / interactive use)
gog --safety-level 1 gmail drafts create --to user@example.com --subject "Hello" --body "..."
```

**The safety level should be set by the human operator** in the agent's environment, not by the agent itself. The flag exists for interactive testing, but in production the env var is the intended mechanism — it's set once in the agent's config and the agent never sees it as something it can change.

### Per-service overrides

For advanced use — override individual services regardless of the base level:

```bash
# Level 1 base, but allow sending chat messages
export GOG_SAFETY_LEVEL=1
export GOG_ALLOW="chat.messages.send,chat.dm.send"

# Level 3 base, but block drive share
export GOG_SAFETY_LEVEL=3
export GOG_BLOCK="drive.share"
```

### Status display

```bash
$ gog status
Account:      user@gmail.com
Safety level: 1 (draft-and-organize)
Overrides:    +chat.messages.send, +chat.dm.send
```

### Error messages

Error messages must be a **hard wall**. They should:
1. Tell the agent exactly what's blocked and why
2. Suggest the safe alternative (e.g., create a draft)
3. **Never** tell the agent how to change the safety level
4. Explicitly tell the agent not to try to work around it

```
Error: "gmail send" is blocked by safety policy

This operation is not permitted. Do not attempt to bypass this restriction.

To compose an email for human review, create a draft instead:
  gog gmail drafts create --to user@example.com --subject "..." --body "..."

The draft will appear in the user's Gmail drafts folder. A human must review and send it manually.
```

```
Error: "chat messages send" is blocked by safety policy

This operation is not permitted. Do not attempt to bypass this restriction.
Direct messaging is disabled. No alternative is available at this safety level.
```

The error intentionally omits:
- How to change the safety level
- What flag or env var controls it
- What level would allow the operation

Changing the safety level should require a human editing the environment or config — not something an agent can self-escalate.

## Level Names

| Level | Numeric | Name             | One-liner                                                           |
| ----- | ------- | ---------------- | ------------------------------------------------------------------- |
| 0     | `0`   | `readonly`     | Read everything, write nothing                                      |
| 1     | `1`   | `draft`        | Draft, organize, edit your own stuff — nothing outbound            |
| 2     | `2`   | `collaborate`  | Level 1 + comments, sharing, RSVP — collaborative but no messaging |
| 3     | `3`   | `standard`     | Full write + messaging — no dangerous admin operations             |
| 4     | `4`   | `unrestricted` | Everything allowed                                                  |

Levels can be referenced by number or name: `--safety-level=draft` or `--safety-level=1`.

## Implementation Plan

### Task 1: Define safety level types and blocked command registry

- Create `internal/cmd/safety_levels.go`
- Define level enum and per-level blocked command paths
- Pattern matching for command paths (e.g., `gmail.settings.*` blocks all settings subcommands)

### Task 2: Add `--safety-level` flag and env var

- Add to `RootFlags` in `root.go`
- Add `GOG_SAFETY_LEVEL` env var
- Add `GOG_ALLOW` and `GOG_BLOCK` for per-service overrides
- Wire into `Run()` after `enforceEnabledCommands`

### Task 3: Enforcement function

- `enforceSafetyLevel(kctx, level, allow, block)`
- Match full command path against blocked list for the active level
- Apply allow/block overrides
- Return clear error with alternative suggestion

### Task 4: Status display

- Show safety level and overrides in `gog status` / `gog auth status`
- Show in `--dry-run` output
- Show in `--verbose` output

### Task 5: Tests

- Unit tests for each level with each blocked/allowed command
- Override tests (GOG_ALLOW / GOG_BLOCK)
- Edge cases: command aliases, top-level shortcuts (`send`, `upload`, etc.)

### Task 6: Documentation

- README section on safety levels
- Agent mode setup guide
- Update `gog auth add` help text to mention safety levels

## Open Questions

1. **Default level for new installs?** Currently effectively level 4. Should we change the default? Probably not — backward compatibility.
2. **Should `--safety-level` be storable in config.json?** Per-account safety levels would let you have one account at level 1 (agent) and another at level 4 (personal). Adds complexity.
3. **Interaction with `--readonly`?** `--readonly` should be equivalent to / alias for `--safety-level=0`. If both are set, take the more restrictive. Note: L0 relies on OAuth scope enforcement, not the CLI blocked registry. Verify that cached tokens with broader scopes are handled correctly.
5. **Threat model assumption:** Safety levels assume the agent cannot modify its own environment variables or config files. If the agent has shell access to set `GOG_SAFETY_LEVEL`, the mechanism can be bypassed. This is inherent to any env-var-based safety mechanism and should be documented.
6. **Filter forwarding bypass:** `gmail settings filters create` is allowed at L1 for inbox organization, but a filter with a forward action can effectively auto-forward email. This is an accepted risk for v1, with a plan to validate filter actions in a future version.
7. **Should `--enable-commands` and `--safety-level` compose?** Both must pass for a command to run. They're orthogonal: `--enable-commands` restricts which services, `--safety-level` restricts what you can do within allowed services.
