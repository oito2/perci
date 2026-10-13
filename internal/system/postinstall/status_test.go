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

package postinstall

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/oito2/perci/internal/executor"
)

// fakePath replaces PATH with a directory holding only the given scripts.
func fakePath(t *testing.T, scripts map[string]string) {
	t.Helper()
	dir := t.TempDir()
	for name, body := range scripts {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir)
}

func newTestProbe() *Probe {
	return NewProbe(context.Background(), executor.New(nil, nil))
}

// Installed packages and the names they provide count; packages in any
// other state don't.
func TestParseDpkgInstalled(t *testing.T) {
	out := "ii |mesa-libgallium|mesa-va-drivers (= 25.0.7), mesa-vdpau-drivers\n" +
		"ii |git|\n" +
		"hi |held-pkg|held-virtual\n" +
		"rc |old-pkg|old-virtual\n" +
		"un |never|\n" +
		"garbage line\n"
	got := parseDpkgInstalled(out)
	for _, name := range []string{"mesa-libgallium", "mesa-va-drivers", "mesa-vdpau-drivers", "git", "held-pkg", "held-virtual"} {
		if !got[name] {
			t.Errorf("%q should be installed", name)
		}
	}
	for _, name := range []string{"old-pkg", "old-virtual", "never"} {
		if got[name] {
			t.Errorf("%q should not be installed", name)
		}
	}
}

// debInstalled needs every package and runs dpkg-query only once per Probe.
func TestProbe_DebInstalledCachesTheList(t *testing.T) {
	counter := filepath.Join(t.TempDir(), "calls")
	fakePath(t, map[string]string{
		"dpkg-query": "echo x >> '" + counter + "'\nprintf 'ii |git|\\nii |curl|\\n'",
	})
	p := newTestProbe()
	if !p.debInstalled("git", "curl") {
		t.Error("git and curl are installed")
	}
	if p.debInstalled("git", "jq") {
		t.Error("jq is not installed")
	}
	data, err := os.ReadFile(counter)
	if err != nil {
		t.Fatal(err)
	}
	if n := len(data) / 2; n != 1 {
		t.Errorf("dpkg-query ran %d times, want 1", n)
	}
}

func TestProbe_SystemChecks(t *testing.T) {
	fakePath(t, map[string]string{
		"swapon":    "echo /swapfile",
		"systemctl": "echo enabled",
		"rpm":       `for a in "$@"; do [ "$a" = missing ] && exit 1; done; exit 0`,
	})
	p := newTestProbe()
	if !p.swapActive() {
		t.Error("swap is active")
	}
	if !p.trimEnabled() {
		t.Error("fstrim.timer is enabled")
	}
	if !p.rpmInstalled("ffmpeg", "unrar") {
		t.Error("ffmpeg and unrar are installed")
	}
	if p.rpmInstalled("ffmpeg", "missing") {
		t.Error("a missing package makes rpmInstalled false")
	}

	fakePath(t, map[string]string{
		"swapon":    "exit 0",
		"systemctl": "echo disabled; exit 1",
	})
	if p.swapActive() {
		t.Error("no swap is listed")
	}
	if p.trimEnabled() {
		t.Error("fstrim.timer is disabled")
	}
}

// Flatpak counts only with the Flathub remote and every app installed.
func TestProbe_FlatpakAppsInstalled(t *testing.T) {
	fakePath(t, map[string]string{
		"flatpak": `case "$1" in remotes) echo flathub;; list) echo org.videolan.VLC;; esac`,
	})
	p := newTestProbe()
	if !p.flatpakAppsInstalled("org.videolan.VLC") {
		t.Error("VLC is installed from Flathub")
	}
	if p.flatpakAppsInstalled("org.videolan.VLC", "net.codelogistics.webapps") {
		t.Error("the web-app manager is not installed")
	}
	fakePath(t, map[string]string{"flatpak": `case "$1" in remotes) echo fedora;; list) echo org.videolan.VLC;; esac`})
	if p.flatpakAppsInstalled("org.videolan.VLC") {
		t.Error("without the Flathub remote the action isn't applied")
	}
	fakePath(t, nil)
	if p.flatpakAppsInstalled("org.videolan.VLC") {
		t.Error("without flatpak the action isn't applied")
	}
}

