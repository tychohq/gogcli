package cmd

import (
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/alecthomas/kong"
)

func TestParseSafetyLevel_ValidNumbers(t *testing.T) {
	for i := 0; i <= 4; i++ {
		s := string(rune('0' + i))
		level, err := parseSafetyLevel(s)
		if err != nil {
			t.Fatalf("parseSafetyLevel(%q): %v", s, err)
		}
		if int(level) != i {
			t.Fatalf("parseSafetyLevel(%q) = %d, want %d", s, level, i)
		}
	}
}

func TestParseSafetyLevel_ValidNames(t *testing.T) {
	tests := map[string]SafetyLevel{
		"readonly":     SafetyLevelReadonly,
		"draft":        SafetyLevelDraft,
		"collaborate":  SafetyLevelCollaborate,
		"standard":     SafetyLevelStandard,
		"unrestricted": SafetyLevelUnrestricted,
	}
	for name, want := range tests {
		got, err := parseSafetyLevel(name)
		if err != nil {
			t.Fatalf("parseSafetyLevel(%q): %v", name, err)
		}
		if got != want {
			t.Fatalf("parseSafetyLevel(%q) = %d, want %d", name, got, want)
		}
	}
}

func TestParseSafetyLevel_CaseInsensitive(t *testing.T) {
	for _, s := range []string{"DRAFT", "Draft", "dRaFt"} {
		got, err := parseSafetyLevel(s)
		if err != nil {
			t.Fatalf("parseSafetyLevel(%q): %v", s, err)
		}
		if got != SafetyLevelDraft {
			t.Fatalf("parseSafetyLevel(%q) = %d, want %d", s, got, SafetyLevelDraft)
		}
	}
}

func TestParseSafetyLevel_Invalid(t *testing.T) {
	for _, s := range []string{"5", "-1", "bogus", "99", ""} {
		_, err := parseSafetyLevel(s)
		if err == nil {
			t.Fatalf("parseSafetyLevel(%q) should fail", s)
		}
		if strings.Contains(err.Error(), "readonly") || strings.Contains(err.Error(), "draft") {
			t.Fatalf("error should not list valid values: %v", err)
		}
	}
}

// parseTestKong creates a kong context for a CLI struct with the given args.
func parseTestKong(t *testing.T, args []string) *kong.Context {
	t.Helper()
	cli := &CLI{}
	parser, err := kong.New(
		cli,
		kong.Name("gog"),
		kong.Writers(io.Discard, io.Discard),
		kong.Exit(func(code int) {}),
		kong.Vars{
			"color":            "auto",
			"client":           "",
			"enabled_commands": "",
			"safety_level":     "4",
			"json":             "false",
			"plain":            "false",
			"version":          "test",
			"auth_services":    "",
			"calendar_weekday": "false",
		},
	)
	if err != nil {
		t.Fatalf("kong.New: %v", err)
	}
	kctx, err := parser.Parse(args)
	if err != nil {
		t.Fatalf("Parse(%v): %v", args, err)
	}
	return kctx
}

func TestCommandPath_NormalCommands(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{[]string{"gmail", "send", "--to", "a@b.com", "--subject", "s", "--body", "b"}, "gmail.send"},
		{[]string{"drive", "ls"}, "drive.ls"},
		{[]string{"version"}, "version"},
		{[]string{"gmail", "settings", "filters", "list"}, "gmail.settings.filters.list"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			kctx := parseTestKong(t, tt.args)
			got := commandPath(kctx)
			if got != tt.want {
				t.Fatalf("commandPath(%v) = %q, want %q", tt.args, got, tt.want)
			}
		})
	}
}

func TestCommandPath_DesirePathShortcuts(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{[]string{"send", "--to", "a@b.com", "--subject", "s", "--body", "b"}, "gmail.send"},
		{[]string{"upload", "file.txt"}, "drive.upload"},
		{[]string{"download", "fileId"}, "drive.download"},
		{[]string{"ls"}, "drive.ls"},
		{[]string{"search", "query"}, "drive.search"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			kctx := parseTestKong(t, tt.args)
			got := commandPath(kctx)
			if got != tt.want {
				t.Fatalf("commandPath(%v) = %q, want %q", tt.args, got, tt.want)
			}
		})
	}
}

