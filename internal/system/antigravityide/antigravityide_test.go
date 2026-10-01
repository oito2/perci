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

package antigravityide

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

func TestFindBinary_SingleCandidate(t *testing.T) {
	workdir := t.TempDir()
	binPath := filepath.Join(workdir, "antigravity")
	if err := os.WriteFile(binPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	dir, name, err := findBinary(workdir)
	if err != nil {
		t.Fatalf("findBinary: %v", err)
	}
	if dir != workdir || name != "antigravity" {
		t.Errorf("findBinary() = (%q, %q), want (%q, %q)", dir, name, workdir, "antigravity")
	}
}

func TestFindBinary_NoCandidate(t *testing.T) {
	workdir := t.TempDir()
	if err := os.WriteFile(filepath.Join(workdir, "readme.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	if _, _, err := findBinary(workdir); err == nil {
		t.Error("expected an error when no 'antigravity*' candidate exists, but it succeeded")
	}
}

// TestFindBinary_PrefersShallowestWithSandboxSibling covers the non-trivial
// tie-break rule: with more than one "antigravity*" candidate, prefer the
// shallowest one that has a "chrome-sandbox" sibling (the real Electron app
// root), not just the first one found by WalkDir.
func TestFindBinary_PrefersShallowestWithSandboxSibling(t *testing.T) {
	workdir := t.TempDir()

	// Decoy: deeper candidate WITHOUT a chrome-sandbox sibling.
	decoyDir := filepath.Join(workdir, "resources", "app.asar.unpacked")
	if err := os.MkdirAll(decoyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(decoyDir, "antigravity-helper"), nil, 0o755); err != nil {
		t.Fatal(err)
	}

	// Real candidate: shallower, WITH a chrome-sandbox sibling.
	realDir := filepath.Join(workdir, "antigravity-linux-x64")
	if err := os.MkdirAll(realDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(realDir, "antigravity"), nil, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(realDir, "chrome-sandbox"), nil, 0o755); err != nil {
		t.Fatal(err)
	}

	dir, name, err := findBinary(workdir)
	if err != nil {
		t.Fatalf("findBinary: %v", err)
	}
	if dir != realDir || name != "antigravity" {
		t.Errorf("findBinary() = (%q, %q), want (%q, %q) — the candidate with a chrome-sandbox sibling", dir, name, realDir, "antigravity")
	}
}

// TestFindBinary_RejectsInvalidCharacters covers a security fix: a
// candidate whose filename contains characters outside validBinName must
// be discarded, not selected — even when it's the only
// "antigravity*"-prefixed entry.
func TestFindBinary_RejectsInvalidCharacters(t *testing.T) {
	workdir := t.TempDir()
	// A newline in the filename would let this name inject extra lines into
	// the .desktop file writeDesktopEntry generates — must never be picked.
	badName := "antigravity\nExec=evil"
	if err := os.WriteFile(filepath.Join(workdir, badName), nil, 0o755); err != nil {
		t.Fatal(err)
	}

	if _, _, err := findBinary(workdir); err == nil {
		t.Error("expected an error (no valid candidate) when the only name has characters outside validBinName, but it succeeded")
	}
}

// runInstallScript runs the real privileged installScript as the current
// user, with a no-op chown first on PATH (the only step that needs root).
func runInstallScript(t *testing.T, installDir, srcDir, sum string) error {
	t.Helper()
	fakeBin := t.TempDir()
	if err := os.WriteFile(filepath.Join(fakeBin, "chown"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("sh", "-c", installScript, "sh", installDir, srcDir, sum, sandboxPermWant)
	cmd.Env = append(os.Environ(), "PATH="+fakeBin+":"+os.Getenv("PATH"))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("script output: %s", out)
	}
	return err
}

func makeAppTree(t *testing.T, sandbox string) string {
	t.Helper()
	src := filepath.Join(t.TempDir(), "Antigravity")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "chrome-sandbox"), []byte(sandbox), 0o755); err != nil {
		t.Fatal(err)
	}
	// A setuid bit smuggled in by the archive must not survive the copy.
	extra := filepath.Join(src, "helper")
	if err := os.WriteFile(extra, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(extra, 0o4755); err != nil {
		t.Fatal(err)
	}
	return src
}

func TestInstallScript_SwapsInVerifiedTree(t *testing.T) {
	if _, err := exec.LookPath("sha256sum"); err != nil {
		t.Skip("sha256sum not available")
	}
	installDir := filepath.Join(t.TempDir(), "antigravity")
	src := makeAppTree(t, "sandbox-v2")
	sum, err := fileSHA256(filepath.Join(src, "chrome-sandbox"))
	if err != nil {
		t.Fatal(err)
	}
	if err := runInstallScript(t, installDir, src, sum); err != nil {
		t.Fatalf("install script failed: %v", err)
	}
	info, err := os.Stat(filepath.Join(installDir, "chrome-sandbox"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSetuid == 0 {
		t.Error("chrome-sandbox should be setuid after install")
	}
	helper, err := os.Stat(filepath.Join(installDir, "helper"))
	if err != nil {
		t.Fatal(err)
	}
	if helper.Mode()&os.ModeSetuid != 0 {
		t.Error("setuid bit from the archive must be stripped")
	}
	for _, leftover := range []string{installDir + ".new", installDir + ".old"} {
		if _, err := os.Stat(leftover); !os.IsNotExist(err) {
			t.Errorf("%s should not remain", leftover)
		}
	}
}

// A chrome-sandbox changed after the checksum was taken (the window while
// the password dialog is open) is refused and the previous install stays.
func TestInstallScript_RefusesTamperedSandbox(t *testing.T) {
	if _, err := exec.LookPath("sha256sum"); err != nil {
		t.Skip("sha256sum not available")
	}
	installDir := filepath.Join(t.TempDir(), "antigravity")
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(installDir, "previous")
	if err := os.WriteFile(marker, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	src := makeAppTree(t, "genuine")
	sum, err := fileSHA256(filepath.Join(src, "chrome-sandbox"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "chrome-sandbox"), []byte("swapped"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := runInstallScript(t, installDir, src, sum); err == nil {
		t.Fatal("tampered chrome-sandbox should be refused")
	}
	if _, err := os.Stat(marker); err != nil {
		t.Errorf("previous install must be kept: %v", err)
	}
	if _, err := os.Stat(installDir + ".new"); !os.IsNotExist(err) {
		t.Error("staging dir should be removed after a refused install")
	}
}

// The menu entry points at the installed binary and carries no directive
// beyond the fixed template.
func TestWriteDesktopEntry(t *testing.T) {
	home := t.TempDir()
	if err := writeDesktopEntry(home, "antigravity"); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(desktopFile(home))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, "Exec="+filepath.Join(installDir, "antigravity")+" %U\n") || !strings.HasPrefix(s, "[Desktop Entry]\n") {
		t.Errorf("unexpected entry:\n%s", s)
	}
	if strings.Count(s, "Exec=") != 1 {
		t.Errorf("exactly one Exec line expected:\n%s", s)
	}
}

func TestInstallIcon(t *testing.T) {
	home := t.TempDir()
	src := filepath.Join(t.TempDir(), "icon.png")
	if err := os.WriteFile(src, []byte("png"), 0o600); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	installIcon(&buf, home, src)
	got, err := os.ReadFile(iconFile(home))
	if err != nil || string(got) != "png" {
		t.Fatalf("icon = %q, %v", got, err)
	}

	// A missing source is a warning, not a crash.
	buf.Reset()
	installIcon(&buf, home, filepath.Join(home, "missing.png"))
	if !strings.Contains(buf.String(), "Falha ao ler") {
		t.Errorf("expected a warning, got %q", buf.String())
	}
}

func TestDoInstallOrUpdate_MissingTarball(t *testing.T) {
	var buf bytes.Buffer
	exe := &executor.Executor{DryRun: true, UsePolicyKit: true, Stdout: &buf, Stderr: &buf}
	err := Update(context.Background(), exe, &buf, filepath.Join(t.TempDir(), "none.tar.gz"))
	if err == nil || strings.Contains(buf.String(), "pkexec") {
		t.Errorf("err = %v, output:\n%s", err, buf.String())
	}
}

// Uninstall: one privileged rm of /opt/antigravity, then the user's menu
// entry and icon are removed without privilege.
func TestUninstall_DryRun(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	for _, p := range []string{desktopFile(home), iconFile(home)} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var buf bytes.Buffer
	exe := &executor.Executor{DryRun: true, UsePolicyKit: true, Stdout: &buf, Stderr: &buf}
	if err := Uninstall(context.Background(), exe, &buf); err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(buf.String(), "pkexec"); n != 1 {
		t.Errorf("want one pkexec call, got %d:\n%s", n, buf.String())
	}
	for _, p := range []string{desktopFile(home), iconFile(home)} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("%s not removed", p)
		}
	}
	if !strings.Contains(buf.String(), "update-desktop-database") {
		t.Errorf("menu database not refreshed:\n%s", buf.String())
	}
}

// verifySandboxPerms accepts only the setuid mode.
func TestVerifySandboxPerms(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root")
	}
	f := filepath.Join(t.TempDir(), "chrome-sandbox")
	if err := os.WriteFile(f, nil, 0o755); err != nil {
		t.Fatal(err)
	}
	exe := executor.New(nil, nil)
	if err := verifySandboxPerms(context.Background(), exe, f); err == nil {
		t.Error("0755 accepted")
	}
	if err := verifySandboxPerms(context.Background(), exe, f+"-missing"); err == nil {
		t.Error("missing file accepted")
	}
}
