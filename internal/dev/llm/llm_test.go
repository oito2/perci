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

package llm

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oito2/perci/internal/executor"
)

func dryRun(buf *bytes.Buffer) *executor.Executor {
	return &executor.Executor{DryRun: true, UsePolicyKit: true, Stdout: buf, Stderr: buf}
}

// isolate gives the test its own HOME (bash as the shell) and a PATH with
// only dir and the system directories, so nothing installed on the
// machine running the tests is seen.
func isolate(t *testing.T) (home, bin string) {
	t.Helper()
	home = t.TempDir()
	bin = t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SHELL", "/bin/bash")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+"/usr/bin:/bin")
	return home, bin
}

func find(t *testing.T, cmd string) LLM {
	t.Helper()
	for _, l := range Catalogue {
		if l.Cmd == cmd {
			return l
		}
	}
	t.Fatalf("%q not in Catalogue", cmd)
	return LLM{}
}

// Every curl-installed CLI runs its official installer without a password
// prompt and puts ~/.local/bin on PATH; Codex goes through npm.
func TestInstallOne(t *testing.T) {
	for cmd, wantURL := range map[string]string{
		"claude":   "https://claude.ai/install.sh",
		"agy":      "https://antigravity.google/cli/install.sh",
		"opencode": "https://opencode.ai/install",
		"codex":    "@openai/codex",
	} {
		t.Run(cmd, func(t *testing.T) {
			home, _ := isolate(t)
			var buf bytes.Buffer
			if err := InstallOne(context.Background(), dryRun(&buf), &buf, find(t, cmd)); err != nil {
				t.Fatal(err)
			}
			out := buf.String()
			if !strings.Contains(out, wantURL) {
				t.Errorf("missing %q:\n%s", wantURL, out)
			}
			if strings.Contains(out, "pkexec") {
				t.Errorf("no password prompt expected:\n%s", out)
			}
			rc, _ := os.ReadFile(filepath.Join(home, ".bashrc"))
			hasPath := strings.Contains(string(rc), ".local/bin")
			if cmd != "codex" && !hasPath {
				t.Error("~/.local/bin not added to PATH")
			}
		})
	}
}

func TestUninstallOne_RemovesBinaryFromPath(t *testing.T) {
	_, bin := isolate(t)
	claude := filepath.Join(bin, "claude")
	if err := os.WriteFile(claude, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := UninstallOne(context.Background(), &executor.Executor{}, io.Discard, find(t, "claude")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(claude); !os.IsNotExist(err) {
		t.Errorf("binary should be removed (err=%v)", err)
	}
}

func TestUninstallOne_NotOnPathWarns(t *testing.T) {
	isolate(t)
	var out bytes.Buffer
	if err := UninstallOne(context.Background(), &executor.Executor{}, &out, find(t, "opencode")); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "não foi encontrado") {
		t.Errorf("expected a warning, got %q", out.String())
	}
}

func TestUninstallOne_Codex(t *testing.T) {
	isolate(t)
	var buf bytes.Buffer
	if err := UninstallOne(context.Background(), dryRun(&buf), &buf, find(t, "codex")); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "uninstall") || !strings.Contains(buf.String(), "@openai/codex") {
		t.Errorf("npm uninstall expected:\n%s", buf.String())
	}
}

func TestUnknownLLM(t *testing.T) {
	isolate(t)
	unknown := LLM{Name: "X", Cmd: "x"}
	if err := InstallOne(context.Background(), &executor.Executor{DryRun: true}, io.Discard, unknown); err == nil {
		t.Error("InstallOne: expected an error")
	}
	if err := UninstallOne(context.Background(), &executor.Executor{DryRun: true}, io.Discard, unknown); err == nil {
		t.Error("UninstallOne: expected an error")
	}
}

