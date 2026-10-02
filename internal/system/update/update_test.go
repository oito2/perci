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

package update

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/oito2/perci/internal/distro"
	"github.com/oito2/perci/internal/executor"
	"github.com/oito2/perci/internal/ui"
)

// dryRunExecutor returns an Executor that logs "[dry-run] <cmd> <args>" to
// buf instead of actually running anything, and reports every command as
// installed — so the presence checks in Run/updateFlatpak always proceed,
// regardless of what the machine running the test has.
func dryRunExecutor(buf *bytes.Buffer) *executor.Executor {
	return &executor.Executor{DryRun: true, Stdout: buf, Stderr: buf,
		LookPath: func(name string) (string, error) { return "/usr/bin/" + name, nil }}
}

// sequentialIndexes fails the test unless every substring in want appears in
// out, in the given order (later substrings must start after earlier ones
// end) — this is how we assert on command *sequence*, not just presence.
func sequentialIndexes(t *testing.T, out string, want []string) {
	t.Helper()
	pos := 0
	for _, w := range want {
		idx := strings.Index(out[pos:], w)
		if idx == -1 {
			t.Fatalf("expected %q to appear after position %d\nfull output:\n%s", w, pos, out)
		}
		pos += idx + len(w)
	}
}

func TestPackageSteps_Debian(t *testing.T) {
	steps, err := packageSteps(distro.Debian, true)
	if err != nil {
		t.Fatalf("packageSteps: %v", err)
	}

	var names []string
	for _, s := range steps {
		names = append(names, s.Name+" "+strings.Join(s.Args, " "))
		if len(s.Env) == 0 || s.Env[0] != "DEBIAN_FRONTEND=noninteractive" {
			t.Errorf("apt-get step %q must set DEBIAN_FRONTEND=noninteractive, got Env=%v", s.Announce, s.Env)
		}
	}
	joined := strings.Join(names, " | ")
	sequentialIndexes(t, joined, []string{
		"apt-get update",
		"apt-get full-upgrade",
		"apt-get autoremove",
		"apt-get autoclean",
	})
	if strings.Contains(joined, "apt-get upgrade") {
		t.Error("full-upgrade already covers a plain upgrade")
	}
}

// Autoremove is opt-in (update.Options): without it no package is removed.
func TestPackageSteps_NoAutoremoveByDefault(t *testing.T) {
	for _, family := range []string{distro.Debian, distro.Fedora} {
		steps, err := packageSteps(family, false)
		if err != nil {
			t.Fatal(err)
		}
		for _, s := range steps {
			if strings.Contains(strings.Join(s.Args, " "), "autoremove") {
				t.Errorf("%s: autoremove without the option", family)
			}
		}
	}
}

func TestPackageSteps_Fedora(t *testing.T) {
	steps, err := packageSteps(distro.Fedora, true)
	if err != nil {
		t.Fatalf("packageSteps: %v", err)
	}
	if len(steps) != 2 {
		t.Fatalf("got %d steps, want 2", len(steps))
	}
	if steps[0].Soft {
		t.Error("dnf upgrade must not be Soft — a failed upgrade must abort the batch")
	}
	if !steps[1].Soft {
		t.Error("dnf autoremove must be Soft — matches the old ui.Warning-not-return behavior")
	}
	joined := steps[0].Name + " " + strings.Join(steps[0].Args, " ") + " | " + steps[1].Name + " " + strings.Join(steps[1].Args, " ")
	sequentialIndexes(t, joined, []string{
		"dnf upgrade --refresh -y",
		"dnf autoremove -y",
	})
}

func TestPackageSteps_UnknownFamilyErrors(t *testing.T) {
	if _, err := packageSteps(distro.Unknown, false); err == nil {
		t.Fatal("expected an error for an unsupported distro family")
	}
}

func TestUpdateFlatpak_UpdatesThenRemovesUnused(t *testing.T) {
	var buf bytes.Buffer
	exe := dryRunExecutor(&buf)

	if err := updateFlatpak(context.Background(), exe, &buf, true); err != nil {
		t.Fatalf("updateFlatpak: %v", err)
	}
	sequentialIndexes(t, buf.String(), []string{
		"flatpak [update",
		"flatpak [uninstall",
	})
	// No scope flag on the update: both installations are updated.
	if line := strings.SplitN(buf.String()[strings.Index(buf.String(), "flatpak [update"):], "\n", 2)[0]; strings.Contains(line, "--system") || strings.Contains(line, "--user") {
		t.Errorf("flatpak update must cover both scopes: %s", line)
	}

	buf.Reset()
	if err := updateFlatpak(context.Background(), exe, &buf, false); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "uninstall") {
		t.Errorf("unused runtimes are only removed with autoremove:\n%s", buf.String())
	}
}

func TestRun_ReportsStepsForBothPhases(t *testing.T) {
	// dryRunExecutor reports every command as installed, so both phases
	// (the privileged batch + flatpak) are counted deterministically.
	// Packages + snap + journal are one single phase, so total is 2.

	var buf bytes.Buffer
	exe := dryRunExecutor(&buf)

	var labels []string
	ui.SetStepHook(func(index, total int, label string) {
		if total != 2 {
			t.Errorf("Step total = %d, want 2", total)
		}
		if index != len(labels)+1 {
			t.Errorf("Step index = %d, want %d (sequential, 1-based)", index, len(labels)+1)
		}
		labels = append(labels, label)
	})
	t.Cleanup(func() { ui.SetStepHook(nil) })

	if err := Run(context.Background(), exe, &buf, Options{}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	want := []string{
		"Atualizando pacotes do sistema...",
		"Atualizando Flatpaks...",
	}
	if len(labels) != len(want) {
		t.Fatalf("got %d Step calls %v, want %d %v", len(labels), labels, len(want), want)
	}
	for i := range want {
		if labels[i] != want[i] {
			t.Errorf("step[%d] label = %q, want %q", i, labels[i], want[i])
		}
	}
}

func TestRun_PrivilegedStepsRunAsOneSudoSequence(t *testing.T) {
	// One authentication for every privileged command in a single run:
	// the escalation binary appears exactly once in the dry-run trace,
	// even though apt-get(x4)+snap+journalctl are all privileged.

	var buf bytes.Buffer
	exe := dryRunExecutor(&buf)

	if err := Run(context.Background(), exe, &buf, Options{CleanJournal: true, Autoremove: true}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	out := buf.String()
	if got := strings.Count(out, "/usr/bin/sudo"); got != 1 {
		t.Errorf("expected exactly 1 escalation (sudo) invocation for the whole run, got %d\noutput:\n%s", got, out)
	}
	// Args are individually shell-quoted inside the batch script, so each
	// token is what appears.
	sequentialIndexes(t, out, []string{
		"'update'",
		"'full-upgrade'",
		"'autoremove'",
		"'autoclean'",
		"'snap'", "'refresh'",
		"'journalctl'", "'--vacuum-time=7d'",
	})
}

// Without the options, nothing is vacuumed or removed.
func TestRun_CleanupsAreOptIn(t *testing.T) {

	var buf bytes.Buffer
	if err := Run(context.Background(), dryRunExecutor(&buf), &buf, Options{}); err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"journalctl", "autoremove", "--unused"} {
		if strings.Contains(buf.String(), s) {
			t.Errorf("%s ran without its option:\n%s", s, buf.String())
		}
	}
}
