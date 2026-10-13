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
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oito2/perci/internal/executor"
)

// allProfiles lists every Profile this package declares — kept in sync by
// hand (no reflection-based discovery), used only by the structural tests
// below. Add new profiles here too.
func allProfiles() []Profile {
	return []Profile{
		mintCinnamonProfile, mintXFCEProfile,
		ubuntu2404Profile, ubuntu2604Profile,
		ubuntuBudgie2404Profile, ubuntuBudgie2604Profile,
		fedoraProfile, fedoraBudgieProfile,
		zorinCoreProfile, zorinLiteProfile,
		poposProfile,
	}
}

// TestProfiles_ActionIDsAreUnique catches a copy-paste ID typo (two
// actions sharing an ID) within a single Profile — DependsOn resolution
// and the GUI's per-item state both silently misbehave if IDs collide.
func TestProfiles_ActionIDsAreUnique(t *testing.T) {
	for _, p := range allProfiles() {
		t.Run(p.ID, func(t *testing.T) {
			seen := make(map[string]bool, len(p.Actions))
			for _, a := range p.Actions {
				if a.ID == "" {
					t.Errorf("action with empty ID (label %q)", a.Label)
					continue
				}
				if seen[a.ID] {
					t.Errorf("duplicate action ID %q", a.ID)
				}
				seen[a.ID] = true
			}
		})
	}
}

// TestProfiles_DependsOnResolves catches a DependsOn referencing an ID
// that doesn't exist in the same Profile.
func TestProfiles_DependsOnResolves(t *testing.T) {
	for _, p := range allProfiles() {
		t.Run(p.ID, func(t *testing.T) {
			ids := make(map[string]bool, len(p.Actions))
			for _, a := range p.Actions {
				ids[a.ID] = true
			}
			for _, a := range p.Actions {
				if a.DependsOn == "" {
					continue
				}
				if !ids[a.DependsOn] {
					t.Errorf("action %q depends on %q, which does not exist in this profile", a.ID, a.DependsOn)
				}
				if a.DependsOn == a.ID {
					t.Errorf("action %q depends on itself", a.ID)
				}
			}
		})
	}
}

// TestProfiles_ActionsHaveRunFunc catches an Action literal that forgot
// to set Run — it would panic (nil func call) only when that specific
// action got selected and executed, possibly long after this was written.
func TestProfiles_ActionsHaveRunFunc(t *testing.T) {
	for _, p := range allProfiles() {
		t.Run(p.ID, func(t *testing.T) {
			for _, a := range p.Actions {
				if a.Run == nil {
					t.Errorf("action %q has a nil Run func", a.ID)
				}
				if a.Label == "" {
					t.Errorf("action %q has an empty Label", a.ID)
				}
				if a.Description == "" {
					t.Errorf("action %q has an empty Description", a.ID)
				}
			}
		})
	}
}

// TestProfiles_HaveLabelAndID sanity-checks the Profile-level metadata
// the GUI shows directly ("Pós instalação do <Label>").
func TestProfiles_HaveLabelAndID(t *testing.T) {
	seen := make(map[string]bool)
	for _, p := range allProfiles() {
		if p.ID == "" {
			t.Errorf("profile with empty ID (label %q)", p.Label)
			continue
		}
		if p.Label == "" {
			t.Errorf("profile %q has an empty Label", p.ID)
		}
		if len(p.Actions) == 0 {
			t.Errorf("profile %q has no actions", p.ID)
		}
		if seen[p.ID] {
			t.Errorf("duplicate profile ID %q", p.ID)
		}
		seen[p.ID] = true
	}
}

func TestSwapfileScript_Syntax(t *testing.T) {
	if out, err := exec.Command("bash", "-n", "-c", swapfileScript).CombinedOutput(); err != nil {
		t.Fatalf("swapfileScript has a syntax error: %v\n%s", err, out)
	}
}

// Every action of every profile runs under DryRun without error and asks
// for the password at most once.
func TestProfiles_EveryActionAtMostOnePrompt(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // default config: Flatpak system scope
	for _, flatpakPresent := range []bool{true, false} {
		for _, p := range allProfiles() {
			for _, a := range p.Actions {
				var buf bytes.Buffer
				exe := &executor.Executor{DryRun: true, UsePolicyKit: true, Stdout: &buf, Stderr: &buf,
					LookPath: func(name string) (string, error) {
						if name == "flatpak" && !flatpakPresent {
							return "", errors.New("not found")
						}
						return "/usr/bin/" + name, nil
					}}
				if err := a.Run(context.Background(), exe, &buf); err != nil {
					t.Errorf("%s/%s: %v", p.ID, a.ID, err)
				}
				if n := strings.Count(buf.String(), "pkexec"); n > 1 {
					t.Errorf("%s/%s (flatpak present=%v): %d password prompts:\n%s", p.ID, a.ID, flatpakPresent, n, buf.String())
				}
			}
		}
	}
}

