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

func TestValidateApp(t *testing.T) {
	php := config.AppContainer{Folder: "loja", URL: "loja.localhost", Type: config.AppTypePHP, PHPVersion: "8.3"}
	node := config.AppContainer{Folder: "spa", URL: "spa.localhost", Type: config.AppTypeNode, NodeVersion: "24", DevCommand: "npm run dev", DevPort: 5173}
	moodle := config.AppContainer{Folder: "mdle", URL: "mdle.localhost", Type: config.AppTypeMoodle, PHPVersion: "8.3", MoodleVersion: "5.1+"}
	with := func(a config.AppContainer, f func(*config.AppContainer)) config.AppContainer { f(&a); return a }

	cases := []struct {
		name string
		app  config.AppContainer
		ok   bool
	}{
		{"php", php, true},
		{"node", node, true},
		{"moodle", moodle, true},
		{"combo", with(node, func(a *config.AppContainer) { a.Type = config.AppTypePHPNode; a.PHPVersion = "8.3" }), true},
		{"memory limit", with(php, func(a *config.AppContainer) { a.PHPMemoryLimit = "1G" }), true},
		{"bad folder", with(php, func(a *config.AppContainer) { a.Folder = "../x" }), false},
		{"reserved folder", with(php, func(a *config.AppContainer) { a.Folder = "nginx" }), false},
		{"bad url", with(php, func(a *config.AppContainer) { a.URL = "a.b.localhost" }), false},
		{"unknown type", with(php, func(a *config.AppContainer) { a.Type = "rails" }), false},
		{"php version", with(php, func(a *config.AppContainer) { a.PHPVersion = "5.6" }), false},
		{"memory limit syntax", with(php, func(a *config.AppContainer) { a.PHPMemoryLimit = "lots" }), false},
		{"node version", with(node, func(a *config.AppContainer) { a.NodeVersion = "12" }), false},
		{"empty dev command", with(node, func(a *config.AppContainer) { a.DevCommand = "" }), false},
		{"dev port", with(node, func(a *config.AppContainer) { a.DevPort = 0 }), false},
		{"moodle version", with(moodle, func(a *config.AppContainer) { a.MoodleVersion = "2.x" }), false},
		{"moodle/php mismatch", with(moodle, func(a *config.AppContainer) { a.PHPVersion = "7.4" }), false},
	}
	for _, tc := range cases {
		if err := ValidateApp(tc.app); (err == nil) != tc.ok {
			t.Errorf("%s: ValidateApp = %v, want ok=%v", tc.name, err, tc.ok)
		}
	}
}

