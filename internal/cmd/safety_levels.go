package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/alecthomas/kong"
)

// SafetyLevel controls which write operations are permitted.
type SafetyLevel int

const (
	SafetyLevelReadonly     SafetyLevel = 0
	SafetyLevelDraft        SafetyLevel = 1
	SafetyLevelCollaborate  SafetyLevel = 2
	SafetyLevelStandard     SafetyLevel = 3
	SafetyLevelUnrestricted SafetyLevel = 4
)

var safetyLevelNames = map[SafetyLevel]string{
	SafetyLevelReadonly:     "readonly",
	SafetyLevelDraft:        "draft",
	SafetyLevelCollaborate:  "collaborate",
	SafetyLevelStandard:     "standard",
	SafetyLevelUnrestricted: "unrestricted",
}

func parseSafetyLevel(s string) (SafetyLevel, error) {
	s = strings.TrimSpace(strings.ToLower(s))

	if n, err := strconv.Atoi(s); err == nil {
		if n >= 0 && n <= 4 {
			return SafetyLevel(n), nil
		}
		return 0, fmt.Errorf("invalid safety level %q", s)
	}

	for level, name := range safetyLevelNames {
		if name == s {
			return level, nil
		}
	}
	return 0, fmt.Errorf("invalid safety level %q", s)
}

// blockedByLevel maps each safety level to a self-contained list of blocked command patterns.
var blockedByLevel = map[SafetyLevel][]string{
	SafetyLevelReadonly: {"*"},
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
		"chat.dm.space",
		"chat.spaces.create",
		"drive.comments.create",
		"drive.comments.update",
		"drive.comments.delete",
		"docs.comments.add",
		"docs.comments.reply",
		"docs.comments.resolve",
		"docs.comments.delete",
		"calendar.respond",
		"calendar.propose-time",
		"appscript.run",
		"classroom.invitations.create",
		"classroom.guardian-invitations.create",
		"classroom.announcements.create",
		"classroom.coursework.create",
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
		"chat.dm.space",
		"chat.spaces.create",
		"classroom.invitations.create",
		"classroom.guardian-invitations.create",
	},
	SafetyLevelStandard: {
		"gmail.batch.delete",
		"gmail.settings.delegates.*",
		"gmail.settings.forwarding.*",
		"gmail.settings.autoforward.*",
		"gmail.settings.watch.*",
	},
}

// alwaysAllowedPrefixes are command path prefixes that are never blocked.
var alwaysAllowedPrefixes = []string{
	"auth", "config", "agent", "schema", "version", "completion", "time", "open", "exit-codes",
}

// desirePathMap maps top-level shortcuts to their canonical nested command paths.
var desirePathMap = map[string]string{
	"send":     "gmail.send",
	"upload":   "drive.upload",
	"download": "drive.download",
	"ls":       "drive.ls",
	"list":     "drive.ls",
	"search":   "drive.search",
	"find":     "drive.search",
}

// legacyGmailSettingsMap maps hidden top-level Gmail aliases to their canonical settings paths.
var legacyGmailSettingsMap = map[string]string{
	"gmail.watch":       "gmail.settings.watch",
	"gmail.autoforward": "gmail.settings.autoforward",
	"gmail.delegates":   "gmail.settings.delegates",
	"gmail.filters":     "gmail.settings.filters",
	"gmail.forwarding":  "gmail.settings.forwarding",
	"gmail.sendas":      "gmail.settings.sendas",
	"gmail.vacation":    "gmail.settings.vacation",
}

// safetyAlternatives maps blocked command paths to alternative suggestion text.
var safetyAlternatives = map[string]string{
	"gmail.send": "To compose an email for human review, create a draft instead:\n" +
		"  gog gmail drafts create --to <recipient> --subject \"...\" --body \"...\"\n\n" +
		"The draft will appear in the user's Gmail drafts folder. A human must review and send it manually.",
	"gmail.drafts.send":  "To compose an email for human review, create a draft instead:\n  gog gmail drafts create --to <recipient> --subject \"...\" --body \"...\"\n\nThe draft will appear in the user's Gmail drafts folder. A human must review and send it manually.",
	"gmail.batch.delete": "Use \"gog gmail trash\" to move messages to trash instead of permanent deletion.",
	"chat.messages.send": "Direct messaging is disabled. No alternative is available at this safety level.",
	"chat.dm.send":       "Direct messaging is disabled. No alternative is available at this safety level.",
	"chat.dm.space":      "DM space creation is disabled. No alternative is available at this safety level.",
	"chat.spaces.create": "Space creation is disabled. No alternative is available at this safety level.",
	"drive.comments.*":   "Drive comments are disabled. No alternative is available at this safety level.",
	"docs.comments.*":    "Doc comments are disabled. No alternative is available at this safety level.",
	"calendar.respond":   "RSVP is disabled. View event details with \"gog calendar get\".",
	"calendar.propose-time":                  "Proposing a new time is disabled. View event details with \"gog calendar get\".",
	"appscript.run":                          "Script execution is disabled. You can view scripts with \"gog appscript get\".",
	"gmail.settings.*":                       "Settings modification is disabled. View current settings with the corresponding \"get\" or \"list\" subcommand.",
	"gmail.track.*":                          "Email tracking is disabled. No alternative is available at this safety level.",
	"classroom.invitations.create":           "Creating invitations is disabled. No alternative is available at this safety level.",
	"classroom.guardian-invitations.create":  "Creating guardian invitations is disabled. No alternative is available at this safety level.",
	"classroom.announcements.create":         "Creating announcements is disabled. No alternative is available at this safety level.",
	"classroom.coursework.create":            "Creating coursework is disabled. No alternative is available at this safety level.",
}