// The sysctl action writes 99-perci.conf and removes the old
// 99-lumina.conf in the same privileged script.
func TestConfigureSysctl_MigratesLegacyFile(t *testing.T) {
	var buf bytes.Buffer
	exe := &executor.Executor{DryRun: true, UsePolicyKit: true, Stdout: &buf, Stderr: &buf}
	if err := configureSysctl(context.Background(), exe, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, sysctlConfPath) || !strings.Contains(out, "rm -f -- '"+legacySysctlConfPath+"'") {
		t.Errorf("unexpected script:\n%s", out)
	}
}

// Each supported os-release ID / desktop / version combination gets its
// own profile; anything else gets none.
func TestDetectProfile(t *testing.T) {
	cases := []struct {
		id, de, ver string
		want        string
	}{
		{"linuxmint", "cinnamon", "22.3", "mint-cinnamon"},
		{"linuxmint", "xfce", "22.3", "mint-xfce"},
		{"ubuntu", "gnome", "24.04", "ubuntu-2404"},
		{"ubuntu", "gnome", "26.04", "ubuntu-2604"},
		{"ubuntu", "budgie", "24.04", "ubuntu-budgie-2404"},
		{"ubuntu", "budgie", "26.04", "ubuntu-budgie-2604"},
		{"ubuntu", "other", "26.04", "ubuntu-2604"},
		{"ubuntu", "budgie", "22.04", ""},
		{"fedora", "gnome", "44", "fedora"},
		{"fedora", "other", "44", "fedora"},
		{"fedora", "budgie", "44", "fedora-budgie"},
		{"zorin", "gnome", "18", "zorin-core"},
		{"zorin", "xfce", "18", "zorin-lite"},
		{"pop", "cosmic", "24.04", "popos-cosmic"},
		{"linuxmint", "budgie", "22.3", ""},
		{"arch", "gnome", "", ""},
	}
	for _, c := range cases {
		got := ""
		if p := detectProfile(c.id, c.de, c.ver); p != nil {
			got = p.ID
		}
		if got != c.want {
			t.Errorf("detectProfile(%q, %q, %q) = %q, want %q", c.id, c.de, c.ver, got, c.want)
		}
	}
}

// Ubuntu Budgie installs neither GNOME Tweaks nor GNOME Software, and its
// 26.04 Flatpak step leaves VLC to the distro package.
func TestUbuntuBudgieProfiles(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // default config: Flatpak system scope
	for _, p := range []Profile{ubuntuBudgie2404Profile, ubuntuBudgie2604Profile} {
		var buf bytes.Buffer
		exe := &executor.Executor{DryRun: true, UsePolicyKit: true, Stdout: &buf, Stderr: &buf}
		for _, a := range p.Actions {
			if err := a.Run(context.Background(), exe, &buf); err != nil {
				t.Fatalf("%s/%s: %v", p.ID, a.ID, err)
			}
		}
		out := buf.String()
		for _, pkg := range []string{"gnome-tweaks", "gnome-software"} {
			if strings.Contains(out, pkg) {
				t.Errorf("%s installs %s:\n%s", p.ID, pkg, out)
			}
		}
		if wantVLC := p.ID == "ubuntu-budgie-2404"; strings.Contains(out, "org.videolan.VLC") != wantVLC {
			t.Errorf("%s: VLC Flatpak installed = %v, want %v", p.ID, !wantVLC, wantVLC)
		}
	}
	if len(ubuntuBudgiePackages) != len(ubuntuPackages)-2 {
		t.Errorf("ubuntuBudgiePackages = %v", ubuntuBudgiePackages)
	}
}

