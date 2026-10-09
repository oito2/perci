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

// Bound methods for "Dev Tools :: Plugins Moodle" — a thin layer over the
// Moodle plugin mounts of internal/appstack.

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/oito2/perci/internal/appstack"
	"github.com/oito2/perci/internal/config"
)

// MoodleAppOption is one Moodle container offered by the screen.
type MoodleAppOption struct {
	Folder string `json:"folder"`
	Label  string `json:"label"`
}

// MoodlePluginRow is one mounted plugin. Status is one of the
// appstack.Plugin* values; DeclaredAs is set when the development folder's
// version.php declares another component.
type MoodlePluginRow struct {
	App        string `json:"app"`
	AppLabel   string `json:"appLabel"`
	Component  string `json:"component"`
	DevPath    string `json:"devPath"`
	DevLabel   string `json:"devLabel"`
	RelPath    string `json:"relPath"`
	DestLabel  string `json:"destLabel"`
	Status     string `json:"status"`
	DeclaredAs string `json:"declaredAs"`
}

// MoodlePluginsScreen is everything the screen lists.
type MoodlePluginsScreen struct {
	Apps []MoodleAppOption `json:"apps"`
	Rows []MoodlePluginRow `json:"rows"`
}

// MoodlePluginCheck is the result of checking the form: Locked means the
// type and name came from the development folder's version.php; OK means
// Criar can run and the plugin will appear at DestLabel; otherwise Message
// says why not.
type MoodlePluginCheck struct {
	Locked    bool   `json:"locked"`
	Type      string `json:"type"`
	Name      string `json:"name"`
	OK        bool   `json:"ok"`
	DestLabel string `json:"destLabel"`
	Message   string `json:"message"`
}

// moodleAppLabel is "Name (folder)", or the folder alone when they match.
func moodleAppLabel(a config.AppContainer) string {
	if a.Name == "" || a.Name == a.Folder {
		return a.Folder
	}
	return a.Name + " (" + a.Folder + ")"
}

// GetMoodlePlugins lists the Moodle containers and every mounted plugin
// with its current status.
func (t *DevToolsService) GetMoodlePlugins() (MoodlePluginsScreen, error) {
	cfg, err := config.Load()
	if err != nil {
		return MoodlePluginsScreen{}, err
	}
	screen := MoodlePluginsScreen{Apps: []MoodleAppOption{}, Rows: []MoodlePluginRow{}}
	labels := map[string]string{}
	for _, a := range appstack.MoodleApps(cfg.Docker.Apps) {
		labels[a.Folder] = moodleAppLabel(a)
		screen.Apps = append(screen.Apps, MoodleAppOption{Folder: a.Folder, Label: labels[a.Folder]})
	}
	workspace, _ := cfg.Workspace()
	for _, st := range appstack.MoodlePluginStatuses(context.Background(), t.exe, cfg) {
		appLabel := labels[st.App]
		if appLabel == "" {
			appLabel = st.App
		}
		screen.Rows = append(screen.Rows, MoodlePluginRow{
			App:        st.App,
			AppLabel:   appLabel,
			Component:  st.Component,
			DevPath:    st.DevPath,
			DevLabel:   relativeToWorkspace(workspace, st.DevPath),
			RelPath:    st.RelPath,
			DestLabel:  filepath.Join(st.App, st.RelPath),
			Status:     st.Status,
			DeclaredAs: st.DeclaredAs,
		})
	}
	return screen, nil
}

// relativeToWorkspace returns path relative to workspace, or path itself
// when it is outside it.
func relativeToWorkspace(workspace, path string) string {
	if workspace == "" {
		return path
	}
	if resolved, err := filepath.EvalSymlinks(workspace); err == nil {
		workspace = resolved
	}
	rel, err := filepath.Rel(workspace, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return path
	}
	return rel
}

// PickMoodlePluginDevFolder opens the native folder picker for the
// plugin's development folder.
func (t *DevToolsService) PickMoodlePluginDevFolder() (string, error) {
	return t.pickFolder("Selecionar a pasta de desenvolvimento do plugin")
}

// GetMoodlePluginTypes lists, sorted, the plugin types the Moodle in
// folder accepts.
func (t *DevToolsService) GetMoodlePluginTypes(folder string) ([]string, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	workspace, err := cfg.Workspace()
	if err != nil {
		return nil, err
	}
	if app, found := appstack.FindAppByFolder(cfg.Docker.Apps, folder); !found || app.Type != config.AppTypeMoodle {
		return nil, fmt.Errorf("%q não é um contêiner Moodle do Perci", folder)
	}
	types, err := appstack.MoodlePluginTypes(workspace, folder)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(types))
	for k := range types {
		out = append(out, k)
	}
	sort.Strings(out)
	return out, nil
}

// requireDevFolder refuses a development folder that isn't an existing
// absolute folder picked in the native dialog.
func requireDevFolder(dev string) error {
	if err := requireFolder(dev); err != nil {
		return err
	}
	return requireApprovedPath(dev)
}

// InspectMoodlePlugin checks the form. When dev/version.php declares a
// component, its type and name replace ptype and name.
func (t *DevToolsService) InspectMoodlePlugin(dev, folder, ptype, name string) (MoodlePluginCheck, error) {
	if err := requireDevFolder(dev); err != nil {
		return MoodlePluginCheck{}, err
	}
	check := MoodlePluginCheck{Type: ptype, Name: name}
	if declType, declName := appstack.DevPluginComponent(dev); declType != "" {
		check.Locked, check.Type, check.Name = true, declType, declName
	}
	switch {
	case folder == "":
		check.Message = "Selecione o Moodle."
		return check, nil
	case check.Type == "" || check.Name == "":
		check.Message = "Informe o tipo e o nome do plugin."
		return check, nil
	}
	cfg, err := config.Load()
	if err != nil {
		return check, err
	}
	plan, err := appstack.PlanMoodlePlugin(cfg, dev, folder, check.Type, check.Name)
	if err != nil {
		check.Message = capitalizeFirst(err.Error())
		return check, nil
	}
	check.OK = true
	check.DestLabel = filepath.Join(folder, plan.RelPath)
	return check, nil
}

// capitalizeFirst returns s with its first letter in upper case.
func capitalizeFirst(s string) string {
	r, size := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r)) + s[size:]
}

// AddMoodlePlugin mounts dev as the plugin ptype_name in the Moodle in
// folder (one password prompt).
func (t *DevToolsService) AddMoodlePlugin(dev, folder, ptype, name string) error {
	if err := requireDevFolder(dev); err != nil {
		return err
	}
	return t.runAction(func(stdout io.Writer) error {
		return appstack.AddMoodlePlugin(context.Background(), t.exe, stdout, dev, folder, ptype, name)
	})
}

// ApplyMoodlePlugins restarts the Moodle container in folder and Nginx so
// they see the current plugin mounts.
func (t *DevToolsService) ApplyMoodlePlugins(folder string) error {
	return t.runAction(func(stdout io.Writer) error {
		return appstack.ApplyMoodlePlugins(context.Background(), t.exe, stdout, folder)
	})
}

// RemoveMoodlePlugin unmounts the recorded plugin at relPath in the Moodle
// in folder (one password prompt) and restarts the containers. Only a
// recorded mount is accepted; the development folder is never touched.
func (t *DevToolsService) RemoveMoodlePlugin(folder, relPath string) error {
	return t.runAction(func(stdout io.Writer) error {
		return appstack.RemoveMoodlePlugin(context.Background(), t.exe, stdout, folder, relPath)
	})
}
