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
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/oito2/perci/internal/appstack"
	"github.com/oito2/perci/internal/config"
	"github.com/oito2/perci/internal/executor"
)

// fakeDockerOnPath puts a `docker` script first on PATH that answers
// `inspect` with status (a failure when empty) and records every call in the
// returned log file.
func fakeDockerOnPath(t *testing.T, status string) string {
	t.Helper()
	bin := t.TempDir()
	log := filepath.Join(bin, "calls.log")
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> '" + log + "'\n" +
		"if [ \"$1\" = inspect ]; then [ -n '" + status + "' ] || exit 1; printf '%s\\n' '" + status + "'; fi\nexit 0\n"
	if err := os.WriteFile(filepath.Join(bin, "docker"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	return log
}

// realDockerService is a DockerService whose executor really runs the
// (fake) docker on PATH instead of only printing the commands.
func realDockerService(t *testing.T) *DockerService {
	t.Helper()
	b, _ := newTestBase(t)
	b.exe = executor.New(nil, nil)
	return &DockerService{*b}
}

func registerApp(t *testing.T, app config.AppContainer) {
	t.Helper()
	if err := config.Update(func(c *config.Config) error {
		c.Docker.Apps = append(c.Docker.Apps, app)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// The quick actions run under the action lock: while another action holds
// it they are refused and nothing reaches docker.
func TestDockerQuickActions_UseTheActionLock(t *testing.T) {
	isolateActions(t)
	registerApp(t, config.AppContainer{Folder: "loja", URL: "loja.localhost", Type: config.AppTypePHP, PHPVersion: "8.3"})
	log := fakeDockerOnPath(t, "running")
	svc := realDockerService(t)

	runMu.Lock()
	errs := map[string]error{
		"Start":   svc.StartDockerContainer("loja"),
		"Stop":    svc.StopDockerContainer("loja"),
		"Restart": svc.RestartDockerContainer("loja"),
		"Remove":  svc.RemoveDockerContainer("app", "loja"),
	}
	runMu.Unlock()
	for name, err := range errs {
		if !errors.Is(err, errActionRunning) {
			t.Errorf("%s while an action runs = %v, want errActionRunning", name, err)
		}
	}
	if b, _ := os.ReadFile(log); len(b) != 0 {
		t.Errorf("docker ran while the action lock was held:\n%s", b)
	}

	if err := svc.StartDockerContainer("loja"); err != nil {
		t.Errorf("Start with the lock free: %v", err)
	}
	if b, _ := os.ReadFile(log); !strings.Contains(string(b), "start -- loja") {
		t.Errorf("docker start not called:\n%s", b)
	}
}

// Containers that are not Perci's are refused for the quick actions, the
// removal and the logs, before docker runs.
func TestDockerQuickActions_RefuseForeignContainers(t *testing.T) {
	isolateActions(t)
	log := fakeDockerOnPath(t, "running")
	svc := realDockerService(t)

	for name, err := range map[string]error{
		"Start":              svc.StartDockerContainer("postgres"),
		"Stop":               svc.StopDockerContainer("postgres"),
		"Restart":            svc.RestartDockerContainer("postgres"),
		"Remove":             svc.RemoveDockerContainer("app", "postgres"),
		"RemoveNginxAsApp":   svc.RemoveDockerContainer("app", appstack.NginxContainerName),
		"RemoveMariaDBAsApp": svc.RemoveDockerContainer("app", appstack.MariaDBContainerName),
	} {
		if err == nil {
			t.Errorf("%s of a foreign container was accepted", name)
		}
	}
	if _, err := svc.GetContainerLogsSnapshot("postgres"); err == nil {
		t.Error("the logs of a foreign container were returned")
	}
	if b, _ := os.ReadFile(log); len(b) != 0 {
		t.Errorf("docker ran for a foreign container:\n%s", b)
	}
	if _, err := svc.GetContainerLogsSnapshot(appstack.NginxContainerName); err != nil {
		t.Errorf("the logs of nginx must be allowed: %v", err)
	}
}

// A second Nginx or MariaDB is refused by the backend, not only by the
// form's select.
func TestCreateDockerContainer_RefusesSecondSingleton(t *testing.T) {
	for _, req := range []ContainerFormRequest{
		{Kind: kindNginx},
		{Kind: kindMariaDB, DBUser: "dev", DBPass: "pw"},
	} {
		isolateActions(t)
		log := fakeDockerOnPath(t, "running")
		b, rec := newTestBase(t)
		b.exe = executor.New(nil, nil)
		svc := &DockerService{*b}
		if err := svc.CreateDockerContainer(req); err != nil {
			t.Fatalf("%s: %v", req.Kind, err)
		}
		_, done := rec.wait(t)
		if done["ok"] != false || !strings.Contains(done["error"].(string), "já existe") {
			t.Errorf("%s: done = %v, want a refusal", req.Kind, done)
		}
		if b, _ := os.ReadFile(log); strings.Contains(string(b), "run ") {
			t.Errorf("%s: docker run reached:\n%s", req.Kind, b)
		}
	}
}

// The import confirmation lists every command the file's apps would run.
func TestPreviewImportConfig_ListsCommands(t *testing.T) {
	isolateActions(t)
	b, _ := newTestBase(t)
	svc := &DockerService{*b}

	exported := appstack.ExportedConfig{WorkspacePath: "/ws", Docker: config.DockerConfig{Apps: []config.AppContainer{
		{Folder: "spa", URL: "spa.localhost", Type: config.AppTypeNode, NodeVersion: "24", DevCommand: "npm run dev", DevPort: 5173},
		{Folder: "api", URL: "api.localhost", Type: config.AppTypePHPNode, PHPVersion: "8.3", NodeVersion: "24", DevCommand: "npm start", WorkerCommand: "php artisan queue:work", DevPort: 5174},
		{Folder: "site", URL: "site.localhost", Type: config.AppTypePHP, PHPVersion: "8.3"},
	}}}
	data, err := yaml.Marshal(exported)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "export.yaml")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.PreviewImportConfig(path); err == nil {
		t.Fatal("an unapproved path was read")
	}
	if _, err := approvePath(path, nil); err != nil {
		t.Fatal(err)
	}
	preview, err := svc.PreviewImportConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"spa: npm run dev", "api: npm start", "api (worker): php artisan queue:work"}
	if !slices.Equal(preview.Commands, want) {
		t.Errorf("Commands = %v, want %v", preview.Commands, want)
	}
	if len(preview.Apps) != 3 || preview.Apps[0].Folder != "spa" || preview.Apps[0].Exists {
		t.Errorf("Apps = %+v", preview.Apps)
	}

	empty, _ := yaml.Marshal(appstack.ExportedConfig{WorkspacePath: "/ws"})
	emptyPath := filepath.Join(t.TempDir(), "empty.yaml")
	if err := os.WriteFile(emptyPath, empty, 0o600); err != nil {
		t.Fatal(err)
	}
	_, _ = approvePath(emptyPath, nil)
	if p, err := svc.PreviewImportConfig(emptyPath); err != nil || p.Commands == nil || len(p.Commands) != 0 {
		t.Errorf("no commands must be an empty list, got %#v, %v", p.Commands, err)
	}
}

// Exportar lists every local item; Importar marks the apps that already
// exist here and the plugin mounts that can't be made on this machine.
func TestTransferItems_ExportAndImportLists(t *testing.T) {
	isolateActions(t)
	b, _ := newTestBase(t)
	svc := &DockerService{*b}
	ws := t.TempDir()
	local := config.AppContainer{Name: "Loja", Folder: "loja", URL: "loja.localhost", Type: config.AppTypePHP, PHPVersion: "8.3"}
	if err := config.Update(func(c *config.Config) error {
		c.WorkspacePath = ws
		c.Docker = config.DockerConfig{NginxCreated: true, Apps: []config.AppContainer{local}}
		c.MoodlePlugins = []config.MoodlePluginMount{{App: "mdle", Component: "local_x", DevPath: "/srv/dev/x", RelPath: "local/x", MegaIgnore: "/a/.megaignore", MegaIgnoreRule: "-dpG:x"}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	items, err := svc.GetExportItems()
	if err != nil {
		t.Fatal(err)
	}
	if !items.Nginx || items.MariaDB || len(items.Apps) != 1 || len(items.Plugins) != 1 || items.Plugins[0].Key != "mdle/local/x" {
		t.Errorf("export items = %+v", items)
	}

	exported := appstack.ExportedConfig{WorkspacePath: ws,
		Docker:        config.DockerConfig{Apps: []config.AppContainer{local, {Name: "Novo", Folder: "novo", URL: "novo.localhost", Type: config.AppTypePHP, PHPVersion: "8.3"}}},
		MoodlePlugins: []config.MoodlePluginMount{{App: "novo", Component: "local_y", DevPath: "/nonexistent/perci-dev", RelPath: "local/y"}},
	}
	data, _ := yaml.Marshal(exported)
	path := filepath.Join(t.TempDir(), "export.yaml")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	_, _ = approvePath(path, nil)
	preview, err := svc.PreviewImportConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if !preview.LocalNginx || preview.LocalMariaDB || !preview.Apps[0].Exists || preview.Apps[1].Exists {
		t.Errorf("preview = %+v", preview)
	}
	if p := preview.Plugins[0]; p.Mounted || p.Reason == "" {
		t.Errorf("a mount into a non-Moodle app was offered: %+v", p)
	}
}
