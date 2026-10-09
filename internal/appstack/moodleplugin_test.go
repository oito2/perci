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
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oito2/perci/internal/config"
	"github.com/oito2/perci/internal/executor"
)

// pluginStack sets up HOME, a workspace with a Moodle app "mdle" (its
// components.json maps types to typeDirs) and a PHP app "loja", saves the
// config, and returns the workspace (links resolved) and a development
// folder outside it.
func pluginStack(t *testing.T, typeDirs map[string]string) (workspace, dev string) {
	t.Helper()
	home, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	workspace = filepath.Join(home, "ws")
	html := AppHTMLDir(workspace, "mdle")
	if err := os.MkdirAll(filepath.Join(html, "lib"), 0o755); err != nil {
		t.Fatal(err)
	}
	var parts []string
	for k, v := range typeDirs {
		parts = append(parts, `"`+k+`": "`+v+`"`)
		if err := os.MkdirAll(filepath.Join(html, v), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	components := `{"plugintypes": {` + strings.Join(parts, ", ") + `}}`
	if err := os.WriteFile(filepath.Join(html, "lib", "components.json"), []byte(components), 0o644); err != nil {
		t.Fatal(err)
	}
	dev = filepath.Join(home, "dev", "mdle-example")
	if err := os.MkdirAll(dev, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := config.Update(func(c *config.Config) error {
		c.WorkspacePath = workspace
		c.Docker.Apps = []config.AppContainer{
			{Name: "Moodle", Folder: "mdle", Type: config.AppTypeMoodle},
			{Name: "Loja", Folder: "loja", Type: config.AppTypePHP},
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return workspace, dev
}

func loadCfg(t *testing.T) *config.Config {
	t.Helper()
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestMountUnitName_MatchesSystemdEscape(t *testing.T) {
	paths := []string{
		"/srv/workspace/localhost/html/mdle-401/public/local/plugin_x",
		"/home/u/ws/localhost/html/a.b/.hidden/c:d",
		"/srv/workspace/localhost/html/mdle/local/ação",
		"/.dotted/x",
	}
	want := map[string]string{
		paths[0]: `srv-workspace-localhost-html-mdle\x2d401-public-local-plugin_x.mount`,
		paths[1]: `home-u-ws-localhost-html-a.b-.hidden-c:d.mount`,
		paths[3]: `\x2edotted-x.mount`,
	}
	for _, p := range paths {
		got := mountUnitName(p)
		if w, ok := want[p]; ok && got != w {
			t.Errorf("mountUnitName(%q) = %q, want %q", p, got, w)
		}
		if _, err := exec.LookPath("systemd-escape"); err == nil {
			out, err := exec.Command("systemd-escape", "--path", "--suffix=mount", p).Output()
			if err != nil {
				t.Fatal(err)
			}
			if sys := strings.TrimSpace(string(out)); got != sys {
				t.Errorf("mountUnitName(%q) = %q, systemd-escape says %q", p, got, sys)
			}
		}
	}
}

func TestMoodlePluginTypes_FollowsTheMoodleLayout(t *testing.T) {
	workspace, _ := pluginStack(t, map[string]string{"local": "public/local", "format": "public/course/format"})
	types, err := MoodlePluginTypes(workspace, "mdle")
	if err != nil {
		t.Fatal(err)
	}
	if types["local"] != "public/local" || types["format"] != "public/course/format" {
		t.Errorf("types = %v", types)
	}
	if _, err := MoodlePluginTypes(workspace, "loja"); err == nil {
		t.Error("a folder without components.json must be refused")
	}
}

func TestDevPluginComponent(t *testing.T) {
	dir := t.TempDir()
	if ty, n := DevPluginComponent(dir); ty != "" || n != "" {
		t.Errorf("empty folder = %q %q", ty, n)
	}
	for _, line := range []string{
		`$plugin->component = 'local_example_two';`,
		`$plugin->component="local_example_two";`,
	} {
		if err := os.WriteFile(filepath.Join(dir, "version.php"), []byte("<?php\n"+line+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if ty, n := DevPluginComponent(dir); ty != "local" || n != "example_two" {
			t.Errorf("%s → %q %q", line, ty, n)
		}
	}
}

func TestPlanMoodlePlugin(t *testing.T) {
	workspace, dev := pluginStack(t, map[string]string{"local": "local", "format": "course/format"})
	html := AppHTMLDir(workspace, "mdle")

	plan, err := PlanMoodlePlugin(loadCfg(t), dev, "mdle", "format", "example")
	if err != nil {
		t.Fatalf("PlanMoodlePlugin: %v", err)
	}
	if plan.RelPath != "course/format/example" || plan.Dest != filepath.Join(html, "course/format/example") || plan.Component != "format_example" {
		t.Errorf("plan = %+v", plan)
	}

	// An existing empty folder is accepted.
	if err := os.MkdirAll(filepath.Join(html, "local", "empty"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := PlanMoodlePlugin(loadCfg(t), dev, "mdle", "local", "empty"); err != nil {
		t.Errorf("empty destination refused: %v", err)
	}

	if err := os.MkdirAll(filepath.Join(html, "local", "full"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(html, "local", "full", "version.php"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	spaced := filepath.Join(filepath.Dir(dev), "with space")
	if err := os.MkdirAll(spaced, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := config.Update(func(c *config.Config) error {
		c.MoodlePlugins = []config.MoodlePluginMount{{App: "mdle", Component: "local_taken", DevPath: dev, RelPath: "local/taken"}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name, dev, app, ptype, pname string
	}{
		{"not a Moodle app", dev, "loja", "local", "x"},
		{"unknown app", dev, "nope", "local", "x"},
		{"unknown type", dev, "mdle", "course_format", "x"},
		{"bad name", dev, "mdle", "local", "Bad-Name"},
		{"destination with content", dev, "mdle", "local", "full"},
		{"already mounted", dev, "mdle", "local", "taken"},
		{"development folder inside the Moodle", filepath.Join(html, "local"), "mdle", "local", "x"},
		{"path with a space", spaced, "mdle", "local", "x"},
		{"relative development folder", "dev/mdle-example", "mdle", "local", "x"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := PlanMoodlePlugin(loadCfg(t), tc.dev, tc.app, tc.ptype, tc.pname); err == nil {
				t.Error("accepted")
			}
		})
	}

	t.Run("version.php declares another component", func(t *testing.T) {
		if err := os.WriteFile(filepath.Join(dev, "version.php"), []byte(`<?php $plugin->component = 'local_other';`), 0o644); err != nil {
			t.Fatal(err)
		}
		defer func() { _ = os.Remove(filepath.Join(dev, "version.php")) }()
		if _, err := PlanMoodlePlugin(loadCfg(t), dev, "mdle", "local", "example"); err == nil {
			t.Error("accepted a name different from version.php")
		}
		if _, err := PlanMoodlePlugin(loadCfg(t), dev, "mdle", "local", "other"); err != nil {
			t.Errorf("the declared component was refused: %v", err)
		}
	})
}

func TestAddAndRemoveMoodlePlugin_OnePasswordEach(t *testing.T) {
	workspace, dev := pluginStack(t, map[string]string{"local": "public/local"})
	dest := filepath.Join(AppHTMLDir(workspace, "mdle"), "public/local/example")
	var buf bytes.Buffer
	exe := &executor.Executor{DryRun: true, UsePolicyKit: true, Stdout: &buf, Stderr: &buf}

	if err := AddMoodlePlugin(context.Background(), exe, &buf, dev, "mdle", "local", "example"); err != nil {
		t.Fatalf("AddMoodlePlugin: %v", err)
	}
	out := buf.String()
	if n := strings.Count(out, "pkexec"); n != 1 {
		t.Errorf("add: %d escalations, want 1\n%s", n, out)
	}
	unit := mountUnitName(dest)
	for _, want := range []string{"'" + filepath.Join(systemdUnitDir, unit) + "'", "'daemon-reload'", "'enable' '--now' '--' '" + unit + "'"} {
		if !strings.Contains(out, want) {
			t.Errorf("add output lacks %s\n%s", want, out)
		}
	}
	if info, err := os.Stat(dest); err != nil || !info.IsDir() {
		t.Errorf("destination folder not created: %v", err)
	}
	cfg := loadCfg(t)
	if len(cfg.MoodlePlugins) != 1 || cfg.MoodlePlugins[0] != (config.MoodlePluginMount{App: "mdle", Component: "local_example", DevPath: dev, RelPath: "public/local/example"}) {
		t.Fatalf("records = %+v", cfg.MoodlePlugins)
	}
	if err := AddMoodlePlugin(context.Background(), exe, &buf, dev, "mdle", "local", "example"); err == nil {
		t.Error("the same plugin was mounted twice")
	}

	if err := RemoveMoodlePlugin(context.Background(), exe, &buf, "mdle", "public/local/nope"); err == nil {
		t.Error("removed an unknown record")
	}
	buf.Reset()
	if err := RemoveMoodlePlugin(context.Background(), exe, &buf, "mdle", "public/local/example"); err != nil {
		t.Fatalf("RemoveMoodlePlugin: %v", err)
	}
	out = buf.String()
	if n := strings.Count(out, "pkexec"); n != 1 {
		t.Errorf("remove: %d escalations, want 1\n%s", n, out)
	}
	for _, want := range []string{"'disable' '--now' '--' '" + unit + "'", "'rm' '-f' '--' '" + filepath.Join(systemdUnitDir, unit) + "'"} {
		if !strings.Contains(out, want) {
			t.Errorf("remove output lacks %s\n%s", want, out)
		}
	}
	if recs := loadCfg(t).MoodlePlugins; len(recs) != 0 {
		t.Errorf("record kept: %+v", recs)
	}
	if _, err := os.Stat(dev); err != nil {
		t.Errorf("development folder touched: %v", err)
	}
}

func TestMountUnitContent(t *testing.T) {
	got := mountUnitContent(MoodlePluginPlan{App: "mdle", Component: "local_x", DevPath: "/w/dev", Dest: "/w/html/mdle/local/x"})
	for _, want := range []string{"What=/w/dev\n", "Where=/w/html/mdle/local/x\n", "Options=bind\n", "Before=docker.service\n", "RequiresMountsFor=/w/dev\n", "WantedBy=multi-user.target\n"} {
		if !strings.Contains(got, want) {
			t.Errorf("unit lacks %q:\n%s", want, got)
		}
	}
}

// fakeStatDocker puts a `docker` script first on PATH: `inspect` prints
// status (or fails when status is empty) and `exec <c> stat` prints the
// id given for that container.
func fakeStatDocker(t *testing.T, status, appID, nginxID string) {
	t.Helper()
	bin := t.TempDir()
	script := `#!/bin/sh
case "$1" in
inspect) [ -n '` + status + `' ] || exit 1; printf '%s\n' '` + status + `' ;;
exec) case "$2" in nginx) printf '%s\n' '` + nginxID + `' ;; *) printf '%s\n' '` + appID + `' ;; esac ;;
esac
`
	if err := os.WriteFile(filepath.Join(bin, "docker"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestMoodlePluginStatuses(t *testing.T) {
	workspace, dev := pluginStack(t, map[string]string{"local": "local"})
	dest := filepath.Join(AppHTMLDir(workspace, "mdle"), "local", "example")
	rec := config.MoodlePluginMount{App: "mdle", Component: "local_example", DevPath: dev, RelPath: "local/example"}
	if err := config.Update(func(c *config.Config) error { c.MoodlePlugins = []config.MoodlePluginMount{rec}; return nil }); err != nil {
		t.Fatal(err)
	}
	devID, _ := dirID(dev)
	exe := executor.New(nil, nil)
	status := func() MoodlePluginState {
		t.Helper()
		states := MoodlePluginStatuses(context.Background(), exe, loadCfg(t))
		if len(states) != 1 {
			t.Fatalf("states = %+v", states)
		}
		return states[0]
	}

	// No mount on the host: the destination is not the development folder.
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	fakeStatDocker(t, "running", devID, devID)
	if got := status().Status; got != PluginInactive {
		t.Errorf("without a host mount: %s", got)
	}

	// A link stands in for the host bind mount: same device and inode.
	if err := os.Remove(dest); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(dev, dest); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		status, appID, nginxID, want string
	}{
		{"running", devID, devID, PluginApplied},
		{"running", "1:1", devID, PluginPending},
		{"running", devID, "1:1", PluginPending},
		{"exited", devID, devID, PluginStopped},
		{"", devID, devID, PluginNoApp},
	}
	for _, tc := range cases {
		fakeStatDocker(t, tc.status, tc.appID, tc.nginxID)
		if got := status().Status; got != tc.want {
			t.Errorf("container %q app %s nginx %s: %s, want %s", tc.status, tc.appID, tc.nginxID, got, tc.want)
		}
	}

	if err := os.WriteFile(filepath.Join(dev, "version.php"), []byte(`<?php $plugin->component = 'local_renamed';`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := status().DeclaredAs; got != "local_renamed" {
		t.Errorf("DeclaredAs = %q", got)
	}
}