func TestCommandPath_LegacyGmailAliases(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{[]string{"gmail", "vacation", "get"}, "gmail.settings.vacation.get"},
		{[]string{"gmail", "filters", "list"}, "gmail.settings.filters.list"},
		{[]string{"gmail", "sendas", "list"}, "gmail.settings.sendas.list"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			kctx := parseTestKong(t, tt.args)
			got := commandPath(kctx)
			if got != tt.want {
				t.Fatalf("commandPath(%v) = %q, want %q", tt.args, got, tt.want)
			}
		})
	}
}

func TestIsBlocked_ExactMatch(t *testing.T) {
	if !isBlocked("gmail.send", []string{"gmail.send", "chat.dm.send"}) {
		t.Fatal("expected gmail.send to be blocked")
	}
}

func TestIsBlocked_WildcardMatch(t *testing.T) {
	if !isBlocked("gmail.settings.watch.create", []string{"gmail.settings.watch.*"}) {
		t.Fatal("expected gmail.settings.watch.create to be blocked by wildcard")
	}
	if !isBlocked("gmail.settings.watch", []string{"gmail.settings.watch.*"}) {
		t.Fatal("expected gmail.settings.watch to be blocked by wildcard")
	}
}

func TestIsBlocked_NoMatch(t *testing.T) {
	if isBlocked("gmail.search", []string{"gmail.send", "gmail.settings.*"}) {
		t.Fatal("expected gmail.search to not be blocked")
	}
}

func TestIsBlocked_StarMatchesAll(t *testing.T) {
	if !isBlocked("anything.at.all", []string{"*"}) {
		t.Fatal("expected * to match everything")
	}
}

func TestEnforceSafetyLevel_UnrestrictedPassesAll(t *testing.T) {
	kctx := parseTestKong(t, []string{"send", "--to", "a@b.com", "--subject", "s", "--body", "b"})
	if err := enforceSafetyLevel(kctx, SafetyLevelUnrestricted, nil, nil); err != nil {
		t.Fatalf("unrestricted should not block: %v", err)
	}
}

func TestEnforceSafetyLevel_DraftBlocksSend(t *testing.T) {
	kctx := parseTestKong(t, []string{"send", "--to", "a@b.com", "--subject", "s", "--body", "b"})
	err := enforceSafetyLevel(kctx, SafetyLevelDraft, nil, nil)
	if err == nil {
		t.Fatal("draft should block send")
	}
	if ExitCode(err) != exitCodeSafetyBlocked {
		t.Fatalf("expected exit code %d, got %d", exitCodeSafetyBlocked, ExitCode(err))
	}
}

func TestEnforceSafetyLevel_DraftAllowsDraftCreate(t *testing.T) {
	kctx := parseTestKong(t, []string{"gmail", "drafts", "create", "--to", "a@b.com", "--subject", "s", "--body", "b"})
	if err := enforceSafetyLevel(kctx, SafetyLevelDraft, nil, nil); err != nil {
		t.Fatalf("draft should allow drafts create: %v", err)
	}
}

func TestEnforceSafetyLevel_DraftBlocksChatSend(t *testing.T) {
	kctx := parseTestKong(t, []string{"chat", "messages", "send", "spaces/x", "--text", "hi"})
	err := enforceSafetyLevel(kctx, SafetyLevelDraft, nil, nil)
	if err == nil {
		t.Fatal("draft should block chat messages send")
	}
}

func TestEnforceSafetyLevel_CollaborateBlocksSend(t *testing.T) {
	kctx := parseTestKong(t, []string{"send", "--to", "a@b.com", "--subject", "s", "--body", "b"})
	err := enforceSafetyLevel(kctx, SafetyLevelCollaborate, nil, nil)
	if err == nil {
		t.Fatal("collaborate should block send")
	}
}

func TestEnforceSafetyLevel_CollaborateAllowsCalendarRespond(t *testing.T) {
	kctx := parseTestKong(t, []string{"calendar", "respond", "primary", "eventId", "--status", "accepted"})
	if err := enforceSafetyLevel(kctx, SafetyLevelCollaborate, nil, nil); err != nil {
		t.Fatalf("collaborate should allow calendar respond: %v", err)
	}
}

