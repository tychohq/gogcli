package cmd

import (
	"errors"
	"strings"
	"testing"
)

func TestEnvOr(t *testing.T) {
	t.Setenv("X_TEST", "")
	if got := envOr("X_TEST", "fallback"); got != "fallback" {
		t.Fatalf("unexpected: %q", got)
	}
	t.Setenv("X_TEST", "value")
	if got := envOr("X_TEST", "fallback"); got != "value" {
		t.Fatalf("unexpected: %q", got)
	}
}

func TestExecute_Help(t *testing.T) {
	out := captureStdout(t, func() {
		_ = captureStderr(t, func() {
			if err := Execute([]string{"--help"}); err != nil {
				t.Fatalf("Execute: %v", err)
			}
		})
	})
	if !strings.Contains(out, "Google CLI") && !strings.Contains(out, "Usage:") {
		t.Fatalf("unexpected help output: %q", out)
	}
	if !strings.Contains(out, "config.json") || !strings.Contains(out, "keyring backend") {
		t.Fatalf("expected config info in help output: %q", out)
	}
	if strings.Contains(out, "gmail (mail,email) thread get") {
		t.Fatalf("expected collapsed help (no expanded subcommands), got: %q", out)
	}
}

func TestExecute_Help_GmailHasGroupsAndRelativeCommands(t *testing.T) {
	out := captureStdout(t, func() {
		_ = captureStderr(t, func() {
			if err := Execute([]string{"gmail", "--help"}); err != nil {
				t.Fatalf("Execute: %v", err)
			}
		})
	})
	if !strings.Contains(out, "\nRead\n") || !strings.Contains(out, "\nWrite\n") || !strings.Contains(out, "\nAdmin\n") {
		t.Fatalf("expected command groups in gmail help, got: %q", out)
	}
	if !strings.Contains(out, "\n  search") || !strings.Contains(out, "Search threads using Gmail query syntax") {
		t.Fatalf("expected relative command summaries in gmail help, got: %q", out)
	}
	if strings.Contains(out, "\n  gmail (mail,email) search <query>") {
		t.Fatalf("unexpected full command prefix in gmail help, got: %q", out)
	}
	if strings.Contains(out, "\n  watch <command>") {
		t.Fatalf("expected watch to be under gmail settings (not top-level gmail help), got: %q", out)
	}
	if !strings.Contains(out, "\n  settings <command>") {
		t.Fatalf("expected settings subgroup in gmail help, got: %q", out)
	}
}

func TestExecute_UnknownCommand(t *testing.T) {
	errText := captureStderr(t, func() {
		_ = captureStdout(t, func() {
			if err := Execute([]string{"no_such_cmd"}); err == nil {
				t.Fatalf("expected error")
			}
		})
	})
	if errText == "" {
		t.Fatalf("expected stderr output")
	}
}

func TestExecute_UnknownFlag(t *testing.T) {
	errText := captureStderr(t, func() {
		_ = captureStdout(t, func() {
			if err := Execute([]string{"--definitely-nope"}); err == nil {
				t.Fatalf("expected error")
			}
		})
	})
	if errText == "" {
		t.Fatalf("expected stderr output")
	}
}

func TestExecute_SafetyLevel_BlocksSend(t *testing.T) {
	t.Setenv("GOG_SAFETY_LEVEL", "1")
	errText := captureStderr(t, func() {
		_ = captureStdout(t, func() {
			err := Execute([]string{"send", "--to", "a@b.com", "--subject", "s", "--body", "b"})
			if err == nil {
				t.Fatal("expected error")
			}
			if ExitCode(err) != exitCodeSafetyBlocked {
				t.Fatalf("expected exit code %d, got %d", exitCodeSafetyBlocked, ExitCode(err))
			}
		})
	})
	if !strings.Contains(errText, "blocked by safety policy") {
		t.Fatalf("expected safety error on stderr, got: %q", errText)
	}
}

