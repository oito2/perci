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

package appstack

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oito2/perci/internal/config"
	"github.com/oito2/perci/internal/executor"
)

var (
	shopApp = config.AppContainer{Name: "Loja", Folder: "loja", URL: "loja.localhost", Type: config.AppTypePHP, PHPVersion: "8.3"}
	blogApp = config.AppContainer{Name: "Blog", Folder: "blog", URL: "blog.localhost", Type: config.AppTypePHP, PHPVersion: "8.3", DBAccess: true}
)

func seedConfig(t *testing.T, mutate func(*config.Config)) {
	t.Helper()
	if err := config.Update(func(c *config.Config) error { mutate(c); return nil }); err != nil {
		t.Fatal(err)
	}
}

func TestExportSelected_OnlyTheSelectedItems(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	seedConfig(t, func(c *config.Config) {
		c.WorkspacePath = "/srv/ws"
		c.Docker = config.DockerConfig{
			NginxCreated: true,
			MariaDB:      config.MariaDBConfig{DBUser: "dev", DBPass: "pw"},
			Apps:         []config.AppContainer{shopApp, blogApp},
		}
		c.MoodlePlugins = []config.MoodlePluginMount{{
			App: "mdle", Component: "local_x", DevPath: "/srv/dev/x", RelPath: "local/x",
			MegaIgnore: "/srv/ws/localhost/html/.megaignore", MegaIgnoreRule: "-dpG:mdle/local/x",
		}}
	})
	path := filepath.Join(t.TempDir(), "export.yaml")

	if err := ExportSelected(path, TransferSelection{Apps: []string{"blog"}, Plugins: []string{"mdle/local/x"}}); err != nil {
		t.Fatalf("ExportSelected: %v", err)
	}
	got, err := LoadExportedConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.WorkspacePath != "/srv/ws" || got.Docker.NginxCreated || got.Docker.MariaDB.DBUser != "" {
		t.Errorf("unselected items exported: %+v", got)
	}
	if len(got.Docker.Apps) != 1 || got.Docker.Apps[0].Folder != "blog" {
		t.Errorf("apps = %+v, want only blog", got.Docker.Apps)
	}
	if len(got.MoodlePlugins) != 1 || got.MoodlePlugins[0].Component != "local_x" {
		t.Fatalf("plugins = %+v", got.MoodlePlugins)
	}
	if p := got.MoodlePlugins[0]; p.MegaIgnore != "" || p.MegaIgnoreRule != "" {
		t.Errorf("the MEGA exclusion of this machine was exported: %+v", p)
	}
	if data, _ := os.ReadFile(path); strings.Contains(string(data), "mega_ignore") {
		t.Errorf("mega_ignore keys in the file:\n%s", data)
	}

	if err := ExportSelected(path, TransferSelection{Apps: []string{"nope"}}); err == nil {
		t.Error("an unknown app was accepted")
	}
	if err := ExportSelected(path, TransferSelection{}); !errors.Is(err, ErrNothingToExport) {
		t.Errorf("empty selection: err = %v, want ErrNothingToExport", err)
	}
}

func TestExportConfig_IncludesPluginMounts(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	seedConfig(t, func(c *config.Config) {
		c.MoodlePlugins = []config.MoodlePluginMount{{App: "mdle", Component: "local_x", DevPath: "/srv/dev/x", RelPath: "local/x"}}
	})
	path := filepath.Join(t.TempDir(), "export.yaml")
	if err := ExportConfig(path); err != nil {
		t.Fatalf("a config with only plugin mounts must export: %v", err)
	}
	got, err := LoadExportedConfig(path)
	if err != nil || len(got.MoodlePlugins) != 1 {
		t.Fatalf("plugins = %+v, err %v", got.MoodlePlugins, err)
	}
}

func TestValidateExported_PluginMounts(t *testing.T) {
	good := config.MoodlePluginMount{App: "mdle", Component: "local_x", DevPath: "/srv/dev/x", RelPath: "local/x"}
	tests := []struct {
		name   string
		mutate func(*config.MoodlePluginMount)
	}{
		{"invalid app folder", func(m *config.MoodlePluginMount) { m.App = "../etc" }},
		{"invalid component", func(m *config.MoodlePluginMount) { m.Component = "local" }},
		{"destination outside the Moodle", func(m *config.MoodlePluginMount) { m.RelPath = "../../etc" }},
		{"relative development folder", func(m *config.MoodlePluginMount) { m.DevPath = "dev/x" }},
	}
	if err := validateExported(ExportedConfig{MoodlePlugins: []config.MoodlePluginMount{good}}); err != nil {
		t.Fatalf("valid mount refused: %v", err)
	}
	if err := validateExported(ExportedConfig{MoodlePlugins: []config.MoodlePluginMount{good, good}}); err == nil {
		t.Error("a duplicated mount was accepted")
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := good
			tt.mutate(&m)
			if err := validateExported(ExportedConfig{MoodlePlugins: []config.MoodlePluginMount{m}}); err == nil {
				t.Errorf("accepted %+v", m)
			}
		})
	}
}