// Every apt-get call of the Debian helpers is unattended: the shared
// options (including --force-confold) and DEBIAN_FRONTEND, in the same
// privileged batch where there is one.
func TestAptCalls_AreUnattended(t *testing.T) {
	var buf bytes.Buffer
	exe := &executor.Executor{DryRun: true, UsePolicyKit: true, Stdout: &buf, Stderr: &buf}
	ctx := context.Background()
	if err := aptStep(ctx, exe, &buf, "x", "update"); err != nil {
		t.Fatal(err)
	}
	if err := aptInstall(ctx, exe, &buf, "git"); err != nil {
		t.Fatal(err)
	}
	if err := acceptEulaAndAptInstall(ctx, exe, &buf, "ttf-mscorefonts-installer"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if n := strings.Count(out, "--force-confold"); n != 3 {
		t.Errorf("want --force-confold in 3 apt-get calls, got %d:\n%s", n, out)
	}
	if n := strings.Count(out, "DEBIAN_FRONTEND=noninteractive"); n != 3 {
		t.Errorf("want DEBIAN_FRONTEND in 3 apt-get calls, got %d:\n%s", n, out)
	}
}

// With only RPM Fusion free installed, enabling installs just nonfree.
func TestEnableRPMFusion_InstallsOnlyMissing(t *testing.T) {
	fakePath(t, map[string]string{
		"rpm": `case "$1" in -E) echo 44;; -q) [ "$3" = rpmfusion-free-release ];; esac`,
	})
	exe := executor.New(nil, nil)
	args, err := rpmFusionInstallArgs(context.Background(), exe)
	if err != nil {
		t.Fatal(err)
	}
	out := strings.Join(args, " ")
	if !strings.Contains(out, "rpmfusion-nonfree-release-44") || strings.Contains(out, "rpmfusion-free-release-44") {
		t.Errorf("only nonfree must be installed: %s", out)
	}

	fakePath(t, map[string]string{"rpm": `exit 0`})
	if args, err := rpmFusionInstallArgs(context.Background(), exe); err != nil || args != nil {
		t.Errorf("nothing to install when both are present: %v %v", args, err)
	}
}

// The Fedora codecs action needs the multimedia group's key packages too.
func TestFedoraCodecsApplied_NeedsGroupPackages(t *testing.T) {
	var codecs Action
	for _, a := range fedoraProfile.Actions {
		if a.ID == "install-codecs" {
			codecs = a
		}
	}
	// rpm succeeds only when every queried package is ffmpeg.
	fakePath(t, map[string]string{"rpm": `for a in "$@"; do case "$a" in -q|--|ffmpeg) ;; *) exit 1;; esac; done`})
	if codecs.Applied(newTestProbe()) {
		t.Error("ffmpeg alone must not count as codecs applied")
	}
	fakePath(t, map[string]string{"rpm": `exit 0`})
	if !codecs.Applied(newTestProbe()) {
		t.Error("all packages present must count as applied")
	}
}

// runSwapScript runs swapfileScript against a temporary /swapfile and fstab,
// with fake swapon/swapoff/fallocate/mkswap/findmnt, and returns the exit
// error, the combined output and the calls recorded by the fakes.
func runSwapScript(t *testing.T, prepare func(dir, swap, fstab string)) (out, calls string, err error) {
	t.Helper()
	dir := t.TempDir()
	swap, fstab, log := filepath.Join(dir, "swapfile"), filepath.Join(dir, "fstab"), filepath.Join(dir, "calls")
	if werr := os.WriteFile(fstab, []byte("# fstab\n"), 0o644); werr != nil {
		t.Fatal(werr)
	}
	prepare(dir, swap, fstab)
	bin := filepath.Join(dir, "bin")
	if merr := os.Mkdir(bin, 0o755); merr != nil {
		t.Fatal(merr)
	}
	fakes := map[string]string{
		"swapon":    `[ $# -eq 0 ] || echo "swapon $*" >> '` + log + `'; exit 0`,
		"swapoff":   `echo "swapoff $*" >> '` + log + `'; exit 0`,
		"fallocate": `touch "$3"`,
		"mkswap":    `exit 0`,
		"findmnt":   `echo ext4`,
	}
	for name, body := range fakes {
		if werr := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\n"+body+"\n"), 0o755); werr != nil {
			t.Fatal(werr)
		}
	}
	script := strings.NewReplacer("/swapfile", swap, "/etc/fstab", fstab).Replace(swapfileScript)
	cmd := exec.Command("bash", "-c", script)
	cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	b, err := cmd.CombinedOutput()
	data, _ := os.ReadFile(log)
	return string(b), string(data), err
}

func TestSwapfileScript_Behaviour(t *testing.T) {
	t.Run("success writes fstab", func(t *testing.T) {
		out, _, err := runSwapScript(t, func(string, string, string) {})
		if err != nil {
			t.Fatalf("%v\n%s", err, out)
		}
	})
	t.Run("symlink aborts", func(t *testing.T) {
		out, _, err := runSwapScript(t, func(dir, swap, _ string) {
			if lerr := os.Symlink(filepath.Join(dir, "elsewhere"), swap); lerr != nil {
				t.Fatal(lerr)
			}
		})
		if err == nil || !strings.Contains(out, "link simbólico") {
			t.Errorf("a symlink must abort with an error: %v\n%s", err, out)
		}
	})
	t.Run("fstab failure fails and undoes", func(t *testing.T) {
		var swapPath string
		out, calls, err := runSwapScript(t, func(dir, swap, fstab string) {
			swapPath = swap
			if rerr := os.Remove(fstab); rerr != nil {
				t.Fatal(rerr)
			}
			if merr := os.Mkdir(fstab, 0o755); merr != nil { // a directory: neither readable nor appendable
				t.Fatal(merr)
			}
		})
		if err == nil || !strings.Contains(out, "/etc/fstab") && !strings.Contains(out, "fstab") {
			t.Errorf("an fstab failure must fail the script: %v\n%s", err, out)
		}
		if _, serr := os.Lstat(swapPath); serr == nil {
			t.Error("the swapfile must be removed after an fstab failure")
		}
		if !strings.Contains(calls, "swapoff") {
			t.Errorf("the swap must be deactivated, calls: %q", calls)
		}
	})
}
