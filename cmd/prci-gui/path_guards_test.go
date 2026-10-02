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

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Every bound method that takes a folder or file path from the frontend
// (a webview) must refuse a bad one before doing anything with it: either
// synchronously (nothing emitted, the action lock left free) or as the
// very first step of its action (action-done with ok:false, no output
// before the refusal). Bad means: empty, relative, missing, or (for files
// read or written) not picked in a native dialog this session.
func TestBoundMethods_RefuseBadPathsBeforeRunning(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	missing := filepath.Join(t.TempDir(), "missing")
	unapproved := filepath.Join(t.TempDir(), "not-picked.txt")
	if err := os.WriteFile(unapproved, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	type method func(b *serviceBase, path string) error
	folderMethods := map[string]method{
		"ApplyAIContext": func(b *serviceBase, f string) error {
			return (&DevToolsService{*b}).ApplyAIContext(f, []string{"x"})
		},
		"InstallAgentSkill": func(b *serviceBase, f string) error {
			return (&DevToolsService{*b}).InstallAgentSkill("daisyui", false, f)
		},
		"RemoveAgentSkill": func(b *serviceBase, f string) error {
			return (&DevToolsService{*b}).RemoveAgentSkill("daisyui", false, f)
		},
		"UpdateAgentSkill": func(b *serviceBase, f string) error {
			return (&DevToolsService{*b}).UpdateAgentSkill("daisyui", false, f)
		},
		"InstallMCPServer": func(b *serviceBase, f string) error {
			return (&DevToolsService{*b}).InstallMCPServer("dart-flutter", false, f, "")
		},
		"RemoveMCPServer": func(b *serviceBase, f string) error {
			return (&DevToolsService{*b}).RemoveMCPServer("dart-flutter", false, f)
		},
		"UpdateMCPServer": func(b *serviceBase, f string) error {
			return (&DevToolsService{*b}).UpdateMCPServer("dart-flutter", false, f, "")
		},
		"AddRepoFolder": func(b *serviceBase, f string) error { return (&DevToolsService{*b}).AddRepoFolder(f) },
		"CloneRepo": func(b *serviceBase, f string) error {
			return (&DevToolsService{*b}).CloneRepo("https://example.com/r.git", f, "", "")
		},
		"InitRepoAt": func(b *serviceBase, f string) error { return (&DevToolsService{*b}).InitRepoAt(f, "", "") },
		"ApplyLocalGitIdentityAt": func(b *serviceBase, f string) error {
			return (&DevToolsService{*b}).ApplyLocalGitIdentityAt(f, "N", "e@x")
		},
		"GenerateGitignoreAt": func(b *serviceBase, f string) error { return (&DevToolsService{*b}).GenerateGitignoreAt(f) },
		"CreateConductAt":     func(b *serviceBase, f string) error { return (&DevToolsService{*b}).CreateConductAt(f, "e@x") },
	}
	fileMethods := map[string]method{
		"InstallAndroidStudio":  func(b *serviceBase, p string) error { return (&DevSetupService{*b}).InstallAndroidStudio(p) },
		"InstallAntigravityIDE": func(b *serviceBase, p string) error { return (&DevSetupService{*b}).InstallAntigravityIDE(p) },
		"UpdateAntigravityIDE":  func(b *serviceBase, p string) error { return (&DevSetupService{*b}).UpdateAntigravityIDE(p) },
		"SaveTextFile":          func(b *serviceBase, p string) error { return (&DockerService{*b}).SaveTextFile(p, "x") },
		"ExportDockerConfig":    func(b *serviceBase, p string) error { return (&DockerService{*b}).ExportDockerConfig(p) },
		"PreviewImportConfig": func(b *serviceBase, p string) error {
			_, err := (&DockerService{*b}).PreviewImportConfig(p)
			return err
		},
		"ImportDockerContainerConfig": func(b *serviceBase, p string) error {
			return (&DockerService{*b}).ImportDockerContainerConfig(p)
		},
		"BackupMariaDBContainer":  func(b *serviceBase, p string) error { return (&DockerService{*b}).BackupMariaDBContainer(p) },
		"RestoreMariaDBContainer": func(b *serviceBase, p string) error { return (&DockerService{*b}).RestoreMariaDBContainer(p) },
		"SetWorkspacePath":        func(b *serviceBase, p string) error { return (&HomeService{*b}).SetWorkspacePath(p) },
		"RunSelfUninstall": func(b *serviceBase, p string) error {
			return (&HomeService{*b}).RunSelfUninstall(false, false, p)
		},
	}

	check := func(t *testing.T, call method, bad string) {
		t.Helper()
		b, rec := newTestBase(t)
		if err := call(b, bad); err != nil {
			rec.mu.Lock()
			n := len(rec.events)
			rec.mu.Unlock()
			if n != 0 {
				t.Errorf("refused %q but emitted %d events", bad, n)
			}
		} else {
			logs, done := rec.wait(t)
			if done["ok"] != false {
				t.Errorf("accepted %q (action-done %v)", bad, done)
			}
			if logs != "" {
				t.Errorf("output before refusing %q: %q", bad, logs)
			}
		}
		if !runMu.TryLock() {
			t.Fatalf("action lock still held after %q", bad)
		}
		runMu.Unlock()
	}
	for name, call := range folderMethods {
		t.Run(name, func(t *testing.T) {
			for _, bad := range []string{"", "relative/dir", missing, unapproved} {
				check(t, call, bad)
			}
		})
	}
	for name, call := range fileMethods {
		t.Run(name, func(t *testing.T) {
			// RunSelfUninstall treats "" as "no backup" — a valid choice.
			bads := []string{"relative.txt", missing, unapproved}
			if name != "RunSelfUninstall" {
				bads = append(bads, "")
			}
			for _, bad := range bads {
				check(t, call, bad)
			}
		})
	}
}

// A folder picked in the native dialog is accepted and saved by
// SetWorkspacePath.
func TestSetWorkspacePath_AcceptsOnlyPickedFolder(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	h := &HomeService{}
	other := t.TempDir()
	if err := h.SetWorkspacePath(other); err == nil {
		t.Fatal("a folder not picked in the dialog must be refused")
	}
	picked, _ := approvePath(t.TempDir(), nil)
	if err := h.SetWorkspacePath(picked); err != nil {
		t.Fatalf("picked folder refused: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(home, ".perci", "config.yaml"))
	if err != nil || !strings.Contains(string(data), picked) {
		t.Errorf("config.yaml = %q (err %v), want the picked folder", data, err)
	}
}