// Importing part of a file merges it: the local app outside the file
// stays, the app with the same folder is replaced, and only the selected
// app's container is recreated.
func TestImportConfig_MergesTheSelection(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	log := fakeDocker(t, "running", false)
	fakeMkcert(t)
	ws := t.TempDir()
	local := config.AppContainer{Name: "Local", Folder: "local1", URL: "local1.localhost", Type: config.AppTypePHP, PHPVersion: "8.3"}
	oldShop := shopApp
	oldShop.PHPVersion = "8.2"
	seedConfig(t, func(c *config.Config) {
		c.WorkspacePath = ws
		c.Docker = config.DockerConfig{NginxCreated: true, Apps: []config.AppContainer{local, oldShop}}
	})
	file := ExportedConfig{WorkspacePath: ws, Docker: config.DockerConfig{
		NginxCreated: true, Apps: []config.AppContainer{shopApp, {Name: "Outro", Folder: "outro", URL: "outro.localhost", Type: config.AppTypePHP, PHPVersion: "8.3"}},
	}}

	var buf bytes.Buffer
	if err := ImportConfig(context.Background(), executor.New(&buf, &buf), nil, &buf, file, TransferSelection{Apps: []string{"loja"}}); err != nil {
		t.Fatalf("ImportConfig: %v\n%s", err, buf.String())
	}
	apps := loadCfg(t).Docker.Apps
	if len(apps) != 2 || apps[0].Folder != "local1" || apps[1].Folder != "loja" || apps[1].PHPVersion != "8.3" {
		t.Errorf("apps after import = %+v, want local1 kept and loja replaced", apps)
	}
	calls, _ := os.ReadFile(log)
	if !strings.Contains(string(calls), "run -d --name loja") {
		t.Errorf("loja not recreated:\n%s", calls)
	}
	for _, unwanted := range []string{"--name outro", "--name local1", "run -d --name " + NginxContainerName} {
		if strings.Contains(string(calls), unwanted) {
			t.Errorf("unselected container touched (%s):\n%s", unwanted, calls)
		}
	}
}

// An app with database access needs MariaDB, here or in the selection;
// otherwise nothing is saved.
func TestImportConfig_RefusesAnAppWithoutItsDatabase(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	log := fakeDocker(t, "running", false)
	ws := t.TempDir()
	seedConfig(t, func(c *config.Config) { c.WorkspacePath = ws })
	file := ExportedConfig{WorkspacePath: ws, Docker: config.DockerConfig{
		MariaDB: config.MariaDBConfig{DBUser: "dev", DBPass: "pw"}, Apps: []config.AppContainer{blogApp},
	}}

	var buf bytes.Buffer
	err := ImportConfig(context.Background(), executor.New(&buf, &buf), nil, &buf, file, TransferSelection{Apps: []string{"blog"}})
	if err == nil || !strings.Contains(err.Error(), "MariaDB") {
		t.Fatalf("err = %v, want a MariaDB dependency error", err)
	}
	if apps := loadCfg(t).Docker.Apps; len(apps) != 0 {
		t.Errorf("saved despite the refusal: %+v", apps)
	}
	if calls, _ := os.ReadFile(log); strings.Contains(string(calls), "run ") {
		t.Errorf("containers recreated despite the refusal:\n%s", calls)
	}
}

func TestImportConfig_UnknownSelectionRefused(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	var buf bytes.Buffer
	err := ImportConfig(context.Background(), executor.New(&buf, &buf), nil, &buf, exportedStack(t.TempDir()), TransferSelection{Apps: []string{"nope"}})
	if err == nil {
		t.Fatal("an app that is not in the file was accepted")
	}
	if err := ImportConfig(context.Background(), executor.New(&buf, &buf), nil, &buf, exportedStack(t.TempDir()), TransferSelection{}); err == nil {
		t.Fatal("an empty selection was accepted")
	}
}

// Plugin mounts are imported in one password prompt; one whose development
// folder doesn't exist here is skipped with a warning, and one already
// mounted is left alone.
func TestImportConfig_PluginMounts(t *testing.T) {
	workspace, dev := pluginStack(t, map[string]string{"local": "local"})
	fakeDocker(t, "running", false)
	writeFile(t, filepath.Join(filepath.Dir(AppHTMLDir(workspace, "mdle")), megaIgnoreName), userMegaIgnore, 0o644)
	other := filepath.Join(filepath.Dir(dev), "other")
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}
	seedConfig(t, func(c *config.Config) {
		c.MoodlePlugins = []config.MoodlePluginMount{{App: "mdle", Component: "local_old", DevPath: other, RelPath: "local/old"}}
	})
	file := ExportedConfig{WorkspacePath: workspace, MoodlePlugins: []config.MoodlePluginMount{
		{App: "mdle", Component: "local_example", DevPath: dev, RelPath: "local/example"},
		{App: "mdle", Component: "local_gone", DevPath: "/nonexistent/perci-dev", RelPath: "local/gone"},
		{App: "mdle", Component: "local_old", DevPath: other, RelPath: "local/old"},
	}}
	states, err := PluginImportStates(file)
	if err != nil {
		t.Fatal(err)
	}
	if states[0].Reason != "" || states[0].Mounted || !strings.Contains(states[1].Reason, "não existe") || !states[2].Mounted {
		t.Errorf("states = %+v", states)
	}

	var buf bytes.Buffer
	exe := &executor.Executor{DryRun: true, UsePolicyKit: true, Stdout: &buf, Stderr: &buf}
	err = ImportConfig(context.Background(), exe, nil, &buf, file, SelectAll(file))
	if !errors.Is(err, executor.ErrCompletedWithWarnings) {
		t.Fatalf("err = %v, want ErrCompletedWithWarnings\n%s", err, buf.String())
	}
	out := buf.String()
	if n := strings.Count(out, "pkexec"); n != 1 {
		t.Errorf("%d escalations, want 1\n%s", n, out)
	}
	if !strings.Contains(out, "local_gone") || !strings.Contains(out, "já está montado") {
		t.Errorf("skipped and existing mounts not reported:\n%s", out)
	}
	recs := loadCfg(t).MoodlePlugins
	if len(recs) != 2 || recs[1].Component != "local_example" || recs[1].MegaIgnoreRule != "-dpG:mdle/local/example" {
		t.Errorf("records = %+v", recs)
	}
}