func TestEnforceSafetyLevel_StandardBlocksBatchDelete(t *testing.T) {
	kctx := parseTestKong(t, []string{"gmail", "batch", "delete", "query"})
	err := enforceSafetyLevel(kctx, SafetyLevelStandard, nil, nil)
	if err == nil {
		t.Fatal("standard should block gmail batch delete")
	}
}

func TestEnforceSafetyLevel_StandardAllowsSend(t *testing.T) {
	kctx := parseTestKong(t, []string{"send", "--to", "a@b.com", "--subject", "s", "--body", "b"})
	if err := enforceSafetyLevel(kctx, SafetyLevelStandard, nil, nil); err != nil {
		t.Fatalf("standard should allow send: %v", err)
	}
}

func TestEnforceSafetyLevel_ReadonlyBlocksEverything(t *testing.T) {
	kctx := parseTestKong(t, []string{"drive", "ls"})
	err := enforceSafetyLevel(kctx, SafetyLevelReadonly, nil, nil)
	if err == nil {
		t.Fatal("readonly should block non-auth commands")
	}
}

func TestEnforceSafetyLevel_AlwaysAllowedPrefixes(t *testing.T) {
	for _, prefix := range alwaysAllowedPrefixes {
		var args []string
		switch prefix {
		case "version":
			args = []string{"version"}
		case "auth":
			args = []string{"auth", "status"}
		case "config":
			args = []string{"config", "path"}
		case "agent":
			args = []string{"agent", "exit-codes"}
		case "schema":
			args = []string{"schema"}
		case "completion":
			args = []string{"completion", "bash"}
		case "time":
			args = []string{"time", "now"}
		case "open":
			args = []string{"open", "some-id"}
		case "exit-codes":
			args = []string{"exit-codes"}
		default:
			t.Fatalf("unhandled always-allowed prefix: %s", prefix)
		}

		t.Run(prefix, func(t *testing.T) {
			kctx := parseTestKong(t, args)
			if err := enforceSafetyLevel(kctx, SafetyLevelReadonly, nil, nil); err != nil {
				t.Fatalf("always-allowed prefix %q should pass at readonly: %v", prefix, err)
			}
		})
	}
}

func TestEnforceSafetyLevel_AllowOverridesBlock(t *testing.T) {
	kctx := parseTestKong(t, []string{"send", "--to", "a@b.com", "--subject", "s", "--body", "b"})
	// GOG_ALLOW unblocks gmail.send.
	err := enforceSafetyLevel(kctx, SafetyLevelDraft, []string{"gmail.send"}, nil)
	if err != nil {
		t.Fatalf("GOG_ALLOW should override level block: %v", err)
	}
}

func TestEnforceSafetyLevel_BlockOverridesAllow(t *testing.T) {
	kctx := parseTestKong(t, []string{"drive", "ls"})
	// GOG_BLOCK wins when same path is in both.
	err := enforceSafetyLevel(kctx, SafetyLevelUnrestricted, []string{"drive.ls"}, []string{"drive.ls"})
	if err == nil {
		t.Fatal("GOG_BLOCK should override GOG_ALLOW")
	}
}

func TestEnforceSafetyLevel_BlockAddsToLevel(t *testing.T) {
	kctx := parseTestKong(t, []string{"drive", "ls"})
	// drive.ls is normally allowed at unrestricted, but GOG_BLOCK blocks it.
	err := enforceSafetyLevel(kctx, SafetyLevelUnrestricted, nil, []string{"drive.ls"})
	if err == nil {
		t.Fatal("GOG_BLOCK should block even at unrestricted")
	}
}

