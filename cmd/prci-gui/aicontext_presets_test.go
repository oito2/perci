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
	"testing"

	managerai "github.com/oito2/perci/internal/manager/ai"
)

// The screen receives every project type with its models and hint.
func TestGetAIContextPresets(t *testing.T) {
	got := (&DevToolsService{}).GetAIContextPresets()
	want := managerai.Presets()
	if len(got) != len(want) || len(got) == 0 {
		t.Fatalf("got %d presets, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].Name != want[i].Name || len(got[i].Models) != len(want[i].Models) || got[i].Hint != want[i].Hint {
			t.Errorf("preset %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// The agents start all checked for a new folder, follow what the folder's
// context was generated for, and Aplicar needs at least one.
func TestAIContextAgents(t *testing.T) {
	isolateActions(t)
	b, rec := newTestBase(t)
	d := &DevToolsService{*b}
	folder, _ := approvePath(t.TempDir(), nil)

	for _, a := range d.GetAIContextAgents(folder) {
		if !a.Selected {
			t.Errorf("agent %s unchecked for a new folder", a.ID)
		}
	}
	if err := d.ApplyAIContext(folder, nil, nil); err == nil {
		t.Error("Aplicar without agents started")
	}
	if err := d.ApplyAIContext(folder, []string{"Linux Bash"}, []string{managerai.AgentCodex}); err != nil {
		t.Fatal(err)
	}
	if logs, done := rec.wait(t); done["ok"] != true {
		t.Fatalf("action-done = %v (logs %q)", done, logs)
	}
	for _, a := range d.GetAIContextAgents(folder) {
		if a.Selected != (a.ID == managerai.AgentCodex) {
			t.Errorf("agent %s selected = %v after applying for Codex only", a.ID, a.Selected)
		}
	}
}

// The screen gets each standard's description, base and state, the
// project type the folder suggests, and a preview that changes nothing.
func TestAIContextItemsAndPreview(t *testing.T) {
	isolateActions(t)
	b, rec := newTestBase(t)
	d := &DevToolsService{*b}
	folder, _ := approvePath(t.TempDir(), nil)
	if err := os.WriteFile(filepath.Join(folder, "project.godot"), []byte("config_version=5\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := d.GetAIContextSuggestedPreset(folder); got != "Jogo Godot (Android)" {
		t.Errorf("suggested preset = %q", got)
	}
	if got := d.GetAIContextSuggestedPreset(t.TempDir()); got != "" {
		t.Errorf("a folder not picked got a suggestion: %q", got)
	}

	changes, err := d.PreviewAIContext(folder, []string{"Linux Bash"}, []string{managerai.AgentCodex})
	if err != nil || len(changes) == 0 || changes[0].Action != "create" {
		t.Fatalf("PreviewAIContext = %+v, %v", changes, err)
	}
	if _, err := os.Stat(filepath.Join(folder, "AGENTS.md")); err == nil {
		t.Error("the preview wrote AGENTS.md")
	}
	if err := d.ApplyAIContext(folder, []string{"Linux Bash"}, []string{managerai.AgentCodex}); err != nil {
		t.Fatal(err)
	}
	if _, done := rec.wait(t); done["ok"] != true {
		t.Fatalf("action-done = %v", done)
	}
	for _, it := range d.GetAIContextItems(folder) {
		switch it.ID {
		case "Linux Bash":
			if it.Status != managerai.StatusApplied || it.Description == "" {
				t.Errorf("Linux Bash item = %+v", it)
			}
		case "Go: Wails v3":
			if it.Base != "Go" || it.Status != "" {
				t.Errorf("Wails item = %+v", it)
			}
		}
	}
	if changes, err := d.PreviewAIContext(folder, []string{"Linux Bash"}, []string{managerai.AgentCodex}); err != nil || len(changes) != 0 {
		t.Errorf("preview after the same Aplicar = %+v, %v", changes, err)
	}
}