func TestInstalledMap(t *testing.T) {
	_, bin := isolate(t)
	if err := os.WriteFile(filepath.Join(bin, "codex"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	got := InstalledMap(context.Background(), &executor.Executor{})
	for _, l := range Catalogue {
		if want := l.Cmd == "codex"; got[l.Name] != want {
			t.Errorf("%s installed = %v, want %v", l.Name, got[l.Name], want)
		}
	}
}

func TestApply_DryRun(t *testing.T) {
	isolate(t)
	var buf bytes.Buffer
	err := Apply(context.Background(), dryRun(&buf), &buf, []string{"Claude Code CLI"}, []string{"Codex CLI"})
	if err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "claude.ai/install.sh") || !strings.Contains(out, "uninstall") {
		t.Errorf("both actions expected:\n%s", out)
	}
}

// Uninstalling the native Claude Code removes the ~/.local/bin/claude
// symlink and every downloaded version in ~/.local/share/claude. Settings
// in ~/.claude stay.
func TestUninstallOne_ClaudeNativeRemovesVersions(t *testing.T) {
	home, bin := isolate(t)
	dataDir := filepath.Join(home, ".local", "share", "claude")
	version := filepath.Join(dataDir, "versions", "2.1.286")
	settings := filepath.Join(home, ".claude", "settings.json")
	for _, p := range []string{version, settings} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	link := filepath.Join(bin, "claude")
	if err := os.Symlink(version, link); err != nil {
		t.Fatal(err)
	}
	if err := UninstallOne(context.Background(), &executor.Executor{}, io.Discard, find(t, "claude")); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{link, dataDir} {
		if _, err := os.Lstat(p); !os.IsNotExist(err) {
			t.Errorf("%s should be removed (err=%v)", p, err)
		}
	}
	if _, err := os.Stat(settings); err != nil {
		t.Errorf("settings must be kept: %v", err)
	}
}

// A claude that isn't the native install (e.g. npm's) only loses its
// binary — nothing under ~/.local/share/claude is touched.
func TestUninstallOne_ClaudeElsewhereKeepsDataDir(t *testing.T) {
	home, bin := isolate(t)
	dataDir := filepath.Join(home, ".local", "share", "claude")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := UninstallOne(context.Background(), &executor.Executor{}, io.Discard, find(t, "claude")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dataDir); err != nil {
		t.Errorf("data dir must be kept: %v", err)
	}
}

// Uninstalling OpenCode also removes what its official installer added:
// ~/.opencode/bin, ~/.opencode once empty, and only its own two rc lines.
func TestUninstallOne_OpenCodeLeftovers(t *testing.T) {
	home, _ := isolate(t)
	binDir := filepath.Join(home, ".opencode", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(binDir, "opencode"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	bashrc := filepath.Join(home, ".bashrc")
	fish := filepath.Join(home, ".config", "fish", "config.fish")
	if err := os.MkdirAll(filepath.Dir(fish), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bashrc, []byte("alias a=b\n\n# opencode\nexport PATH="+binDir+":$PATH\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fish, []byte("set -x A 1\n\n# opencode\nfish_add_path "+binDir+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	settings := filepath.Join(home, ".config", "opencode", "opencode.json")
	if err := os.MkdirAll(filepath.Dir(settings), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settings, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := UninstallOne(context.Background(), &executor.Executor{}, io.Discard, find(t, "opencode")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(home, ".opencode")); !os.IsNotExist(err) {
		t.Errorf("~/.opencode should be removed once empty (err=%v)", err)
	}
	for path, want := range map[string]string{bashrc: "alias a=b\n\n", fish: "set -x A 1\n\n"} {
		b, _ := os.ReadFile(path)
		if string(b) != want {
			t.Errorf("%s = %q, want %q", path, b, want)
		}
	}
	if _, err := os.Stat(settings); err != nil {
		t.Errorf("settings must be kept: %v", err)
	}
}

// Anything else the user keeps in ~/.opencode stays.
func TestUninstallOne_OpenCodeKeepsOtherContent(t *testing.T) {
	home, bin := isolate(t)
	other := filepath.Join(home, ".opencode", "plugins", "x.js")
	if err := os.MkdirAll(filepath.Dir(other), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(other, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "opencode"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := UninstallOne(context.Background(), &executor.Executor{}, io.Discard, find(t, "opencode")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(other); err != nil {
		t.Errorf("unrelated content removed: %v", err)
	}
}

// An OpenCode installed some other way (no ~/.opencode) uninstalls
// without a spurious warning about that directory.
func TestUninstallOne_OpenCodeWithoutInstallerDir(t *testing.T) {
	_, bin := isolate(t)
	if err := os.WriteFile(filepath.Join(bin, "opencode"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := UninstallOne(context.Background(), &executor.Executor{}, &out, find(t, "opencode")); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), ".opencode") {
		t.Errorf("unexpected message about ~/.opencode: %q", out.String())
	}
}
