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
	"strings"
	"testing"

	"github.com/oito2/perci/internal/dev/agentskills"
	"github.com/oito2/perci/internal/dev/llm"
	"github.com/oito2/perci/internal/dev/mcpservers"
	"github.com/oito2/perci/internal/dev/prereqs"
	"github.com/oito2/perci/internal/dev/sdks"
	"github.com/oito2/perci/internal/dev/terminal"
	"github.com/oito2/perci/internal/system/apps"
	"github.com/oito2/perci/internal/system/templates"
)

// isolateActions gives every action a throwaway HOME and a PATH with
// nothing on it, so whatever the machine running the tests has installed
// reads as missing and no real command can run (the executor is DryRun:
// commands are printed, not executed).
func isolateActions(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("PATH", t.TempDir())
	t.Setenv("SHELL", "/bin/bash")
}

// Each button's bound method reaches the domain code it's meant to: run in
// DryRun, the action finishes (action-done ok) and its output carries the
// command that identifies that domain function.
func TestActions_ReachTheirDomainCode(t *testing.T) {
	cases := []struct {
		name string
		call func(b *serviceBase) error
		want []string
	}{
		{"RunTerminals", func(b *serviceBase) error {
			return (&DevSetupService{*b}).RunTerminals([]string{"Alacritty"})
		}, []string{"alacritty"}},
		{"ApplyStarship", func(b *serviceBase) error { return (&DevSetupService{*b}).ApplyStarship() },
			[]string{"starship.rs/install.sh"}},
		{"RemoveStarship", func(b *serviceBase) error { return (&DevSetupService{*b}).RemoveStarship() },
			[]string{"Starship removidas"}},
		// DryRun answers every shell-based "is it installed?" with yes, so
		// an empty selection removes them all.
		{"RunAIApps", func(b *serviceBase) error {
			return (&DevSetupService{*b}).RunAIApps(nil)
		}, []string{"uninstall", "@openai/codex"}},
		{"RunPrereqs", func(b *serviceBase) error {
			return (&DevSetupService{*b}).RunPrereqs([]string{"Git"})
		}, []string{"git"}},
		{"InstallIDE", func(b *serviceBase) error { return (&DevSetupService{*b}).InstallIDE("zed") },
			[]string{"zed.dev"}},
		{"UninstallAndroidStudio", func(b *serviceBase) error { return (&DevSetupService{*b}).UninstallAndroidStudio() },
			[]string{"/opt/android-studio"}},
		{"RunFonts", func(b *serviceBase) error {
			return (&LinuxService{*b}).RunFonts([]string{"Carlito"})
		}, []string{"carlito"}},
		{"RunFlatpakApps", func(b *serviceBase) error {
			return (&LinuxService{*b}).RunFlatpakApps([]string{"org.gnome.Loupe"})
		}, []string{"flatpak", "org.gnome.Loupe"}},
		{"RunSystemUpdate", func(b *serviceBase) error { return (&LinuxService{*b}).RunSystemUpdate(false, false) },
			[]string{"[dry-run] /usr/bin/sudo", "update"}},
		{"UninstallLinuxToys", func(b *serviceBase) error { return (&LinuxService{*b}).UninstallLinuxToys() },
			[]string{"linuxtoys"}},
		{"UninstallMegaSync", func(b *serviceBase) error { return (&LinuxService{*b}).UninstallMegaSync() },
			[]string{"megasync"}},
		{"ApplyGlobalGitIdentity", func(b *serviceBase) error {
			return (&DevToolsService{*b}).ApplyGlobalGitIdentity("Dev", "dev@example.com")
		}, []string{"config --global -- user.name Dev"}},
		{"InstallAgentSkill", func(b *serviceBase) error {
			return (&DevToolsService{*b}).InstallAgentSkill("code-review", true, "")
		}, []string{"skills@1.7.0", "add", "code-review", "--global"}},
		{"InstallMCPServer", func(b *serviceBase) error {
			return (&DevToolsService{*b}).InstallMCPServer("dart-flutter", true, "", "")
		}, []string{"claude [plugin install dart-flutter@dart-flutter"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			isolateActions(t)
			b, rec := newTestBase(t)
			if err := c.call(b); err != nil {
				t.Fatalf("refused: %v", err)
			}
			logs, done := rec.wait(t)
			if done["ok"] != true {
				t.Errorf("action failed: %v\n%s", done["error"], logs)
			}
			for _, w := range c.want {
				if !strings.Contains(logs, w) {
					t.Errorf("output missing %q:\n%s", w, logs)
				}
			}
		})
	}
}

