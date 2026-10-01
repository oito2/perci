// Copyright (C) 2026  oito2
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package executor_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oito2/perci/internal/executor"
)

func TestDryRun(t *testing.T) {
	var buf bytes.Buffer
	exe := executor.New(&buf, &buf)
	exe.DryRun = true

	if err := exe.Run(context.Background(), executor.Options{}, "echo", "hello"); err != nil {
		t.Fatalf("DryRun must not error: %v", err)
	}
	if !strings.Contains(buf.String(), "dry-run") {
		t.Errorf("expected dry-run marker in output, got: %q", buf.String())
	}
}

func TestDryRunSudoPrepended(t *testing.T) {
	var buf bytes.Buffer
	exe := executor.New(&buf, &buf)
	exe.DryRun = true

	if err := exe.Run(context.Background(), executor.Options{RequiresSudo: true}, "apt-get", "update"); err != nil {
		t.Fatalf("DryRun must not error: %v", err)
	}
	if !strings.Contains(buf.String(), "sudo") {
		t.Errorf("expected 'sudo' in dry-run output, got: %q", buf.String())
	}
}

func TestDryRunSudoPropagatesEnv(t *testing.T) {
	var buf bytes.Buffer
	exe := executor.New(&buf, &buf)
	exe.DryRun = true

	if err := exe.Run(context.Background(), executor.Options{
		RequiresSudo: true,
		Env:          []string{"DEBIAN_FRONTEND=noninteractive"},
	}, "apt-get", "update"); err != nil {
		t.Fatalf("DryRun must not error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "env") || !strings.Contains(out, "DEBIAN_FRONTEND=noninteractive") {
		t.Errorf("expected env to be forwarded through sudo via 'env KEY=VALUE ...', got: %q", out)
	}
}

func TestDryRunPolicyKitPrepended(t *testing.T) {
	// GUI processes (cmd/prci-gui) set UsePolicyKit — sudo's terminal
	// password prompt has nothing to attach to there. Escalation must go
	// through pkexec instead of sudo.
	var buf bytes.Buffer
	exe := executor.New(&buf, &buf)
	exe.DryRun = true
	exe.UsePolicyKit = true

	if err := exe.Run(context.Background(), executor.Options{RequiresSudo: true}, "mkcert", "-install"); err != nil {
		t.Fatalf("DryRun must not error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "pkexec") {
		t.Errorf("expected 'pkexec' in dry-run output, got: %q", out)
	}
	if strings.Contains(out, "/usr/bin/sudo") {
		t.Errorf("UsePolicyKit must not fall back to sudo, got: %q", out)
	}
}

func TestDryRunPolicyKitPropagatesEnv(t *testing.T) {
	var buf bytes.Buffer
	exe := executor.New(&buf, &buf)
	exe.DryRun = true
	exe.UsePolicyKit = true

	if err := exe.Run(context.Background(), executor.Options{
		RequiresSudo: true,
		Env:          []string{"DEBIAN_FRONTEND=noninteractive"},
	}, "apt-get", "update"); err != nil {
		t.Fatalf("DryRun must not error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "env") || !strings.Contains(out, "DEBIAN_FRONTEND=noninteractive") {
		t.Errorf("expected env to be forwarded through pkexec via 'env KEY=VALUE ...', got: %q", out)
	}
}

func TestUsePolicyKitDefaultsFalse(t *testing.T) {
	// TUI/CLI (cmd/prci) must keep today's sudo behavior unless they
	// explicitly opt in — UsePolicyKit is additive, not a global switch.
	var buf bytes.Buffer
	exe := executor.New(&buf, &buf)
	exe.DryRun = true

	if err := exe.Run(context.Background(), executor.Options{RequiresSudo: true}, "mkcert", "-install"); err != nil {
		t.Fatalf("DryRun must not error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "/usr/bin/sudo") {
		t.Errorf("expected sudo by default (UsePolicyKit unset), got: %q", out)
	}
	if strings.Contains(out, "pkexec") {
		t.Errorf("expected no pkexec by default, got: %q", out)
	}
}

func TestRunEcho(t *testing.T) {
	var buf bytes.Buffer
	exe := executor.New(nil, nil)

	err := exe.Run(context.Background(), executor.Options{Stdout: &buf}, "echo", "lumina")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "lumina") {
		t.Errorf("expected 'lumina' in output, got: %q", buf.String())
	}
}

func TestRunFailure(t *testing.T) {
	exe := executor.New(nil, nil)
	err := exe.Run(context.Background(), executor.Options{}, "false")
	if err == nil {
		t.Fatal("expected error from 'false' command")
	}
}

func TestOutput(t *testing.T) {
	exe := executor.New(nil, nil)
	out, err := exe.Output(context.Background(), executor.Options{}, "echo", "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "hello") {
		t.Errorf("expected 'hello' in output, got: %q", out)
	}
}

func TestOutputDryRun(t *testing.T) {
	exe := executor.New(nil, nil)
	exe.DryRun = true
	out, err := exe.Output(context.Background(), executor.Options{}, "whoami")
	if err != nil {
		t.Fatalf("DryRun Output must not error: %v", err)
	}
	if !strings.Contains(out, "dry-run") {
		t.Errorf("expected dry-run marker in output, got: %q", out)
	}
}

func TestContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	exe := executor.New(nil, nil)
	err := exe.Run(ctx, executor.Options{}, "sleep", "10")
	if err == nil {
		t.Fatal("expected error with cancelled context")
	}
}

// RunSudoSequence (decided with the user on 2026-09-14): one authentication
// for a whole batch of privileged commands instead of one prompt per
// command — see internal/system/update.Run, its first real caller, for
// the motivating case (up to 7 pkexec prompts for one "Atualizar Sistema"
// click before this existed).

func TestRunSudoSequence_DryRunEscalatesOnce(t *testing.T) {
	var buf bytes.Buffer
	exe := executor.New(&buf, &buf)
	exe.DryRun = true

	if err := exe.RunSudoSequence(context.Background(), executor.Options{}, []executor.PrivilegedStep{
		{Name: "apt-get", Args: []string{"update"}},
		{Name: "apt-get", Args: []string{"upgrade", "-y"}},
		{Name: "snap", Args: []string{"refresh"}},
	}); err != nil {
		t.Fatalf("DryRun must not error: %v", err)
	}
	out := buf.String()
	if got := strings.Count(out, "/usr/bin/sudo"); got != 1 {
		t.Errorf("expected exactly 1 escalation for the whole batch, got %d: %q", got, out)
	}
	if strings.Count(out, "sh -c") == 0 {
		t.Errorf("expected the batch to run as one 'sh -c' script, got: %q", out)
	}
}

func TestRunSudoSequence_PolicyKitEscalatesOnce(t *testing.T) {
	var buf bytes.Buffer
	exe := executor.New(&buf, &buf)
	exe.DryRun = true
	exe.UsePolicyKit = true

	if err := exe.RunSudoSequence(context.Background(), executor.Options{}, []executor.PrivilegedStep{
		{Name: "apt-get", Args: []string{"update"}},
		{Name: "apt-get", Args: []string{"upgrade", "-y"}},
	}); err != nil {
		t.Fatalf("DryRun must not error: %v", err)
	}
	out := buf.String()
	if got := strings.Count(out, "pkexec"); got != 1 {
		t.Errorf("expected exactly 1 pkexec invocation for the whole batch, got %d: %q", got, out)
	}
	if strings.Contains(out, "/usr/bin/sudo") {
		t.Errorf("UsePolicyKit must not fall back to sudo, got: %q", out)
	}
}

func TestRunSudoSequence_StepsRunInOrder(t *testing.T) {
	var buf bytes.Buffer
	exe := executor.New(&buf, &buf)
	exe.DryRun = true

	if err := exe.RunSudoSequence(context.Background(), executor.Options{}, []executor.PrivilegedStep{
		{Name: "apt-get", Args: []string{"update"}},
		{Name: "apt-get", Args: []string{"upgrade"}},
		{Name: "journalctl", Args: []string{"--vacuum-time=7d"}},
	}); err != nil {
		t.Fatalf("DryRun must not error: %v", err)
	}
	out := buf.String()
	iUpdate := strings.Index(out, "'update'")
	iUpgrade := strings.Index(out, "'upgrade'")
	iJournal := strings.Index(out, "'journalctl'")
	if iUpdate < 0 || iUpgrade < 0 || iJournal < 0 || iUpdate >= iUpgrade || iUpgrade >= iJournal {
		t.Errorf("expected steps in order update < upgrade < journalctl, got: %q", out)
	}
}

func TestRunSudoSequence_AnnounceIsEchoed(t *testing.T) {
	var buf bytes.Buffer
	exe := executor.New(&buf, &buf)
	exe.DryRun = true

	if err := exe.RunSudoSequence(context.Background(), executor.Options{}, []executor.PrivilegedStep{
		{Announce: "Sincronizando lista de pacotes...", Name: "apt-get", Args: []string{"update"}},
	}); err != nil {
		t.Fatalf("DryRun must not error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "printf '%s\\n'") || !strings.Contains(out, "Sincronizando lista de pacotes...") {
		t.Errorf("expected Announce to be echoed before the command, got: %q", out)
	}
}

func TestRunSudoSequence_SoftStepFallsBackInsteadOfAborting(t *testing.T) {
	var buf bytes.Buffer
	exe := executor.New(&buf, &buf)
	exe.DryRun = true

	if err := exe.RunSudoSequence(context.Background(), executor.Options{}, []executor.PrivilegedStep{
		{Name: "apt-get", Args: []string{"update"}},
		{Soft: true, WarnMessage: "Falha ao atualizar snaps.", Name: "snap", Args: []string{"refresh"}},
	}); err != nil {
		t.Fatalf("DryRun must not error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "|| printf '%s\\n'") || !strings.Contains(out, "Falha ao atualizar snaps.") {
		t.Errorf("expected the soft step to fall back to its WarnMessage instead of aborting, got: %q", out)
	}
}

func TestRunSudoSequence_QuotesSingleQuotesInArgs(t *testing.T) {
	var buf bytes.Buffer
	exe := executor.New(&buf, &buf)
	exe.DryRun = true

	if err := exe.RunSudoSequence(context.Background(), executor.Options{}, []executor.PrivilegedStep{
		{Name: "echo", Args: []string{"it's a test"}},
	}); err != nil {
		t.Fatalf("DryRun must not error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, `it'\''s a test`) {
		t.Errorf("expected a single quote in an arg to be shell-escaped as '\\'', got: %q", out)
	}
}

// TestShellQuote moved here from internal/manager/db — ShellQuote is the
// security-sensitive primitive behind every raw shell string this project
// builds (RunSudoSequence above, internal/manager/db, internal/system/
// fonts); it belongs with its one canonical implementation, not duplicated
// per package.
func TestShellQuote(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty", "", "''"},
		{"simple", "hello", "'hello'"},
		{"with spaces", "hello world", "'hello world'"},
		{"single quote", "it's", "'it'\\''s'"},
		{"multiple quotes", "can't stop, won't stop", "'can'\\''t stop, won'\\''t stop'"},
		{"only single quote", "'", "''\\'''"},
		// ShellQuote is the project's single most security-sensitive
		// primitive (its own doc comment says as much) — these cover every
		// other shell metacharacter besides a bare single quote, since
		// wrapping in single quotes must neutralize all of them, not just
		// quotes.
		{"command substitution dollar-paren", "$(rm -rf /)", "'$(rm -rf /)'"},
		{"backtick command substitution", "`whoami`", "'`whoami`'"},
		{"semicolon command separator", "a; rm -rf /", "'a; rm -rf /'"},
		{"double ampersand", "a && rm -rf /", "'a && rm -rf /'"},
		{"pipe", "a | rm -rf /", "'a | rm -rf /'"},
		{"embedded newline", "a\nrm -rf /", "'a\nrm -rf /'"},
		{"variable expansion", "$HOME/$USER", "'$HOME/$USER'"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := executor.ShellQuote(tt.input); got != tt.want {
				t.Errorf("ShellQuote(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestShellQuote_SurvivesRealShell proves ShellQuote's output is treated as
// inert data by an actual shell, not reinterpreted — a payload packed with
// every metacharacter above, echoed back through a real `sh -c`, must come
// out byte-for-byte identical, and none of its embedded "touch <canary>"
// attempts may have actually run. Uses touch (not e.g. rm -rf) as the
// canary specifically so a real regression here only ever creates one empty
// file inside this test's own t.TempDir(), never destroys anything.
func TestShellQuote_SurvivesRealShell(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}
	dir := t.TempDir()
	canary := filepath.Join(dir, "canary")
	payload := "$(touch " + canary + ") `touch " + canary + "` ; touch " + canary + " && touch " + canary + " | touch " + canary
	out, err := exec.Command("sh", "-c", "printf '%s' "+executor.ShellQuote(payload)).Output()
	if err != nil {
		t.Fatalf("sh -c failed: %v", err)
	}
	if string(out) != payload {
		t.Errorf("payload not preserved verbatim by the shell — got %q, want %q", out, payload)
	}
	if _, err := os.Stat(canary); err == nil {
		t.Error("canary file exists — a shell metacharacter in the payload was executed instead of neutralized by ShellQuote")
	}
}

func TestRunSudoSequence_PropagatesPerStepEnv(t *testing.T) {
	var buf bytes.Buffer
	exe := executor.New(&buf, &buf)
	exe.DryRun = true

	if err := exe.RunSudoSequence(context.Background(), executor.Options{}, []executor.PrivilegedStep{
		{Env: []string{"DEBIAN_FRONTEND=noninteractive"}, Name: "apt-get", Args: []string{"upgrade", "-y"}},
	}); err != nil {
		t.Fatalf("DryRun must not error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "env") || !strings.Contains(out, "DEBIAN_FRONTEND=noninteractive") {
		t.Errorf("expected per-step env to be forwarded via 'env KEY=VALUE ...', got: %q", out)
	}
}

func TestCurrentUser(t *testing.T) {
	t.Setenv("SUDO_USER", "")
	t.Setenv("USER", "")
	t.Setenv("LOGNAME", "fallback-user")
	if got := executor.CurrentUser(); got != "fallback-user" {
		t.Errorf("CurrentUser() = %q, want %q (LOGNAME fallback)", got, "fallback-user")
	}

	t.Setenv("USER", "regular-user")
	if got := executor.CurrentUser(); got != "regular-user" {
		t.Errorf("CurrentUser() = %q, want %q (USER over LOGNAME)", got, "regular-user")
	}

	t.Setenv("SUDO_USER", "original-user")
	if got := executor.CurrentUser(); got != "original-user" {
		t.Errorf("CurrentUser() = %q, want %q (SUDO_USER takes precedence)", got, "original-user")
	}
}
