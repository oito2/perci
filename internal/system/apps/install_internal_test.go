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

package apps

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/oito2/perci/internal/distro"
	"github.com/oito2/perci/internal/executor"
)

func TestParseFlatpakIDs(t *testing.T) {
	cases := []struct {
		name string
		out  string
		want map[string]bool
	}{
		{"empty output", "", map[string]bool{}},
		{
			"single id",
			"org.gnome.Calculator",
			map[string]bool{"org.gnome.Calculator": true},
		},
		{
			"multiple ids",
			"org.gnome.Calculator\norg.gnome.Loupe\ncom.spotify.Client",
			map[string]bool{"org.gnome.Calculator": true, "org.gnome.Loupe": true, "com.spotify.Client": true},
		},
		{
			"blank lines are ignored",
			"org.gnome.Calculator\n\n\norg.gnome.Loupe\n",
			map[string]bool{"org.gnome.Calculator": true, "org.gnome.Loupe": true},
		},
		{
			"surrounding whitespace is trimmed",
			"  org.gnome.Calculator  \n",
			map[string]bool{"org.gnome.Calculator": true},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseFlatpakIDs(tc.out)
			if len(got) != len(tc.want) {
				t.Fatalf("parseFlatpakIDs(%q) = %v, want %v", tc.out, got, tc.want)
			}
			for id := range tc.want {
				if !got[id] {
					t.Errorf("parseFlatpakIDs(%q) missing %q", tc.out, id)
				}
			}
		})
	}
}

// With flatpak already installed, EnsureFlatpak still adds the Flathub
// remote.
func TestEnsureFlatpak_AddsRemoteWhenFlatpakAlreadyInstalled(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	var out strings.Builder
	exe := executor.New(&out, &out)
	exe.DryRun = true
	exe.LookPath = func(name string) (string, error) { return "/usr/bin/" + name, nil } // flatpak present

	if err := EnsureFlatpak(context.Background(), exe, &out); err != nil {
		t.Fatalf("EnsureFlatpak: %v", err)
	}
	got := out.String()
	for _, want := range []string{"remote-add", "--if-not-exists", "flathub", flathubRepoURL} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in the commands run:\n%s", want, got)
		}
	}
}

// A system-scope install with flatpak missing: installing flatpak, adding
// Flathub and installing the apps share ONE password prompt.
func TestApply_SystemScopeSetupAndInstallOnePrompt(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // default config: system scope
	if distro.Detect() == distro.Unknown {
		t.Skip("needs a Debian- or Fedora-family host")
	}
	var out strings.Builder
	exe := executor.New(&out, &out)
	exe.DryRun = true
	exe.UsePolicyKit = true
	exe.LookPath = func(name string) (string, error) {
		if name == "flatpak" {
			return "", errors.New("not found")
		}
		return "/usr/bin/" + name, nil
	}
	if err := Apply(context.Background(), exe, &out, []string{Catalogue[0].FlatID}, nil); err != nil {
		t.Fatalf("Apply: %v\n%s", err, out.String())
	}
	s := out.String()
	if n := strings.Count(s, "pkexec"); n != 1 {
		t.Errorf("want one prompt, got %d:\n%s", n, s)
	}
	for _, want := range []string{"'flatpak'", "'remote-add'", "'install'", Catalogue[0].FlatID} {
		if !strings.Contains(s, want) {
			t.Errorf("batch missing %s:\n%s", want, s)
		}
	}
}

// Apply keeps both the user-scope failures and the privileged batch outcome.
func TestApplyResult(t *testing.T) {
	if err := applyResult(nil, nil); err != nil {
		t.Errorf("no failure: got %v", err)
	}
	if err := applyResult(nil, executor.ErrCompletedWithWarnings); !errors.Is(err, executor.ErrCompletedWithWarnings) {
		t.Errorf("warnings batch must surface ErrCompletedWithWarnings, got %v", err)
	}
	boom := errors.New("boom")
	err := applyResult([]string{"Loupe"}, boom)
	if !errors.Is(err, boom) || !strings.Contains(err.Error(), "1 aplicativo(s) com falha: Loupe") {
		t.Errorf("both failures must be reported, got %v", err)
	}
	if err := applyResult([]string{"Loupe"}, nil); err == nil || !strings.Contains(err.Error(), "Loupe") {
		t.Errorf("user failures alone must be reported, got %v", err)
	}
}

// The Debian flatpak install uses the shared unattended apt options.
func TestFlatpakSetupSteps_DebianUsesSharedAptOptions(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if distro.Detect() != distro.Debian {
		t.Skip("needs a Debian-family host")
	}
	exe := executor.New(nil, nil)
	exe.LookPath = func(string) (string, error) { return "", errors.New("not found") }
	steps, _, err := FlatpakSetupSteps(context.Background(), exe)
	if err != nil {
		t.Fatal(err)
	}
	args := strings.Join(steps[0].Args, " ")
	if !strings.Contains(args, "--force-confold") || strings.Join(steps[0].Env, " ") != strings.Join(distro.AptEnv, " ") {
		t.Errorf("apt step does not use the shared options: %v %v", steps[0].Env, steps[0].Args)
	}
}