func TestEnforceSafetyLevel_ErrorFormat(t *testing.T) {
	kctx := parseTestKong(t, []string{"send", "--to", "a@b.com", "--subject", "s", "--body", "b"})
	err := enforceSafetyLevel(kctx, SafetyLevelDraft, nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if !strings.Contains(msg, `"gmail.send" is blocked by safety policy`) {
		t.Fatalf("error missing command path: %s", msg)
	}
	if !strings.Contains(msg, "Do not attempt to bypass this restriction") {
		t.Fatalf("error missing bypass warning: %s", msg)
	}
	if !strings.Contains(msg, "draft") {
		// The alternative text for gmail.send should mention creating a draft.
		t.Fatalf("error should include draft alternative: %s", msg)
	}
}

func TestParseSafetyOverrides(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"", nil},
		{"  ", nil},
		{"gmail.send", []string{"gmail.send"}},
		{"gmail.send, chat.dm.send", []string{"gmail.send", "chat.dm.send"}},
		{" gmail.send , , chat.dm.send , ", []string{"gmail.send", "chat.dm.send"}},
	}
	for _, tt := range tests {
		got := parseSafetyOverrides(tt.input)
		if !reflect.DeepEqual(got, tt.want) {
			t.Fatalf("parseSafetyOverrides(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestDesirePathMap_Completeness(t *testing.T) {
	// Every top-level CLI command should be either:
	// 1. In alwaysAllowedPrefixes
	// 2. In desirePathMap
	// 3. A service namespace (has subcommands, not a shortcut)
	// 4. An always-allowed shortcut (login, logout, status, me, whoami, etc.)

	allowedPrefixSet := make(map[string]bool)
	for _, p := range alwaysAllowedPrefixes {
		allowedPrefixSet[p] = true
	}

	serviceNamespaces := map[string]bool{
		"gmail": true, "drive": true, "docs": true, "slides": true,
		"calendar": true, "classroom": true, "chat": true, "contacts": true,
		"tasks": true, "people": true, "keep": true, "sheets": true,
		"forms": true, "appscript": true, "groups": true,
	}

	// Shortcuts that map to always-allowed operations.
	alwaysAllowedShortcuts := map[string]bool{
		"login": true, "logout": true, "status": true,
		"me": true, "whoami": true,
	}

	typ := reflect.TypeOf(CLI{})
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		tag := field.Tag.Get("cmd")
		if tag == "" {
			continue // not a command field (e.g., RootFlags embed)
		}
		name := field.Tag.Get("name")
		if name == "" {
			name = strings.ToLower(field.Name)
		}
		// Skip the internal completion helper and Version flag.
		if name == "__complete" || field.Type == reflect.TypeOf(kong.VersionFlag(false)) {
			continue
		}

		if allowedPrefixSet[name] {
			continue
		}
		if _, ok := desirePathMap[name]; ok {
			continue
		}
		if serviceNamespaces[name] {
			continue
		}
		if alwaysAllowedShortcuts[name] {
			continue
		}
		t.Errorf("CLI field %q (name=%q) is not covered by alwaysAllowedPrefixes, desirePathMap, serviceNamespaces, or alwaysAllowedShortcuts", field.Name, name)
	}
}

func TestLegacyGmailSettingsMap_Completeness(t *testing.T) {
	// All hidden GmailCmd subcommands should appear in legacyGmailSettingsMap.
	typ := reflect.TypeOf(GmailCmd{})
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		hidden := field.Tag.Get("hidden")
		if hidden == "" {
			continue
		}
		name := field.Tag.Get("name")
		if name == "" {
			name = strings.ToLower(field.Name)
		}
		key := "gmail." + name
		if _, ok := legacyGmailSettingsMap[key]; !ok {
			t.Errorf("hidden GmailCmd field %q (key=%q) is not in legacyGmailSettingsMap", field.Name, key)
		}
	}
}

func TestSafetyAlternative_ExactMatch(t *testing.T) {
	alt := safetyAlternative("gmail.send")
	if alt == "" {
		t.Fatal("expected alternative for gmail.send")
	}
	if !strings.Contains(alt, "draft") {
		t.Fatalf("gmail.send alternative should mention drafts: %s", alt)
	}
}

func TestSafetyAlternative_WildcardMatch(t *testing.T) {
	alt := safetyAlternative("gmail.settings.watch.create")
	if alt == "" {
		t.Fatal("expected alternative for gmail.settings.watch.create via wildcard")
	}
}

func TestSafetyAlternative_NoMatch(t *testing.T) {
	alt := safetyAlternative("drive.ls")
	if alt != "" {
		t.Fatalf("expected no alternative for drive.ls, got: %s", alt)
	}
}

func TestEnforceSafetyLevel_LegacyGmailAlias_Blocked(t *testing.T) {
	// "gmail watch" is a hidden alias for "gmail settings watch",
	// which should be blocked at draft level.
	kctx := parseTestKong(t, []string{"gmail", "watch", "start"})
	err := enforceSafetyLevel(kctx, SafetyLevelDraft, nil, nil)
	if err == nil {
		t.Fatal("gmail watch should be blocked at draft level via legacy alias mapping")
	}
}