// commandPath normalizes a kong command path into dot-separated form,
// dropping placeholder and flag tokens, then applying desire-path and
// legacy alias mappings.
func commandPath(kctx *kong.Context) string {
	parts := strings.Fields(kctx.Command())
	var tokens []string
	for _, p := range parts {
		if strings.HasPrefix(p, "<") || strings.HasPrefix(p, "--") {
			continue
		}
		tokens = append(tokens, p)
	}
	path := strings.Join(tokens, ".")
	if path == "" {
		return ""
	}

	// Map top-level desire-path shortcuts.
	if mapped, ok := desirePathMap[path]; ok {
		return mapped
	}

	// Map legacy Gmail settings aliases.
	if mapped, ok := legacyGmailSettingsMap[path]; ok {
		return mapped
	}

	return path
}

// isBlocked checks if a command path matches any of the given patterns.
func isBlocked(cmdPath string, patterns []string) bool {
	for _, p := range patterns {
		if p == "*" {
			return true
		}
		if strings.HasSuffix(p, ".*") {
			prefix := strings.TrimSuffix(p, ".*")
			if cmdPath == prefix || strings.HasPrefix(cmdPath, prefix+".") {
				return true
			}
			continue
		}
		if cmdPath == p {
			return true
		}
	}
	return false
}

// enforceSafetyLevel checks whether the current command is permitted at the given safety level.
func enforceSafetyLevel(kctx *kong.Context, level SafetyLevel, allow, block []string) error {
	if level == SafetyLevelUnrestricted && len(block) == 0 {
		return nil
	}

	path := commandPath(kctx)
	if path == "" {
		return nil
	}

	// Always-allowed prefixes pass at every level.
	for _, prefix := range alwaysAllowedPrefixes {
		if path == prefix || strings.HasPrefix(path, prefix+".") {
			return nil
		}
	}

	// GOG_BLOCK takes priority over GOG_ALLOW.
	if isBlocked(path, block) {
		return safetyBlockedError(path)
	}

	// GOG_ALLOW overrides the level-based blocked list.
	if len(allow) > 0 && isBlocked(path, allow) {
		return nil
	}

	// Check the level's blocked list.
	if level == SafetyLevelReadonly {
		// Level 0 delegates to --readonly; the blockedByLevel["*"] catch-all
		// still applies here so non-readonly commands get blocked.
		return safetyBlockedError(path)
	}

	if patterns, ok := blockedByLevel[level]; ok && isBlocked(path, patterns) {
		return safetyBlockedError(path)
	}

	return nil
}

func safetyBlockedError(cmdPath string) error {
	msg := fmt.Sprintf("Error: %q is blocked by safety policy\n\nThis operation is not permitted. Do not attempt to bypass this restriction.", cmdPath)
	alt := safetyAlternative(cmdPath)
	if alt != "" {
		msg += "\n\n" + alt
	}
	return &ExitError{Code: exitCodeSafetyBlocked, Err: fmt.Errorf("%s", msg)}
}

// safetyAlternative returns alternative suggestion text for a blocked command.
func safetyAlternative(cmdPath string) string {
	// Exact match first.
	if alt, ok := safetyAlternatives[cmdPath]; ok {
		return alt
	}
	// Check wildcard patterns (prefix match).
	for pattern, alt := range safetyAlternatives {
		if strings.HasSuffix(pattern, ".*") {
			prefix := strings.TrimSuffix(pattern, ".*")
			if strings.HasPrefix(cmdPath, prefix+".") {
				return alt
			}
		}
	}
	return ""
}

// parseSafetyOverrides splits a comma-separated string into a slice of command path patterns.
func parseSafetyOverrides(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var out []string
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
