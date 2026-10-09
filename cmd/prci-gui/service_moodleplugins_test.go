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
	"testing"

	"github.com/oito2/perci/internal/config"
)

// moodlePluginWorkspace sets HOME to a temporary folder with a workspace
// holding a Moodle app "mdle" (5.1+ layout) and returns an approved,
// empty development folder.
func moodlePluginWorkspace(t *testing.T) string {
	t.Helper()
	isolateActions(t)
	home, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	ws := filepath.Join(home, "ws")
	html := filepath.Join(ws, "localhost", "html", "mdle")
	for _, d := range []string{"lib", "public/local", "public/course/format"} {
		if err := os.MkdirAll(filepath.Join(html, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	components := `{"plugintypes": {"local": "public/local", "format": "public/course/format"}}`
	if err := os.WriteFile(filepath.Join(html, "lib", "components.json"), []byte(components), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := config.Update(func(c *config.Config) error {
		c.WorkspacePath = ws
		c.Docker.Apps = []config.AppContainer{{Name: "Moodle 5", Folder: "mdle", Type: config.AppTypeMoodle}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	dev := filepath.Join(ws, "oito2-hub", "mdle-example")
	if err := os.MkdirAll(dev, 0o755); err != nil {
		t.Fatal(err)
	}
	approved, _ := approvePath(dev, nil)
	return approved
}

func TestMoodlePlugins_InspectWithEmptyAndDeclaredFolder(t *testing.T) {
	dev := moodlePluginWorkspace(t)
	b, _ := newTestBase(t)
	svc := &DevToolsService{*b}

	types, err := svc.GetMoodlePluginTypes("mdle")
	if err != nil || !slices.Equal(types, []string{"format", "local"}) {
		t.Fatalf("GetMoodlePluginTypes = %v, %v", types, err)
	}

	// Empty development folder: type and name come from the form.
	check, err := svc.InspectMoodlePlugin(dev, "mdle", "", "")
	if err != nil || check.Locked || check.OK || check.Message == "" {
		t.Errorf("without type/name: %+v, %v", check, err)
	}
	check, err = svc.InspectMoodlePlugin(dev, "mdle", "local", "example")
	if err != nil || check.Locked || !check.OK || check.DestLabel != "mdle/public/local/example" {
		t.Errorf("empty folder: %+v, %v", check, err)
	}
	check, err = svc.InspectMoodlePlugin(dev, "mdle", "course_format", "example")
	if err != nil || check.OK || check.Message == "" {
		t.Errorf("unknown type: %+v, %v", check, err)
	}

	// version.php declares the component: the form values are replaced.
	if err := os.WriteFile(filepath.Join(dev, "version.php"), []byte(`<?php $plugin->component = 'format_grid';`), 0o644); err != nil {
		t.Fatal(err)
	}
	check, err = svc.InspectMoodlePlugin(dev, "mdle", "local", "example")
	if err != nil || !check.Locked || check.Type != "format" || check.Name != "grid" || !check.OK || check.DestLabel != "mdle/public/course/format/grid" {
		t.Errorf("declared component: %+v, %v", check, err)
	}

	screen, err := svc.GetMoodlePlugins()
	if err != nil || len(screen.Apps) != 1 || screen.Apps[0].Label != "Moodle 5 (mdle)" || len(screen.Rows) != 0 {
		t.Errorf("GetMoodlePlugins = %+v, %v", screen, err)
	}
}

func TestMoodlePlugins_RefuseNonMoodleApp(t *testing.T) {
	moodlePluginWorkspace(t)
	if _, err := (&DevToolsService{}).GetMoodlePluginTypes("nope"); err == nil {
		t.Error("listed types of an unknown app")
	}
}
