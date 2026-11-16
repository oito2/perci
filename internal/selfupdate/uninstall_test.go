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

package selfupdate

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oito2/perci/internal/executor"
)

// withExecutable makes executablePath return a fake binary (content
// "old") in dir for the duration of the test.
func withExecutable(t *testing.T, dir string) string {
	t.Helper()
	exe := filepath.Join(dir, "prci")
	if err := os.WriteFile(exe, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	orig := executablePath
	executablePath = func() (string, error) { return exe, nil }
	t.Cleanup(func() { executablePath = orig })
	return exe
}

func dryRunExecutor(buf *bytes.Buffer) *executor.Executor {
	return &executor.Executor{DryRun: true, UsePolicyKit: true, Stdout: buf, Stderr: buf}
}

// Uninstall removes a user-owned binary and its 'perci' alias without
// escalating, removes ~/.perci only when asked, and reports ErrUninstalled.
func TestUninstall_UserOwnedBinary(t *testing.T) {
	for _, removeConfig := range []bool{false, true} {
		t.Run(fmt.Sprint("removeConfig=", removeConfig), func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			cfgDir := filepath.Join(home, ".perci")
			if err := os.MkdirAll(cfgDir, 0o700); err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			exe := withExecutable(t, dir)
			alias := filepath.Join(dir, "perci")
			if err := os.Symlink(exe, alias); err != nil {
				t.Fatal(err)
			}

			// Menu files: one the user can delete, and two in a read-only
			// directory that need root.
			userEntry := filepath.Join(t.TempDir(), "perci.desktop")
			sysDir := t.TempDir()
			sysIcons := []string{filepath.Join(sysDir, "a.png"), filepath.Join(sysDir, "b.png")}
			for _, p := range append([]string{userEntry}, sysIcons...) {
				if err := os.WriteFile(p, nil, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Chmod(sysDir, 0o555); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(sysDir, 0o755) })
			origMenu := uninstallMenuPaths
			uninstallMenuPaths = func() []string { return append([]string{userEntry}, sysIcons...) }
			t.Cleanup(func() { uninstallMenuPaths = origMenu })

			var buf bytes.Buffer
			err := Uninstall(context.Background(), dryRunExecutor(&buf), &buf, removeConfig)
			if !errors.Is(err, ErrUninstalled) {
				t.Fatalf("err = %v, want ErrUninstalled", err)
			}
			for _, p := range []string{exe, alias} {
				if _, statErr := os.Lstat(p); !os.IsNotExist(statErr) {
					t.Errorf("%s still exists", p)
				}
			}
			if _, statErr := os.Stat(cfgDir); os.IsNotExist(statErr) != removeConfig {
				t.Errorf("~/.perci removed = %v, want %v", os.IsNotExist(statErr), removeConfig)
			}
			if _, statErr := os.Stat(userEntry); !os.IsNotExist(statErr) {
				t.Error("a user-removable menu file must be removed directly")
			}
			// The root-owned files go in ONE privileged batch — never one
			// prompt per file (skipped as root, which needs no batch).
			if os.Geteuid() != 0 {
				out := buf.String()
				if n := strings.Count(out, "pkexec"); n != 1 || !strings.Contains(out, sysIcons[0]) || !strings.Contains(out, sysIcons[1]) {
					t.Errorf("want one pkexec call removing both icons, got %d:\n%s", n, out)
				}
			}
		})
	}
}