// fakeDocker puts a `docker` script first on PATH: `inspect` reports the
// Nginx container as status, `exec ... nginx -t` fails when testFails,
// and every call is appended to the returned log file.
func fakeDocker(t *testing.T, status string, testFails bool) string {
	t.Helper()
	bin := t.TempDir()
	log := filepath.Join(bin, "calls.log")
	fail := "0"
	if testFails {
		fail = "1"
	}
	script := `#!/bin/sh
printf '%s\n' "$*" >> '` + log + `'
case "$1" in
inspect) printf '%s\n' '` + status + `' ;;
exec) case "$*" in *"nginx -t"*) if [ ` + fail + ` = 1 ]; then echo "emerg: bad directive" >&2; exit 1; fi ;; esac ;;
esac
`
	if err := os.WriteFile(filepath.Join(bin, "docker"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return log
}

func nginxConfPath(t *testing.T) string {
	t.Helper()
	dir, err := NginxConfDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(dir, "default.conf")
}

var reloadApps = []config.AppContainer{{Folder: "loja", URL: "loja.localhost", Type: config.AppTypePHP, PHPVersion: "8.3"}}

// A config failing `nginx -t` is rolled back to the previous file and
// nginx is never reloaded with it.
func TestReloadNginxConfig_RollsBackInvalidConfig(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	log := fakeDocker(t, "running", true)
	path := nginxConfPath(t)
	if err := os.WriteFile(path, []byte("OLD"), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	err := ReloadNginxConfig(context.Background(), executor.New(&buf, &buf), &buf, reloadApps)
	if err == nil || !strings.Contains(err.Error(), "revertidas") || !strings.Contains(err.Error(), "bad directive") {
		t.Fatalf("err = %v, want a rollback with nginx's message", err)
	}
	if b, _ := os.ReadFile(path); string(b) != "OLD" {
		t.Errorf("default.conf = %q, want the previous content", b)
	}
	if calls, _ := os.ReadFile(log); strings.Contains(string(calls), "-s reload") {
		t.Errorf("nginx reloaded with an invalid config:\n%s", calls)
	}
}

// Without a previous file there is nothing to roll back to: the broken
// config is removed, never left as the next rollback's baseline.
func TestReloadNginxConfig_RemovesInvalidFirstConfig(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	fakeDocker(t, "running", true)
	path := nginxConfPath(t)

	var buf bytes.Buffer
	if err := ReloadNginxConfig(context.Background(), executor.New(&buf, &buf), &buf, reloadApps); err == nil {
		t.Fatal("expected an error")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("the invalid config was left on disk")
	}
}

// A valid config is tested, then reloaded (no container restart).
func TestReloadNginxConfig_ValidConfigReloads(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	log := fakeDocker(t, "running", false)
	path := nginxConfPath(t)

	var buf bytes.Buffer
	if err := ReloadNginxConfig(context.Background(), executor.New(&buf, &buf), &buf, reloadApps); err != nil {
		t.Fatalf("ReloadNginxConfig: %v\n%s", err, buf.String())
	}
	calls, _ := os.ReadFile(log)
	testIdx, reloadIdx := strings.Index(string(calls), "nginx -t"), strings.Index(string(calls), "nginx -s reload")
	if testIdx < 0 || reloadIdx < testIdx {
		t.Errorf("want nginx -t then nginx -s reload:\n%s", calls)
	}
	if b, _ := os.ReadFile(path); !strings.Contains(string(b), "loja.localhost") {
		t.Errorf("default.conf missing the app:\n%s", b)
	}
}

// With the Nginx container stopped the file is only written; nothing runs
// inside the container.
func TestReloadNginxConfig_ContainerStopped(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	log := fakeDocker(t, "exited", false)
	path := nginxConfPath(t)

	var buf bytes.Buffer
	if err := ReloadNginxConfig(context.Background(), executor.New(&buf, &buf), &buf, reloadApps); err != nil {
		t.Fatal(err)
	}
	if calls, _ := os.ReadFile(log); strings.Contains(string(calls), "exec") {
		t.Errorf("nothing may run in a stopped container:\n%s", calls)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("default.conf not written: %v", err)
	}
}

// fakeMkcert adds a `mkcert` that writes the requested cert/key files.
func fakeMkcert(t *testing.T) {
	t.Helper()
	bin := t.TempDir()
	script := `#!/bin/sh
while [ $# -gt 0 ]; do
  case "$1" in
    -cert-file|-key-file) : > "$2"; shift 2 ;;
    *) shift ;;
  esac
done
`
	if err := os.WriteFile(filepath.Join(bin, "mkcert"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func exportedStack(workspace string) ExportedConfig {
	return ExportedConfig{
		WorkspacePath: workspace,
		Docker: config.DockerConfig{
			NginxCreated: true,
			Apps:         []config.AppContainer{{Name: "Loja", Folder: "loja", URL: "loja.localhost", Type: config.AppTypePHP, PHPVersion: "8.3"}},
		},
	}
}

// Import onto a machine with no workspace chosen: adopts the exported
// one, saves the stack and recreates Nginx and the app.
func TestImportConfig_AdoptsWorkspaceAndRecreates(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	log := fakeDocker(t, "running", false)
	fakeMkcert(t)
	ws := t.TempDir()

	var buf bytes.Buffer
	if err := ImportConfig(context.Background(), executor.New(&buf, &buf), nil, &buf, exportedStack(ws)); err != nil {
		t.Fatalf("ImportConfig: %v\n%s", err, buf.String())
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.WorkspacePath != ws || len(cfg.Docker.Apps) != 1 || cfg.Docker.Apps[0].Folder != "loja" {
		t.Errorf("config after import: workspace=%q apps=%+v", cfg.WorkspacePath, cfg.Docker.Apps)
	}
	calls, _ := os.ReadFile(log)
	for _, want := range []string{"run -d --name " + NginxContainerName, "run -d --name loja"} {
		if !strings.Contains(string(calls), want) {
			t.Errorf("missing docker %q:\n%s", want, calls)
		}
	}
	if _, err := os.Stat(AppHTMLDir(ws, "loja")); err != nil {
		t.Errorf("project folder not created under the adopted workspace: %v", err)
	}
}

// A different local workspace refuses the import before anything is saved
// or recreated.
func TestImportConfig_WorkspaceMismatchTouchesNothing(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	log := fakeDocker(t, "running", false)
	if err := config.Update(func(c *config.Config) error { c.WorkspacePath = "/srv/local-ws"; return nil }); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	err := ImportConfig(context.Background(), executor.New(&buf, &buf), nil, &buf, exportedStack("/srv/other-ws"))
	if !errors.Is(err, ErrWorkspaceMismatch) {
		t.Fatalf("err = %v, want ErrWorkspaceMismatch", err)
	}
	cfg, _ := config.Load()
	if len(cfg.Docker.Apps) != 0 {
		t.Errorf("stack saved despite the refusal: %+v", cfg.Docker.Apps)
	}
	if calls, _ := os.ReadFile(log); strings.Contains(string(calls), "run ") {
		t.Errorf("containers recreated despite the refusal:\n%s", calls)
	}
}

// An invalid file is refused before the config is even read.
func TestImportConfig_InvalidFileRefused(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	bad := exportedStack("relative/path")
	var buf bytes.Buffer
	if err := ImportConfig(context.Background(), executor.New(&buf, &buf), nil, &buf, bad); err == nil {
		t.Fatal("expected a validation error")
	}
}