func TestExecute_SafetyLevel_Flag(t *testing.T) {
	errText := captureStderr(t, func() {
		_ = captureStdout(t, func() {
			err := Execute([]string{"--safety-level", "1", "send", "--to", "a@b.com", "--subject", "s", "--body", "b"})
			if err == nil {
				t.Fatal("expected error")
			}
			if ExitCode(err) != exitCodeSafetyBlocked {
				t.Fatalf("expected exit code %d, got %d", exitCodeSafetyBlocked, ExitCode(err))
			}
		})
	})
	if !strings.Contains(errText, "blocked by safety policy") {
		t.Fatalf("expected safety error on stderr, got: %q", errText)
	}
}

func TestExecute_SafetyLevel_InvalidValue(t *testing.T) {
	t.Setenv("GOG_SAFETY_LEVEL", "bogus")
	errText := captureStderr(t, func() {
		_ = captureStdout(t, func() {
			err := Execute([]string{"version"})
			if err == nil {
				t.Fatal("expected error for invalid safety level")
			}
			if ExitCode(err) != 2 {
				t.Fatalf("expected exit code 2, got %d", ExitCode(err))
			}
		})
	})
	if !strings.Contains(errText, "invalid safety level") {
		t.Fatalf("expected invalid safety level error on stderr, got: %q", errText)
	}
}

func TestExecute_SafetyLevel_GOGAllow(t *testing.T) {
	t.Setenv("GOG_SAFETY_LEVEL", "1")
	t.Setenv("GOG_ALLOW", "gmail.send")
	// gmail.send is blocked at level 1 (draft), but GOG_ALLOW overrides.
	// We still expect an auth error since we have no credentials,
	// but NOT a safety-blocked error.
	_ = captureStderr(t, func() {
		_ = captureStdout(t, func() {
			err := Execute([]string{"send", "--to", "a@b.com", "--subject", "s", "--body", "b"})
			if err == nil {
				return // allowed through safety — may fail for other reasons
			}
			if ExitCode(err) == exitCodeSafetyBlocked {
				t.Fatal("GOG_ALLOW should override safety block")
			}
		})
	})
}

func TestExecute_SafetyLevel_GOGBlock(t *testing.T) {
	t.Setenv("GOG_SAFETY_LEVEL", "4")
	t.Setenv("GOG_BLOCK", "drive.ls")
	errText := captureStderr(t, func() {
		_ = captureStdout(t, func() {
			err := Execute([]string{"drive", "ls"})
			if err == nil {
				t.Fatal("expected error")
			}
			if ExitCode(err) != exitCodeSafetyBlocked {
				t.Fatalf("expected exit code %d, got %d", exitCodeSafetyBlocked, ExitCode(err))
			}
		})
	})
	if !strings.Contains(errText, "blocked by safety policy") {
		t.Fatalf("expected safety error on stderr, got: %q", errText)
	}
}

func TestExecute_SafetyLevel_DefaultUnrestricted(t *testing.T) {
	// Without GOG_SAFETY_LEVEL set, default is 4 (unrestricted).
	// drive ls should pass safety (may fail for other reasons like no auth).
	_ = captureStderr(t, func() {
		_ = captureStdout(t, func() {
			err := Execute([]string{"drive", "ls"})
			if err != nil && ExitCode(err) == exitCodeSafetyBlocked {
				t.Fatal("default level (4) should not block drive ls")
			}
		})
	})
}

func TestNewUsageError(t *testing.T) {
	if newUsageError(nil) != nil {
		t.Fatalf("expected nil for nil error")
	}

	err := errors.New("bad")
	wrapped := newUsageError(err)
	if wrapped == nil {
		t.Fatalf("expected wrapped error")
	}
	var exitErr *ExitError
	if !errors.As(wrapped, &exitErr) || exitErr.Code != 2 || !errors.Is(exitErr.Err, err) {
		t.Fatalf("unexpected wrapped error: %#v", wrapped)
	}
}
