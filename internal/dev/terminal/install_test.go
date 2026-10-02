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

package terminal

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oito2/perci/internal/distro"
	"github.com/oito2/perci/internal/executor"
)

func dryRun(buf *bytes.Buffer) *executor.Executor {
	return &executor.Executor{DryRun: true, UsePolicyKit: true, Stdout: buf, Stderr: buf}
}

// fakeBin writes executable scripts into a directory placed first on PATH.
func fakeBin(t *testing.T, scripts map[string]string) {
	t.Helper()
	dir := t.TempDir()
	for name, body := range scripts {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// Every terminal installs and uninstalls with at most one password prompt
// on both families.
func TestInstallUninstall_AtMostOnePrompt(t *testing.T) {
	for _, family := range []string{distro.Debian, distro.Fedora} {
		for _, term := range Catalogue {
			for action, run := range map[string]func(*executor.Executor, *bytes.Buffer) error{
				"install": func(exe *executor.Executor, buf *bytes.Buffer) error {
					return InstallOne(context.Background(), exe, buf, term, family)
				},
				"uninstall": func(exe *executor.Executor, buf *bytes.Buffer) error {
					return UninstallOne(context.Background(), exe, buf, term, family)
				},
			} {
				t.Run(family+"/"+term.Name+"/"+action, func(t *testing.T) {
					var buf bytes.Buffer
					if err := run(dryRun(&buf), &buf); err != nil {
						t.Fatal(err)
					}
					if n := strings.Count(buf.String(), "pkexec"); n > 1 {
						t.Errorf("pkexec called %d times:\n%s", n, buf.String())
					}
				})
			}
		}
	}
}

// Alacritty on Debian installs through a single privileged apt-get
// install.
func TestInstallAlacritty_DebianOneBatch(t *testing.T) {
	var buf bytes.Buffer
	if err := InstallOne(context.Background(), dryRun(&buf), &buf, findTerminal(t, "Alacritty"), distro.Debian); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if strings.Contains(out, "add-apt-repository") {
		t.Errorf("add-apt-repository must not run:\n%s", out)
	}
	if !strings.Contains(out, "'alacritty'") || strings.Count(out, "pkexec") != 1 {
		t.Errorf("want one batch installing alacritty:\n%s", out)
	}
}

// A failed download fails the install instead of reporting it as done.
func TestInstallKitty_DownloadFailureFails(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	fakeBin(t, map[string]string{"curl": "exit 6"})
	err := InstallOne(context.Background(), &executor.Executor{}, io.Discard, findTerminal(t, "Kitty"), distro.Debian)
	if err == nil {
		t.Fatal("expected an error")
	}
	if _, statErr := os.Lstat(filepath.Join(home, ".local", "bin", "kitty")); !os.IsNotExist(statErr) {
		t.Error("no symlink should be created after a failed download")
	}
}

func TestInstallKitty_LinksIntoLocalBin(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	fakeBin(t, map[string]string{"curl": "printf 'mkdir -p \"$HOME/.local/kitty.app/bin\"\\n'"})
	if err := InstallOne(context.Background(), &executor.Executor{}, io.Discard, findTerminal(t, "Kitty"), distro.Fedora); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"kitty", "kitten"} {
		target, err := os.Readlink(filepath.Join(home, ".local", "bin", name))
		if err != nil || target != filepath.Join(home, ".local", "kitty.app", "bin", name) {
			t.Errorf("%s link = %q (err %v)", name, target, err)
		}
	}

	if err := UninstallOne(context.Background(), &executor.Executor{}, io.Discard, findTerminal(t, "Kitty"), distro.Fedora); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{".local/kitty.app", ".local/bin/kitty", ".local/bin/kitten"} {
		if _, err := os.Lstat(filepath.Join(home, p)); !os.IsNotExist(err) {
			t.Errorf("%s should be removed", p)
		}
	}
}

func TestInstallUninstall_Commands(t *testing.T) {
	for _, c := range []struct {
		name, family, action, want string
	}{
		{"Alacritty", distro.Fedora, "install", "dnf install -y -- alacritty"},
		{"Alacritty", distro.Fedora, "uninstall", "dnf remove -y -- alacritty"},
		{"Alacritty", distro.Debian, "uninstall", "apt-get purge -y -- alacritty"},
		{"GNOME Console", distro.Debian, "install", "'gnome-console'"},
		{"GNOME Console", distro.Fedora, "uninstall", "dnf remove -y -- gnome-console"},
		{"Black Box", distro.Debian, "install", "flatpak [install --noninteractive"},
		{"Black Box", distro.Debian, "uninstall", "flatpak [uninstall --noninteractive"},
	} {
		t.Run(c.name+"/"+c.family+"/"+c.action, func(t *testing.T) {
			var buf bytes.Buffer
			term := findTerminal(t, c.name)
			var err error
			if c.action == "install" {
				err = InstallOne(context.Background(), dryRun(&buf), &buf, term, c.family)
			} else {
				err = UninstallOne(context.Background(), dryRun(&buf), &buf, term, c.family)
			}
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(buf.String(), c.want) {
				t.Errorf("missing %q:\n%s", c.want, buf.String())
			}
		})
	}
}

func TestUnsupportedFamilyAndTerminal(t *testing.T) {
	exe := dryRun(&bytes.Buffer{})
	for _, name := range []string{"Alacritty", "GNOME Console"} {
		term := findTerminal(t, name)
		if err := InstallOne(context.Background(), exe, io.Discard, term, distro.Unknown); err == nil {
			t.Errorf("install %s on unknown family: expected an error", name)
		}
		if err := UninstallOne(context.Background(), exe, io.Discard, term, distro.Unknown); err == nil {
			t.Errorf("uninstall %s on unknown family: expected an error", name)
		}
	}
	unknown := Terminal{Name: "X", Cmd: "x"}
	if err := InstallOne(context.Background(), exe, io.Discard, unknown, distro.Debian); err == nil {
		t.Error("unknown terminal install: expected an error")
	}
	if err := UninstallOne(context.Background(), exe, io.Discard, unknown, distro.Debian); err == nil {
		t.Error("unknown terminal uninstall: expected an error")
	}
}

func TestInstalledMap(t *testing.T) {
	// Only the fakes on PATH: the machine running the tests may have
	// real terminals installed.
	bin := t.TempDir()
	t.Setenv("PATH", bin)
	for name, body := range map[string]string{
		"kitty":   "",
		"flatpak": "printf 'org.other.App\\ncom.raggesilver.BlackBox\\n'",
	} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	got := InstalledMap(context.Background(), &executor.Executor{})
	for _, term := range Catalogue {
		want := term.Name == "Kitty" || term.Name == "Black Box"
		if got[term.Name] != want {
			t.Errorf("%s installed = %v, want %v", term.Name, got[term.Name], want)
		}
	}
}

// DryRun never touches the context-menu files of the real HOME.
func TestApply_DryRunSkipsContextMenuSync(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	var buf bytes.Buffer
	if err := Apply(context.Background(), dryRun(&buf), &buf, []string{"GNOME Console"}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(home, ".local", "share")); !os.IsNotExist(err) {
		t.Error("DryRun created context-menu files")
	}
}
