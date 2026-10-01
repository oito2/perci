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

// Regression: with flatpak already installed, EnsureFlatpak returned before
// adding the Flathub remote, so every "flatpak install ... flathub" failed
// on distros that ship flatpak without it.
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