// Binary, alias and menu files that all need root are removed by ONE
// privileged batch.
func TestUninstall_AllPrivilegedInOneBatch(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	exe := withExecutable(t, dir)
	alias := filepath.Join(dir, "perci")
	if err := os.Symlink(exe, alias); err != nil {
		t.Fatal(err)
	}
	sysDir := t.TempDir()
	icon := filepath.Join(sysDir, "a.png")
	if err := os.WriteFile(icon, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{dir, sysDir} {
		if err := os.Chmod(d, 0o555); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(d, 0o755) })
	}
	origMenu := uninstallMenuPaths
	uninstallMenuPaths = func() []string { return []string{icon} }
	t.Cleanup(func() { uninstallMenuPaths = origMenu })

	var buf bytes.Buffer
	if err := Uninstall(context.Background(), dryRunExecutor(&buf), &buf, false); !errors.Is(err, ErrUninstalled) {
		t.Fatalf("err = %v, want ErrUninstalled", err)
	}
	out := buf.String()
	if n := strings.Count(out, "pkexec"); n != 1 {
		t.Errorf("want exactly one pkexec call, got %d:\n%s", n, out)
	}
	for _, p := range []string{exe, alias, icon} {
		if !strings.Contains(out, p) {
			t.Errorf("batch must remove %s:\n%s", p, out)
		}
	}
}

// Only a symlink to the binary counts as the 'perci' alias; anything else
// with that name is left alone.
func TestUninstall_KeepsForeignPerciEntry(t *testing.T) {
	makers := map[string]func(t *testing.T, alias string){
		"regular file": func(t *testing.T, alias string) {
			if err := os.WriteFile(alias, []byte("other tool"), 0o755); err != nil {
				t.Fatal(err)
			}
		},
		"directory": func(t *testing.T, alias string) {
			if err := os.Mkdir(alias, 0o755); err != nil {
				t.Fatal(err)
			}
		},
		"symlink elsewhere": func(t *testing.T, alias string) {
			if err := os.Symlink("/usr/bin/true", alias); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, mk := range makers {
		t.Run(name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			dir := t.TempDir()
			withExecutable(t, dir)
			alias := filepath.Join(dir, "perci")
			mk(t, alias)
			origMenu := uninstallMenuPaths
			uninstallMenuPaths = func() []string { return nil }
			t.Cleanup(func() { uninstallMenuPaths = origMenu })

			var buf bytes.Buffer
			if err := Uninstall(context.Background(), dryRunExecutor(&buf), &buf, false); !errors.Is(err, ErrUninstalled) {
				t.Fatalf("err = %v", err)
			}
			if _, err := os.Lstat(alias); err != nil {
				t.Errorf("foreign %s was removed: %v", name, err)
			}
		})
	}
}

func TestIsOurAlias_RelativeLink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "prci")
	alias := filepath.Join(dir, "perci")
	if err := os.Symlink("prci", alias); err != nil {
		t.Fatal(err)
	}
	if !isOurAlias(alias, target) {
		t.Error("relative symlink to the binary must count as the alias")
	}
}

// The alias counts when the link goes through a symlinked directory.
func TestIsOurAlias_SymlinkedDirectory(t *testing.T) {
	real := t.TempDir()
	link := filepath.Join(t.TempDir(), "bin")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(real, "perci")
	if err := os.Symlink(filepath.Join(link, "prci"), alias); err != nil {
		t.Fatal(err)
	}
	if !isOurAlias(alias, filepath.Join(real, "prci")) {
		t.Error("a link through a symlinked directory must count as the alias")
	}
}

// While the binary needs the privileged batch, a user-writable menu entry
// is not removed directly: canceling the prompt keeps both.
func TestUninstall_MenuWaitsForPrivilegedBinary(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	withExecutable(t, dir)
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	entry := filepath.Join(t.TempDir(), "perci.desktop")
	if err := os.WriteFile(entry, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	origMenu := uninstallMenuPaths
	uninstallMenuPaths = func() []string { return []string{entry} }
	t.Cleanup(func() { uninstallMenuPaths = origMenu })

	var buf bytes.Buffer
	_ = Uninstall(context.Background(), dryRunExecutor(&buf), &buf, false)
	if _, err := os.Stat(entry); err != nil {
		t.Errorf("the menu entry was removed directly: %v", err)
	}
	if !strings.Contains(buf.String(), entry) {
		t.Errorf("the menu entry must go in the privileged batch:\n%s", buf.String())
	}
}