// Settings saved straight from the Home screen: a known value round-trips
// through config.yaml, anything else falls back to the default instead of
// being stored as-is.
func TestHomeSettings_RoundTripAndFallback(t *testing.T) {
	isolateActions(t)
	b, _ := newTestBase(t)
	h := &HomeService{*b}

	themes := h.GetGUIThemes()
	logos := h.GetSidebarLogos()
	icons := h.GetAppIcons()
	if len(themes) == 0 || len(logos) == 0 || len(icons) == 0 {
		t.Fatal("empty option lists")
	}
	last := func(s []string) string { return s[len(s)-1] }

	if err := h.SetTheme(last(themes)); err != nil || h.GetTheme() != last(themes) {
		t.Errorf("theme = %q (err %v), want %q", h.GetTheme(), err, last(themes))
	}
	if err := h.SetSidebarLogo(last(logos)); err != nil || h.GetSidebarLogo() != last(logos) {
		t.Errorf("logo = %q (err %v), want %q", h.GetSidebarLogo(), err, last(logos))
	}
	if err := h.SetAppIcon(last(icons)); err != nil || h.GetAppIcon() != last(icons) {
		t.Errorf("icon = %q (err %v), want %q", h.GetAppIcon(), err, last(icons))
	}

	for _, set := range []func(string) error{h.SetTheme, h.SetSidebarLogo, h.SetAppIcon} {
		if err := set("../../etc/passwd"); err != nil {
			t.Fatal(err)
		}
	}
	if h.GetTheme() == "../../etc/passwd" || h.GetSidebarLogo() == "../../etc/passwd" || h.GetAppIcon() == "../../etc/passwd" {
		t.Error("an unknown value was stored as-is")
	}

	if err := h.SetFlatpakScope("user"); err != nil || h.GetSelfConfigInfo().FlatpakScope != "user" {
		t.Errorf("scope = %q (err %v), want user", h.GetSelfConfigInfo().FlatpakScope, err)
	}
	if err := h.SetFlatpakScope("anything"); err != nil || h.GetSelfConfigInfo().FlatpakScope != "system" {
		t.Errorf("unknown scope should fall back to system, got %q (err %v)", h.GetSelfConfigInfo().FlatpakScope, err)
	}
}

// Repositórios' folder cards: add (deduplicated), list, inspect, forget —
// forgetting never touches the folder on disk.
func TestRepoFolders_AddListStateRemove(t *testing.T) {
	isolateActions(t)
	b, _ := newTestBase(t)
	d := &DevToolsService{*b}
	dir := t.TempDir()

	for range 2 {
		if err := d.AddRepoFolder(dir); err != nil {
			t.Fatal(err)
		}
	}
	folders := d.GetRepoFolders()
	if len(folders) != 1 || folders[0].Path != dir {
		t.Fatalf("folders = %+v, want just %s", folders, dir)
	}
	state, err := d.GetRepoFolderState(dir)
	if err != nil || !state.IsEmpty || state.IsGitRepo {
		t.Errorf("state = %+v (err %v), want an empty non-repo folder", state, err)
	}
	if err := d.RemoveRepoFolder(dir); err != nil {
		t.Fatal(err)
	}
	if got := d.GetRepoFolders(); len(got) != 0 {
		t.Errorf("folders after remove = %+v", got)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("removing the card must not delete the folder: %v", err)
	}
}

