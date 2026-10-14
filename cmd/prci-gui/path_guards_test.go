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
	"slices"
	"strings"
	"testing"

	"github.com/oito2/perci/internal/config"
	"github.com/oito2/perci/internal/dev/agentskills"
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
	// An existing file: refused as a folder, and as a file not picked.
	unapprovedFile := filepath.Join(t.TempDir(), "not-picked.txt")
	if err := os.WriteFile(unapprovedFile, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	// An existing directory nobody picked or registered: valid in itself,
	// so only the approval check can refuse it.
	unapprovedDir := t.TempDir()

	type method func(b *serviceBase, path string) error
	// These only require an existing absolute directory.
	plainFolderMethods := map[string]method{
		"InspectMoodlePlugin": func(b *serviceBase, f string) error {
			_, err := (&DevToolsService{*b}).InspectMoodlePlugin(f, "mdle", "local", "x")
			return err
		},
		"AddMoodlePlugin": func(b *serviceBase, f string) error {
			return (&DevToolsService{*b}).AddMoodlePlugin(f, "mdle", "local", "x")
		},
	}
	// These also require the folder to be picked this session or already
	// registered.
	folderMethods := map[string]method{
		"GetRepoFolderState": func(b *serviceBase, f string) error {
			_, err := (&DevToolsService{*b}).GetRepoFolderState(f)
			return err
		},
		"GetAgentSkillsRows": func(b *serviceBase, f string) error {
			_, err := (&DevToolsService{*b}).GetAgentSkillsRows(false, f)
			return err
		},
		"GetMCPServerRows": func(b *serviceBase, f string) error {
			_, err := (&DevToolsService{*b}).GetMCPServerRows(false, f)
			return err
		},
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
		"ExportDockerConfig": func(b *serviceBase, p string) error {
			return (&DockerService{*b}).ExportDockerConfig(p, TransferSelection{Nginx: true})
		},
		"PreviewImportConfig": func(b *serviceBase, p string) error {
			_, err := (&DockerService{*b}).PreviewImportConfig(p)
			return err
		},
		"ImportDockerContainerConfig": func(b *serviceBase, p string) error {
			return (&DockerService{*b}).ImportDockerContainerConfig(p, TransferSelection{Nginx: true})
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
	for name, call := range plainFolderMethods {
		t.Run(name, func(t *testing.T) {
			for _, bad := range []string{"", "relative/dir", missing, unapprovedFile} {
				check(t, call, bad)
			}
		})
	}
	for name, call := range folderMethods {
		t.Run(name, func(t *testing.T) {
			for _, bad := range []string{"", "relative/dir", missing, unapprovedFile, unapprovedDir} {
				check(t, call, bad)
			}
		})
	}
	for name, call := range fileMethods {
		t.Run(name, func(t *testing.T) {
			// RunSelfUninstall treats "" as "no backup" — a valid choice.
			bads := []string{"relative.txt", missing, unapprovedFile}
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

// A folder is known when picked this session or already registered as a
// remembered repository folder or as the folder of a local SKILL/MCP
// record; global records and anything else do not count.
func TestRequireKnownFolder(t *testing.T) {
	isolateActions(t)
	unknown := t.TempDir()
	if err := requireKnownFolder(unknown); err == nil {
		t.Fatal("an existing directory nobody picked or registered must be refused")
	}

	picked, _ := approvePath(t.TempDir(), nil)
	if err := requireKnownFolder(picked); err != nil {
		t.Errorf("picked folder: %v", err)
	}

	repoDir, skillDir, mcpDir, globalSkillDir, globalMCPDir := t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir()
	if err := config.Update(func(c *config.Config) error {
		c.RepoFolders = []string{repoDir + "/"}
		c.AgentSkills = []config.AgentSkillInstall{
			{Slug: "a", Folder: skillDir},
			{Slug: "a", Global: true, Folder: globalSkillDir},
		}
		c.MCPServers = []config.MCPServerInstall{
			{Slug: "b", Folder: mcpDir},
			{Slug: "b", Global: true, Folder: globalMCPDir},
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for name, dir := range map[string]string{"repo folder": repoDir, "local skill": skillDir, "local mcp": mcpDir} {
		if err := requireKnownFolder(dir); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	for name, dir := range map[string]string{"global skill": globalSkillDir, "global mcp": globalMCPDir, "unknown": unknown} {
		if err := requireKnownFolder(dir); err == nil {
			t.Errorf("%s: must be refused", name)
		}
	}
	if err := requireKnownFolder(filepath.Join(picked, "missing")); err == nil {
		t.Error("a missing folder must be refused")
	}
	if err := requireKnownFolderUnlessGlobal(true, "relative"); err != nil {
		t.Errorf("the folder does not matter in Global scope: %v", err)
	}
}

// AddRepoFolder only remembers a folder picked in a dialog this session,
// compares cleaned paths and forgets by the cleaned path too.
func TestAddRepoFolder_RequiresPickedFolderAndCleansPaths(t *testing.T) {
	isolateActions(t)
	d := &DevToolsService{}
	existing := t.TempDir()
	if err := d.AddRepoFolder(existing); err == nil {
		t.Fatal("a folder that was not picked must be refused")
	}
	picked, _ := approvePath(t.TempDir(), nil)
	for _, spelling := range []string{picked, picked + "/", picked + "/./"} {
		if err := d.AddRepoFolder(spelling); err != nil {
			t.Fatal(err)
		}
	}
	if got := d.GetRepoFolders(); len(got) != 1 || got[0].Path != picked {
		t.Fatalf("folders = %+v, want the one cleaned path %s", got, picked)
	}
	if err := d.RemoveRepoFolder(picked + "/"); err != nil {
		t.Fatal(err)
	}
	if got := d.GetRepoFolders(); len(got) != 0 {
		t.Errorf("folders after remove = %+v", got)
	}
}

// The MCP parameter must come from a dialog this session or be the value
// already recorded for that very server and scope.
func TestRequireMCPParam(t *testing.T) {
	isolateActions(t)
	dir := t.TempDir()
	if err := requireMCPParam("filesystem", true, "", dir); err == nil {
		t.Fatal("an unpicked directory must be refused")
	}
	if err := requireMCPParam("sqlite", true, "", filepath.Join(dir, "db.sqlite")); err == nil {
		t.Fatal("an unpicked file must be refused")
	}
	if err := requireMCPParam("dart-flutter", true, "", "anything"); err != nil {
		t.Errorf("a server without a parameter: %v", err)
	}
	if err := requireMCPParam("filesystem", true, "", ""); err != nil {
		t.Errorf("an empty parameter is the install's own error: %v", err)
	}

	picked, _ := approvePath(t.TempDir(), nil)
	if err := requireMCPParam("filesystem", true, "", picked); err != nil {
		t.Errorf("picked: %v", err)
	}

	if err := recordMCPServerInstall("filesystem", true, "", dir, nil, true); err != nil {
		t.Fatal(err)
	}
	if err := requireMCPParam("filesystem", true, "", dir); err != nil {
		t.Errorf("the recorded parameter must pass: %v", err)
	}
	if err := requireMCPParam("filesystem", false, t.TempDir(), dir); err == nil {
		t.Error("the record of another scope must not authorize the parameter")
	}
	if err := requireMCPParam("sqlite", true, "", dir); err == nil {
		t.Error("the record of another server must not authorize the parameter")
	}

	b, _ := newTestBase(t)
	if err := (&DevToolsService{*b}).InstallMCPServer("filesystem", true, "", t.TempDir()); err == nil {
		t.Error("InstallMCPServer must refuse an unpicked parameter")
	}
	if err := (&DevToolsService{*b}).UpdateMCPServer("filesystem", true, "", t.TempDir()); err == nil {
		t.Error("UpdateMCPServer must refuse an unpicked parameter")
	}
}

// A record that names its agents reports "partial" when one is missing;
// a record from before the field never does.
func TestRows_PartialState(t *testing.T) {
	isolateActions(t)
	err := config.Update(func(c *config.Config) error {
		c.AgentSkills = []config.AgentSkillInstall{
			{Slug: "godot-ui", Global: true, Agents: []string{"claude-code", "codex"}},
			{Slug: "godot-master", Global: true, Agents: agentskills.TargetAgents()},
			{Slug: "daisyui", Global: true},
		}
		c.MCPServers = []config.MCPServerInstall{
			{Slug: "filesystem", Global: true, Param: "/p", Agents: []string{"claude", "antigravity"}},
			{Slug: "sqlite", Global: true, Param: "/d", Agents: []string{"claude", "codex", "antigravity"}},
			{Slug: "dart-flutter", Global: true},
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	d := &DevToolsService{}

	skills, err := d.GetAgentSkillsRows(true, "")
	if err != nil {
		t.Fatal(err)
	}
	bySlug := map[string]AgentSkillRow{}
	for _, r := range skills {
		bySlug[r.Slug] = r
	}
	if r := bySlug["godot-ui"]; !r.Installed || !r.Partial || !slices.Equal(r.Agents, []string{"Claude", "Codex"}) {
		t.Errorf("godot-ui = %+v", r)
	}
	if r := bySlug["godot-master"]; !r.Installed || r.Partial || len(r.Agents) != 4 {
		t.Errorf("godot-master = %+v", r)
	}
	if r := bySlug["daisyui"]; !r.Installed || r.Partial || len(r.Agents) != 0 {
		t.Errorf("an old record = %+v", r)
	}
	if r := bySlug["code-review"]; r.Installed || r.Partial {
		t.Errorf("not installed = %+v", r)
	}

	servers, err := d.GetMCPServerRows(true, "")
	if err != nil {
		t.Fatal(err)
	}
	mcp := map[string]MCPServerRow{}
	for _, r := range servers {
		mcp[r.Slug] = r
	}
	if r := mcp["filesystem"]; !r.Partial || !slices.Equal(r.Agents, []string{"Claude", "Antigravity"}) || r.Param != "/p" {
		t.Errorf("filesystem = %+v", r)
	}
	if r := mcp["sqlite"]; r.Partial {
		t.Errorf("sqlite = %+v", r)
	}
	if r := mcp["dart-flutter"]; !r.Installed || r.Partial {
		t.Errorf("an old record = %+v", r)
	}
}

// Installing records which agents accepted it, and a row built from that
// record is not partial.
func TestInstallRecordsTheAcceptingAgents(t *testing.T) {
	isolateActions(t)
	b, rec := newTestBase(t)
	d := &DevToolsService{*b}
	if err := d.InstallMCPServer("dart-flutter", true, "", ""); err != nil {
		t.Fatal(err)
	}
	if _, done := rec.wait(t); done["ok"] != true {
		t.Fatalf("done = %v", done)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.MCPServers) != 1 || !slices.Equal(cfg.MCPServers[0].Agents, []string{"claude", "codex"}) {
		t.Errorf("record = %+v", cfg.MCPServers)
	}

	b2, rec2 := newTestBase(t)
	if err := (&DevToolsService{*b2}).InstallAgentSkill("daisyui", true, ""); err != nil {
		t.Fatal(err)
	}
	if _, done := rec2.wait(t); done["ok"] != true {
		t.Fatalf("done = %v", done)
	}
	cfg, _ = config.Load()
	if len(cfg.AgentSkills) != 1 || !slices.Equal(cfg.AgentSkills[0].Agents, agentskills.TargetAgents()) {
		t.Errorf("record = %+v", cfg.AgentSkills)
	}
	rows, _ := d.GetAgentSkillsRows(true, "")
	for _, r := range rows {
		if r.Slug == "daisyui" && (!r.Installed || r.Partial) {
			t.Errorf("row = %+v", r)
		}
	}
}
