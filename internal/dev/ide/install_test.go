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

package ide

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oito2/perci/internal/distro"
	"github.com/oito2/perci/internal/executor"
)

// TestInstallVSCode/TestInstallVSCodium confirm that the assembled install
// script has the keyring, key URL, repository path/content, idempotency
// check and package name, without installing anything (DryRun captures the
// command instead of running it).
func TestInstallVSCode(t *testing.T) {
	t.Run("debian", func(t *testing.T) {
		out := dryRunScript(t, func(exe *executor.Executor, stdout *bytes.Buffer) error {
			return installVSCode(context.Background(), exe, stdout, distro.Debian)
		})
		for _, want := range []string{
			"/usr/share/keyrings/microsoft-archive-keyring.gpg",
			"https://packages.microsoft.com/keys/microsoft.asc",
			"gpg --batch --yes --dearmor",
			"/etc/apt/sources.list.d/vscode.list",
			"deb [arch=amd64,arm64 signed-by=/usr/share/keyrings/microsoft-archive-keyring.gpg] https://packages.microsoft.com/repos/code stable main",
			"apt-get install -y 'code'",
		} {
			if !strings.Contains(out, want) {
				t.Errorf("script do VS Code (Debian) não contém %q\nscript completo:\n%s", want, out)
			}
		}
	})

	t.Run("fedora", func(t *testing.T) {
		out := dryRunScript(t, func(exe *executor.Executor, stdout *bytes.Buffer) error {
			return installVSCode(context.Background(), exe, stdout, distro.Fedora)
		})
		for _, want := range []string{
			"rpm --import 'https://packages.microsoft.com/keys/microsoft.asc'",
			"/etc/yum.repos.d/vscode.repo",
			"[code]",
			"baseurl=https://packages.microsoft.com/yumrepos/vscode",
			"dnf install -y 'code'",
		} {
			if !strings.Contains(out, want) {
				t.Errorf("script do VS Code (Fedora) não contém %q\nscript completo:\n%s", want, out)
			}
		}
	})
}

func TestInstallVSCodium(t *testing.T) {
	t.Run("debian", func(t *testing.T) {
		out := dryRunScript(t, func(exe *executor.Executor, stdout *bytes.Buffer) error {
			return installVSCodium(context.Background(), exe, stdout, distro.Debian)
		})
		for _, want := range []string{
			"/usr/share/keyrings/vscodium-archive-keyring.gpg",
			"https://gitlab.com/paulcarroty/vscodium-deb-rpm-repo/raw/master/pub.gpg",
			"gpg --batch --yes --dearmor",
			"/etc/apt/sources.list.d/vscodium.sources",
			"Types: deb",
			"URIs: https://download.vscodium.com/debs",
			"apt-get install -y 'codium'",
		} {
			if !strings.Contains(out, want) {
				t.Errorf("script do VSCodium (Debian) não contém %q\nscript completo:\n%s", want, out)
			}
		}
	})

	t.Run("fedora", func(t *testing.T) {
		out := dryRunScript(t, func(exe *executor.Executor, stdout *bytes.Buffer) error {
			return installVSCodium(context.Background(), exe, stdout, distro.Fedora)
		})
		for _, want := range []string{
			"rpm --import 'https://gitlab.com/paulcarroty/vscodium-deb-rpm-repo/raw/master/pub.gpg'",
			"/etc/yum.repos.d/vscodium.repo",
			"[vscodium]",
			"baseurl=https://download.vscodium.com/rpms/",
			"dnf install -y 'codium'",
		} {
			if !strings.Contains(out, want) {
				t.Errorf("script do VSCodium (Fedora) não contém %q\nscript completo:\n%s", want, out)
			}
		}
	})
}

// dryRunScript runs fn with an Executor in DryRun (nothing actually runs)
// and returns the captured output — includes the command/args formatted
// via %v by Executor.Run itself.
func dryRunScript(t *testing.T, fn func(exe *executor.Executor, stdout *bytes.Buffer) error) string {
	t.Helper()
	var buf bytes.Buffer
	exe := executor.New(&buf, &buf)
	exe.DryRun = true
	if err := fn(exe, &buf); err != nil {
		t.Fatalf("DryRun não deveria falhar: %v", err)
	}
	return buf.String()
}

// Every IDE installs, updates and uninstalls on both families asking for
// the password at most once; an unknown one is an error.
func TestIDEs_EveryActionAtMostOnePrompt(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for _, family := range []string{distro.Debian, distro.Fedora} {
		for _, e := range Catalogue {
			for name, run := range map[string]func(*executor.Executor, *bytes.Buffer) error{
				"install": func(exe *executor.Executor, b *bytes.Buffer) error {
					return InstallOne(context.Background(), exe, b, e, family)
				},
				"update": func(exe *executor.Executor, b *bytes.Buffer) error {
					return UpdateOne(context.Background(), exe, b, e, family)
				},
				"uninstall": func(exe *executor.Executor, b *bytes.Buffer) error {
					return UninstallOne(context.Background(), exe, b, e, family)
				},
			} {
				var buf bytes.Buffer
				exe := &executor.Executor{DryRun: true, UsePolicyKit: true, Stdout: &buf, Stderr: &buf}
				if err := run(exe, &buf); err != nil {
					t.Errorf("%s %s %s: %v", family, e.Cmd, name, err)
				}
				if n := strings.Count(buf.String(), "pkexec"); n > 1 {
					t.Errorf("%s %s %s: %d prompts:\n%s", family, e.Cmd, name, n, buf.String())
				}
			}
		}
	}
	unknown := IDE{Name: "Vim", Cmd: "vim"}
	var buf bytes.Buffer
	exe := &executor.Executor{DryRun: true, Stdout: &buf, Stderr: &buf}
	if InstallOne(context.Background(), exe, &buf, unknown, distro.Debian) == nil || UninstallOne(context.Background(), exe, &buf, unknown, distro.Debian) == nil {
		t.Error("an unknown IDE must fail")
	}
	if aptDnfRemove(context.Background(), exe, "code", distro.Unknown, executor.Options{}) == nil {
		t.Error("an unknown family must fail")
	}
}

func TestUpdateOne_Messages(t *testing.T) {
	var buf bytes.Buffer
	exe := &executor.Executor{DryRun: true, Stdout: &buf, Stderr: &buf}
	if err := UpdateOne(context.Background(), exe, &buf, Catalogue[0], distro.Debian); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "Atualizando Zed Editor") || !strings.Contains(buf.String(), "Zed Editor atualizado") {
		t.Errorf("output:\n%s", buf.String())
	}
}

// Zed lives in the user's home: uninstalling removes it without privilege.
func TestUninstallZed_RemovesUserFiles(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	app := filepath.Join(home, ".local", "zed.app")
	bin := filepath.Join(home, ".local", "bin", "zed")
	for _, p := range []string{filepath.Join(app, "x"), bin} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, nil, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	var buf bytes.Buffer
	if err := UninstallOne(context.Background(), executor.New(&buf, &buf), &buf, Catalogue[0], distro.Debian); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{app, bin} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("%s still exists", p)
		}
	}
}

// InstalledMap reports what resolves on PATH, keyed by name.
func TestInstalledMap(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // no shell fallback: only LookPath counts
	exe := executor.New(nil, nil)
	exe.LookPath = func(name string) (string, error) {
		if name == "code" {
			return "/usr/bin/code", nil
		}
		return "", errors.New("not found")
	}
	got := InstalledMap(context.Background(), exe)
	if len(got) != 1 || !got["VS Code"] {
		t.Errorf("InstalledMap = %v", got)
	}
}