func TestDockerHelpers(t *testing.T) {
	isolateActions(t)
	b, _ := newTestBase(t)
	s := &DockerService{*b}

	cat := s.GetDockerCreateCatalog()
	if len(cat.PHPVersions) == 0 || len(cat.NodeVersions) == 0 || len(cat.MoodleVersions) == 0 {
		t.Errorf("catalog missing versions: %+v", cat)
	}
	if cat.MariaDBReady {
		t.Error("MariaDB can't be ready without its credentials in config.yaml")
	}
	if p1, p2 := s.GenMariaDBPassword(), s.GenMariaDBPassword(); len(p1) < 12 || p1 == p2 {
		t.Errorf("passwords %q / %q: want long and different", p1, p2)
	}
	if v := s.GetPHPVersionsForMoodleVersion(cat.MoodleVersions[0]); len(v) == 0 {
		t.Errorf("no PHP versions for Moodle %s", cat.MoodleVersions[0])
	}
	if err := s.RemoveDockerContainer("unknown", ""); err == nil {
		t.Error("RemoveDockerContainer: unknown kind must fail")
	}
}

func TestRecreateDockerContainer_UnknownKindFailsTheAction(t *testing.T) {
	isolateActions(t)
	b, rec := newTestBase(t)
	if err := (&DockerService{*b}).RecreateDockerContainer("unknown", ""); err != nil {
		t.Fatal(err)
	}
	if _, done := rec.wait(t); done["ok"] != false {
		t.Errorf("action-done = %v, want a failure", done)
	}
}

// The screens' read-only getters answer without panicking and list one row
// per catalogue entry. GetSelfUpdateInfo is left out: it asks GitHub.
func TestGetters_ListTheirCatalogues(t *testing.T) {
	isolateActions(t)
	b, _ := newTestBase(t)
	home, linux := &HomeService{*b}, &LinuxService{*b}
	devSetup, devTools, docker := &DevSetupService{*b}, &DevToolsService{*b}, &DockerService{*b}

	lists := map[string]struct {
		got  int
		want int
	}{
		"terminals": {len(devSetup.GetTerminalsInfo()), len(terminal.Catalogue)},
		"ai apps":   {len(devSetup.GetAIAppsInfo()), len(llm.Catalogue)},
		"prereqs":   {len(devSetup.GetPrereqsInfo()), len(prereqs.Catalogue)},
		"sdks":      {len(devSetup.GetSDKsInfo()), len(sdks.Catalogue)},
		"flatpak":   {len(linux.GetFlatpakAppsInfo()), len(apps.Catalogue)},
		"templates": {len(linux.GetTemplatesInfo()), len(templates.Catalogue)},
		"skills":    {mustLen(devTools.GetAgentSkillsRows(true, "")), len(agentskills.Catalogue)},
		"mcp":       {mustLen(devTools.GetMCPServerRows(true, "")), len(mcpservers.Catalogue)},
	}
	for name, l := range lists {
		if l.got != l.want || l.got == 0 {
			t.Errorf("%s: %d rows, want %d", name, l.got, l.want)
		}
	}
	if len(linux.GetFontsInfo()) == 0 || len(devTools.GetAIContextItems(t.TempDir())) == 0 {
		t.Error("fonts / AI contexts: empty lists")
	}
	if home.GetAppVersion() == "" || len(home.GetCategories()) == 0 {
		t.Error("empty app version or sidebar categories")
	}
	// Single-app screens and the rest: just answer.
	_ = devSetup.GetIDEInfo("zed")
	_ = devSetup.GetAndroidStudioInfo()
	_ = devSetup.GetAntigravityIDEInfo()
	_ = linux.GetLinuxToysInfo()
	_ = linux.GetMegaSyncInfo()
	_ = linux.GetPostinstallProfile()
	_ = docker.GetContainerRows()
	_ = home.GetSelfConfigInfo()
}

func mustLen[T any](rows []T, err error) int {
	if err != nil {
		return -1
	}
	return len(rows)
}
