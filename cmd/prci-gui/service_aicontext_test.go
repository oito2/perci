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
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/oito2/perci/internal/config"
	managerai "github.com/oito2/perci/internal/manager/ai"
)

// runAIC runs one bound action on a fresh test base and returns its
// output and action-done payload; a synchronous refusal fails the test.
func runAIC(t *testing.T, call func(d *DevToolsService) error) (string, map[string]interface{}) {
	t.Helper()
	b, rec := newTestBase(t)
	if err := call(&DevToolsService{*b}); err != nil {
		t.Fatalf("refused: %v", err)
	}
	return rec.wait(t)
}

// refusedAIC asserts call is refused synchronously, emitting nothing.
func refusedAIC(t *testing.T, what string, call func(d *DevToolsService) error) {
	t.Helper()
	b, rec := newTestBase(t)
	if err := call(&DevToolsService{*b}); err == nil {
		t.Errorf("%s: accepted", what)
		rec.wait(t)
		return
	}
	rec.mu.Lock()
	defer rec.mu.Unlock()
	if len(rec.events) != 0 {
		t.Errorf("%s: refused but emitted %d events", what, len(rec.events))
	}
}

// aicRows returns GetAIContexts or fails the test.
func aicRows(t *testing.T) []AIContextRow {
	t.Helper()
	rows, err := (&DevToolsService{}).GetAIContexts()
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

// Aplicar registers the folder under its name, which must be set and
// unique, and the list shows it with its state.
func TestApplyAIContext_RegistersTheFolder(t *testing.T) {
	isolateActions(t)
	a, _ := approvePath(t.TempDir(), nil)
	b, _ := approvePath(t.TempDir(), nil)
	bash := []string{"Linux Bash"}
	codex := []string{managerai.AgentCodex}

	refusedAIC(t, "empty name", func(d *DevToolsService) error { return d.ApplyAIContext(a, " ", bash, codex) })
	if _, err := os.Stat(filepath.Join(a, "AGENTS.md")); err == nil {
		t.Error("refused Aplicar wrote AGENTS.md")
	}

	if _, done := runAIC(t, func(d *DevToolsService) error { return d.ApplyAIContext(a, " Projeto A ", bash, codex) }); done["ok"] != true {
		t.Fatalf("action-done = %v", done)
	}
	rows := aicRows(t)
	if len(rows) != 1 || rows[0].Name != "Projeto A" || rows[0].Path != a || rows[0].State != managerai.StatusApplied ||
		rows[0].Divergent || !slices.Equal(rows[0].Models, bash) || !slices.Equal(rows[0].Agents, codex) {
		t.Fatalf("rows = %+v", rows)
	}

	refusedAIC(t, "name in use", func(d *DevToolsService) error { return d.ApplyAIContext(b, "projeto a", bash, codex) })

	// Applying again to the same folder updates its record.
	if _, done := runAIC(t, func(d *DevToolsService) error {
		return d.ApplyAIContext(a, "Renomeado", nil, []string{managerai.AgentGemini})
	}); done["ok"] != true {
		t.Fatalf("action-done = %v", done)
	}
	rows = aicRows(t)
	if len(rows) != 1 || rows[0].Name != "Renomeado" || len(rows[0].Models) != 0 || rows[0].Agents[0] != managerai.AgentGemini {
		t.Errorf("rows after the second Aplicar = %+v", rows)
	}
}

// A folder with a context from before the list existed is offered for
// registration with what its files hold; a registered folder is accepted
// without being picked again.
func TestAIContextRecordAndRegister(t *testing.T) {
	isolateActions(t)
	d := &DevToolsService{}
	folder, _ := approvePath(t.TempDir(), nil)

	got, err := d.GetAIContextRecord(folder)
	if err != nil || got.Registered || got.HasContext || got.Models == nil {
		t.Fatalf("empty folder: %+v, %v", got, err)
	}
	if err := d.RegisterAIContext(folder, "X"); err == nil {
		t.Error("registered a folder without context")
	}

	if err := managerai.GenerateSharedFiles(folder, managerai.ModelsNamed([]string{"Linux Bash"}), []string{managerai.AgentClaude}, true, io.Discard); err != nil {
		t.Fatal(err)
	}
	got, err = d.GetAIContextRecord(folder)
	if err != nil || got.Registered || !got.HasContext || !slices.Equal(got.Models, []string{"Linux Bash"}) || !slices.Equal(got.Agents, []string{managerai.AgentClaude}) {
		t.Fatalf("unregistered context: %+v, %v", got, err)
	}
	if err := d.RegisterAIContext(folder, "Antigo"); err != nil {
		t.Fatal(err)
	}
	if got, err = d.GetAIContextRecord(folder); err != nil || !got.Registered || got.Name != "Antigo" {
		t.Errorf("after Registrar: %+v, %v", got, err)
	}

	// Registered in an earlier session: never picked in this one.
	earlier := t.TempDir()
	if err := managerai.RecordContext("Earlier", earlier, nil, []string{managerai.AgentCodex}); err != nil {
		t.Fatal(err)
	}
	if got, err := d.GetAIContextRecord(earlier); err != nil || got.Name != "Earlier" {
		t.Errorf("registered folder: %+v, %v", got, err)
	}
	if _, err := d.GetAIContextRecord(t.TempDir()); err == nil {
		t.Error("a folder neither picked nor registered was accepted")
	}
}

// Atualizar regenerates each folder from its record; a missing folder is
// pending in the preview and skipped with a warning.
func TestUpdateAIContexts(t *testing.T) {
	isolateActions(t)
	a, missing := t.TempDir(), filepath.Join(t.TempDir(), "gone")
	codex := []string{managerai.AgentCodex}
	if err := managerai.GenerateSharedFiles(a, managerai.ModelsNamed([]string{"Linux Bash"}), codex, true, io.Discard); err != nil {
		t.Fatal(err)
	}
	for name, p := range map[string]string{"A": a, "Gone": missing} {
		if err := managerai.RecordContext(name, p, []string{"Linux Bash"}, codex); err != nil {
			t.Fatal(err)
		}
	}
	std := filepath.Join(a, ".instructions", "BASH.md")
	want, err := os.ReadFile(std)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(std, []byte("edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if rows := aicRows(t); rows[0].State != managerai.StatusEdited || rows[1].State != managerai.StatusPending {
		t.Fatalf("rows = %+v", rows)
	}

	preview, err := (&DevToolsService{}).PreviewAIContextUpdate([]string{a, missing})
	if err != nil {
		t.Fatal(err)
	}
	if len(preview) != 2 || preview[0].Pending || len(preview[0].Changes) != 1 || preview[0].Changes[0].Path != filepath.Join(".instructions", "BASH.md") ||
		!preview[1].Pending || preview[1].Changes == nil {
		t.Fatalf("preview = %+v", preview)
	}
	if got, _ := os.ReadFile(std); string(got) != "edited\n" {
		t.Error("the preview changed the folder")
	}

	logs, done := runAIC(t, func(d *DevToolsService) error { return d.UpdateAIContexts([]string{a, missing}) })
	if done["ok"] != false || !strings.Contains(logs, "Pasta não encontrada") {
		t.Errorf("action-done = %v, logs %q", done, logs)
	}
	if got, _ := os.ReadFile(std); string(got) != string(want) {
		t.Error("the existing folder was not regenerated")
	}

	if _, done := runAIC(t, func(d *DevToolsService) error { return d.UpdateAIContexts([]string{a}) }); done["ok"] != true {
		t.Errorf("update of an existing folder: %v", done)
	}
}

// Remover takes the generated files out of the folder and the folder out
// of the list; a missing folder only leaves the list.
func TestRemoveAIContext(t *testing.T) {
	isolateActions(t)
	a, missing := t.TempDir(), filepath.Join(t.TempDir(), "gone")
	agents := []string{managerai.AgentClaude, managerai.AgentGemini}
	if err := managerai.GenerateSharedFiles(a, managerai.ModelsNamed([]string{"Linux Bash"}), agents, true, io.Discard); err != nil {
		t.Fatal(err)
	}
	for name, p := range map[string]string{"A": a, "Gone": missing} {
		if err := managerai.RecordContext(name, p, []string{"Linux Bash"}, agents); err != nil {
			t.Fatal(err)
		}
	}

	d := &DevToolsService{}
	changes, err := d.PreviewAIContextRemove(a)
	if err != nil || !slices.ContainsFunc(changes, func(c AIContextChange) bool { return c.Path == "AGENTS.md" && c.Action == "remove" }) {
		t.Fatalf("preview = %+v, %v", changes, err)
	}
	if changes, err := d.PreviewAIContextRemove(missing); err != nil || changes == nil || len(changes) != 0 {
		t.Errorf("preview of a missing folder = %+v, %v", changes, err)
	}

	if _, done := runAIC(t, func(d *DevToolsService) error { return d.RemoveAIContext(a) }); done["ok"] != true {
		t.Fatalf("action-done = %v", done)
	}
	if _, err := os.Stat(filepath.Join(a, "AGENTS.md")); err == nil {
		t.Error("AGENTS.md kept")
	}
	if _, err := os.Stat(filepath.Join(a, ".instructions", "PROJECT.md")); err != nil {
		t.Error("PROJECT.md removed")
	}
	if _, done := runAIC(t, func(d *DevToolsService) error { return d.RemoveAIContext(missing) }); done["ok"] != true {
		t.Fatalf("action-done = %v", done)
	}
	if rows := aicRows(t); len(rows) != 0 {
		t.Errorf("rows left: %+v", rows)
	}
}

// The list actions accept only registered folders, at least one, none
// repeated.
func TestAIContextListActions_RefuseUnregistered(t *testing.T) {
	isolateActions(t)
	a := t.TempDir()
	if err := managerai.RecordContext("A", a, nil, []string{managerai.AgentCodex}); err != nil {
		t.Fatal(err)
	}
	picked, _ := approvePath(t.TempDir(), nil)
	for _, paths := range [][]string{nil, {picked}, {"relative"}, {a, a}} {
		refusedAIC(t, "UpdateAIContexts "+strings.Join(paths, ","), func(d *DevToolsService) error { return d.UpdateAIContexts(paths) })
		if _, err := (&DevToolsService{}).PreviewAIContextUpdate(paths); err == nil {
			t.Errorf("PreviewAIContextUpdate %v accepted", paths)
		}
	}
	refusedAIC(t, "RemoveAIContext", func(d *DevToolsService) error { return d.RemoveAIContext(picked) })
	if _, err := (&DevToolsService{}).PreviewAIContextRemove(picked); err == nil {
		t.Error("PreviewAIContextRemove of an unregistered folder accepted")
	}
}

// Exportar writes the picked registered folders (relative inside the
// workspace) to a picked file; Importar reads it back, checks it, previews
// the changes and registers the folders.
func TestAIContextExportImport(t *testing.T) {
	isolateActions(t)
	ws := t.TempDir()
	if err := config.Update(func(cfg *config.Config) error { cfg.WorkspacePath = ws; return nil }); err != nil {
		t.Fatal(err)
	}
	inside := filepath.Join(ws, "org", "app")
	outside := filepath.Join(t.TempDir(), "gone")
	if err := os.MkdirAll(inside, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, p := range map[string]string{"App": inside, "Gone": outside} {
		if err := managerai.RecordContext(name, p, []string{"Linux Bash"}, []string{managerai.AgentCodex}); err != nil {
			t.Fatal(err)
		}
	}
	d := &DevToolsService{}
	file := filepath.Join(t.TempDir(), "ctx.yaml")
	if err := d.ExportAIContexts(file, []string{inside, outside}); err == nil {
		t.Error("export to a file not picked accepted")
	}
	file, _ = approvePath(file, nil)
	if err := d.ExportAIContexts(file, []string{t.TempDir()}); err == nil {
		t.Error("export of an unregistered folder accepted")
	}
	if err := d.ExportAIContexts(file, []string{inside, outside}); err != nil {
		t.Fatal(err)
	}
	f, err := managerai.LoadTransfer(file)
	if err != nil || len(f.Contexts) != 2 || f.Contexts[0].Path != "org/app" || !f.Contexts[0].Relative || f.Contexts[1].Path != outside {
		t.Fatalf("exported %+v, %v", f, err)
	}

	// On the "other machine" nothing is registered yet.
	if err := config.Update(func(cfg *config.Config) error { cfg.AIContexts = nil; return nil }); err != nil {
		t.Fatal(err)
	}
	if _, err := d.PreviewAIContextImport(filepath.Join(t.TempDir(), "x.yaml"), nil); err == nil {
		t.Error("preview of a file not picked accepted")
	}
	if _, err := d.PreviewAIContextImport(file, map[string]string{"Gone": t.TempDir()}); err == nil {
		t.Error("replacement folder not picked accepted")
	}
	prev, err := d.PreviewAIContextImport(file, nil)
	if err != nil || len(prev.Items) != 2 || prev.Items[0].State != managerai.ImportApply || prev.Items[0].Path != inside ||
		prev.Items[1].State != managerai.ImportPending || prev.Taken == nil {
		t.Fatalf("preview = %+v, %v", prev, err)
	}

	picks := []managerai.ImportPick{{Source: "App", Name: "App"}, {Source: "Gone", Name: "Gone"}}
	changes, err := d.PreviewAIContextImportChanges(file, picks)
	if err != nil || len(changes) != 2 || len(changes[0].Changes) == 0 || !changes[1].Pending {
		t.Fatalf("changes = %+v, %v", changes, err)
	}
	refusedAIC(t, "repeated name", func(d *DevToolsService) error {
		return d.ImportAIContexts(file, []managerai.ImportPick{{Source: "App", Name: "X"}, {Source: "Gone", Name: "x"}})
	})
	if _, done := runAIC(t, func(d *DevToolsService) error { return d.ImportAIContexts(file, picks) }); done["ok"] != true {
		t.Fatalf("action-done = %v", done)
	}
	rows := aicRows(t)
	if len(rows) != 2 || rows[0].State != managerai.StatusApplied || rows[1].State != managerai.StatusPending {
		t.Errorf("rows after import = %+v", rows)
	}
	prev, err = d.PreviewAIContextImport(file, nil)
	if err != nil || prev.Items[0].State != managerai.ImportRegistered || !slices.Equal(prev.Taken, []string{"App", "Gone"}) {
		t.Errorf("preview after import = %+v, %v", prev, err)
	}
}