// Detected actions follow their Applied func; the others follow the record.
func TestProfile_Status(t *testing.T) {
	profile := Profile{ID: "test", Actions: []Action{
		{ID: "detected-yes", Applied: func(*Probe) bool { return true }},
		{ID: "detected-no", Applied: func(*Probe) bool { return false }},
		{ID: "recorded"},
		{ID: "not-recorded"},
	}}
	got := profile.Status(newTestProbe(), []string{"recorded", "detected-no"})
	want := map[string]bool{"detected-yes": true, "detected-no": false, "recorded": true, "not-recorded": false}
	for id, w := range want {
		if got[id] != w {
			t.Errorf("%s = %v, want %v", id, got[id], w)
		}
	}
}

// Only the actions that leave no state on the system go without detection.
func TestProfiles_UndetectableActions(t *testing.T) {
	undetectable := []string{"update-index", "upgrade-system", "detect-drivers", "cleanup"}
	for _, p := range allProfiles() {
		for _, a := range p.Actions {
			if (a.Applied == nil) != slices.Contains(undetectable, a.ID) {
				t.Errorf("%s/%s: Applied set = %v", p.ID, a.ID, a.Applied != nil)
			}
		}
	}
}

// The Flatpak check lists the installed apps once for the whole Probe.
func TestProbe_FlatpakListRunsOnce(t *testing.T) {
	counter := filepath.Join(t.TempDir(), "calls")
	fakePath(t, map[string]string{
		"flatpak": `case "$1" in remotes) echo flathub;; list) echo x >> '` + counter + `'; printf 'org.a\norg.b\n';; esac`,
	})
	p := newTestProbe()
	if !p.flatpakAppsInstalled("org.a") || !p.flatpakAppsInstalled("org.a", "org.b") || p.flatpakAppsInstalled("org.c") {
		t.Error("unexpected flatpak result")
	}
	data, err := os.ReadFile(counter)
	if err != nil {
		t.Fatal(err)
	}
	if n := len(data) / 2; n != 1 {
		t.Errorf("flatpak list ran %d times, want 1", n)
	}
}

// Only enabled "deb" sources count, in both one-line and DEB822 files.
func TestAptComponents(t *testing.T) {
	dir := t.TempDir()
	list := filepath.Join(dir, "sources.list")
	d := filepath.Join(dir, "sources.list.d")
	if err := os.Mkdir(d, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(path, body string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(list, "# deb http://x noble commented\ndeb-src http://x noble srcOnly\ndeb [arch=amd64 signed-by=/k] http://x noble main restricted\n")
	write(filepath.Join(d, "a.sources"),
		"Types: deb\nURIs: http://x\nSuites: noble\nComponents: universe\n\n"+
			"Types: deb\nURIs: http://x\nSuites: noble\nComponents: multiverse\nEnabled: no\n\n"+
			"Types: deb-src\nURIs: http://x\nSuites: noble\nComponents: srcOnly2\n")
	write(filepath.Join(d, "b.list"), "deb http://y noble partner # trailing\n")

	oldList, oldDir := aptSourcesList, aptSourcesDir
	aptSourcesList, aptSourcesDir = list, d
	t.Cleanup(func() { aptSourcesList, aptSourcesDir = oldList, oldDir })

	got := aptComponents()
	for _, c := range []string{"main", "restricted", "universe", "partner"} {
		if !got[c] {
			t.Errorf("%q should be enabled: %v", c, got)
		}
	}
	for _, c := range []string{"multiverse", "srcOnly", "srcOnly2", "commented", "#"} {
		if got[c] {
			t.Errorf("%q must not be enabled: %v", c, got)
		}
	}
	if newTestProbe().ubuntuComponentsEnabled() {
		t.Error("multiverse is disabled, so the pair must not count as enabled")
	}
}
