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

// Bound methods for "Dev Tools :: IA: Contextos" — a thin layer over
// internal/manager/ai, whose functions take a `dir` parameter: "" means
// the process's current folder, the GUI passes the folder the user picked.
// Models are identified by their display name (e.g. "Linux Bash"), agents
// by their ID (e.g. "claude"). GenerateSharedFiles/DetectActiveModels/
// DetectAgents do the actual work.

import (
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/oito2/perci/internal/config"
	"github.com/oito2/perci/internal/executor"
	managerai "github.com/oito2/perci/internal/manager/ai"
	"github.com/oito2/perci/internal/ui"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// PickAIContextFolder opens the native folder picker for the working
// folder this screen operates on.
func (t *DevToolsService) PickAIContextFolder() (string, error) {
	return t.pickFolder("Selecionar pasta de trabalho")
}

// AIContextItem is one standard of "Dev Tools :: IA: Contextos": Base is
// the standard it complements ("" for a base), Status its state in the
// folder ("", "applied", "outdated" or "edited", see managerai.GuideStatus).
type AIContextItem struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Base        string `json:"base"`
	Status      string `json:"status"`
}

// GetAIContextItems lists every standard of the catalog, in catalog order,
// with its state in folder.
func (t *DevToolsService) GetAIContextItems(folder string) []AIContextItem {
	known := requireKnownFolder(folder) == nil
	all := managerai.Models()
	items := make([]AIContextItem, len(all))
	for i, m := range all {
		items[i] = AIContextItem{ID: m.Name, Label: m.Name, Description: m.Description, Base: m.Requires}
		if known {
			items[i].Status = managerai.GuideStatus(folder, m)
		}
	}
	return items
}

// GetAIContextSuggestedPreset names the project type the files in folder
// point to ("" for none).
func (t *DevToolsService) GetAIContextSuggestedPreset(folder string) string {
	if requireKnownFolder(folder) != nil {
		return ""
	}
	return managerai.SuggestPreset(folder)
}

// AIContextChange is one file an Aplicar would create, update or remove
// ("create", "update", "remove"), relative to the folder.
type AIContextChange struct {
	Path   string `json:"path"`
	Action string `json:"action"`
}

// PreviewAIContext tells what ApplyAIContext(folder, selectedNames,
// agentIDs) would change, without changing anything.
func (t *DevToolsService) PreviewAIContext(folder string, selectedNames, agentIDs []string) ([]AIContextChange, error) {
	if err := requireKnownFolder(folder); err != nil {
		return nil, err
	}
	changes, err := managerai.Preview(folder, selectedModels(selectedNames), agentIDs)
	if err != nil {
		return nil, err
	}
	out := make([]AIContextChange, len(changes))
	for i, c := range changes {
		out[i] = AIContextChange{Path: c.Path, Action: c.Action}
	}
	return out, nil
}

// selectedModels returns the catalog models named by names, in catalog
// order.
func selectedModels(names []string) []managerai.Model {
	all := managerai.Models()
	active := make([]managerai.Model, 0, len(all))
	for _, m := range all {
		if slices.Contains(names, m.Name) {
			active = append(active, m)
		}
	}
	return active
}

// AIContextAgent is one coding agent of "Dev Tools :: IA: Contextos";
// Selected says the folder's context was generated for it (every agent
// for a folder without an AGENTS.md yet).
type AIContextAgent struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Selected bool   `json:"selected"`
}

// GetAIContextAgents lists the supported agents, marking the ones the
// context in folder was generated for.
func (t *DevToolsService) GetAIContextAgents(folder string) []AIContextAgent {
	var detected map[string]bool
	if requireKnownFolder(folder) == nil {
		detected = managerai.DetectAgents(folder)
	}
	all := managerai.Agents()
	out := make([]AIContextAgent, len(all))
	for i, a := range all {
		out[i] = AIContextAgent{ID: a.ID, Label: a.Label, Selected: len(detected) == 0 || detected[a.ID]}
	}
	return out
}

// AIContextPreset is a project type of "Dev Tools :: IA: Contextos": the
// models it checks at once and an optional hint about what to add by hand.
type AIContextPreset struct {
	Name   string   `json:"name"`
	Models []string `json:"models"`
	Hint   string   `json:"hint,omitempty"`
}

// GetAIContextPresets lists the project types the screen offers.
func (t *DevToolsService) GetAIContextPresets() []AIContextPreset {
	all := managerai.Presets()
	out := make([]AIContextPreset, len(all))
	for i, p := range all {
		out[i] = AIContextPreset{Name: p.Name, Models: p.Models, Hint: p.Hint}
	}
	return out
}

// ApplyAIContext (re)generates, in folder, AGENTS.md, each checked model's
// .instructions/*.md and the files of each checked agent, and removes what
// Perci wrote for unchecked models and agents (see GenerateSharedFiles'
// own doc comment), then registers folder as name with that selection
// (managerai.RecordContext). The name is checked before anything is
// written. "Aplicar" always overwrites (overwrite=true) with no extra
// confirmation — the button click is itself the confirmation, same
// convention as every non-interactive GUI action.
func (t *DevToolsService) ApplyAIContext(folder, name string, selectedNames, agentIDs []string) error {
	if err := requireKnownFolder(folder); err != nil {
		return err
	}
	if len(agentIDs) == 0 {
		return errors.New("marque ao menos um agente de IA")
	}
	if _, err := managerai.ValidateContextName(name, folder); err != nil {
		return err
	}
	return t.runAction(func(stdout io.Writer) error {
		if err := managerai.GenerateSharedFiles(folder, selectedModels(selectedNames), agentIDs, true, stdout); err != nil {
			return err
		}
		if err := managerai.RecordContext(name, folder, selectedNames, agentIDs); err != nil {
			return fmt.Errorf("contexto aplicado, mas o registro da pasta falhou: %w", err)
		}
		ui.Success(stdout, "Contexto registrado como \""+strings.TrimSpace(name)+"\".")
		return nil
	})
}

// AIContextRow is one registered folder of the "Contextos" tab: State is
// "applied", "outdated", "edited" or "pending" (managerai.ContextState),
// Divergent says its files hold other standards or agents than the
// recorded ones; Models and Agents are the recorded standards and agent
// IDs.
type AIContextRow struct {
	Name      string   `json:"name"`
	Path      string   `json:"path"`
	State     string   `json:"state"`
	Divergent bool     `json:"divergent"`
	Models    []string `json:"models"`
	Agents    []string `json:"agents"`
}

// GetAIContexts lists the registered folders, in registration order, each
// with its state read from its files.
func (t *DevToolsService) GetAIContexts() ([]AIContextRow, error) {
	all, err := managerai.Contexts()
	if err != nil {
		return nil, err
	}
	rows := make([]AIContextRow, len(all))
	for i, c := range all {
		st, err := managerai.ContextState(c)
		if err != nil {
			return nil, err
		}
		rows[i] = AIContextRow{Name: c.Name, Path: c.Path, State: st.State, Divergent: st.Divergent,
			Models: nonNil(c.Models), Agents: nonNil(c.Agents)}
	}
	return rows, nil
}

// nonNil returns s, or an empty slice for nil (sent as [] rather than
// null).
func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// AIContextRecord is what "Definir Contexto" loads for a folder:
// Registered says it is in the list (Name, Models and Agents are then the
// record); otherwise HasContext says its files hold a context Perci
// generated (Models and Agents are then what they hold) that can be
// registered with RegisterAIContext.
type AIContextRecord struct {
	Registered bool     `json:"registered"`
	HasContext bool     `json:"hasContext"`
	Name       string   `json:"name"`
	Models     []string `json:"models"`
	Agents     []string `json:"agents"`
}

// GetAIContextRecord returns the record of folder, or what its files hold
// when it is not registered.
func (t *DevToolsService) GetAIContextRecord(folder string) (AIContextRecord, error) {
	if err := requireKnownFolder(folder); err != nil {
		return AIContextRecord{}, err
	}
	rec, err := managerai.FindContext(folder)
	switch {
	case err == nil:
		return AIContextRecord{Registered: true, HasContext: managerai.HasContext(folder), Name: rec.Name,
			Models: nonNil(rec.Models), Agents: nonNil(rec.Agents)}, nil
	case !errors.Is(err, managerai.ErrContextNotFound):
		return AIContextRecord{}, err
	}
	out := AIContextRecord{HasContext: managerai.HasContext(folder), Models: []string{}, Agents: []string{}}
	if out.HasContext {
		ms, ids := managerai.Detected(folder)
		out.Models, out.Agents = nonNil(ms), nonNil(ids)
	}
	return out, nil
}

// RegisterAIContext adds folder, whose files hold a context Perci
// generated, to the list as name with the standards and agents they hold;
// no file is written.
func (t *DevToolsService) RegisterAIContext(folder, name string) error {
	if err := requireKnownFolder(folder); err != nil {
		return err
	}
	return runExclusive(func() error { return managerai.RegisterDetected(name, folder) })
}

// requireRegisteredContext accepts only the path of a registered folder,
// returning its record. The folder itself may be missing (a pending
// record).
func requireRegisteredContext(path string) (config.AIContext, error) {
	rec, err := managerai.FindContext(path)
	if err != nil {
		return config.AIContext{}, fmt.Errorf("%w: %s", err, path)
	}
	return rec, nil
}

// requireRegisteredContexts is requireRegisteredContext for each of
// paths, which must be at least one and not repeat.
func requireRegisteredContexts(paths []string) ([]config.AIContext, error) {
	if len(paths) == 0 {
		return nil, errors.New("nenhuma pasta selecionada")
	}
	recs := make([]config.AIContext, 0, len(paths))
	for _, p := range paths {
		rec, err := requireRegisteredContext(p)
		if err != nil {
			return nil, err
		}
		if slices.ContainsFunc(recs, func(r config.AIContext) bool { return r.Path == rec.Path }) {
			return nil, fmt.Errorf("pasta repetida: %s", p)
		}
		recs = append(recs, rec)
	}
	return recs, nil
}

// folderExists reports whether path is an existing directory.
func folderExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// AIContextFolderChanges is the preview of one registered folder: the
// files an Atualizar would change in it, or Pending when the folder does
// not exist (it is skipped).
type AIContextFolderChanges struct {
	Name    string            `json:"name"`
	Path    string            `json:"path"`
	Pending bool              `json:"pending"`
	Changes []AIContextChange `json:"changes"`
}

// toChanges converts the domain changes for the frontend.
func toChanges(changes []managerai.Change) []AIContextChange {
	out := make([]AIContextChange, len(changes))
	for i, c := range changes {
		out[i] = AIContextChange{Path: c.Path, Action: c.Action}
	}
	return out
}

// PreviewAIContextUpdate tells, for each registered folder of paths, what
// UpdateAIContexts would change in it, without changing anything.
func (t *DevToolsService) PreviewAIContextUpdate(paths []string) ([]AIContextFolderChanges, error) {
	recs, err := requireRegisteredContexts(paths)
	if err != nil {
		return nil, err
	}
	out := make([]AIContextFolderChanges, len(recs))
	for i, rec := range recs {
		out[i] = AIContextFolderChanges{Name: rec.Name, Path: rec.Path, Changes: []AIContextChange{}}
		if !folderExists(rec.Path) {
			out[i].Pending = true
			continue
		}
		changes, err := managerai.Preview(rec.Path, managerai.ModelsNamed(rec.Models), rec.Agents)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", rec.Name, err)
		}
		out[i].Changes = toChanges(changes)
	}
	return out, nil
}

// UpdateAIContexts regenerates, in each registered folder of paths, the
// context of its record (standards and agents), one folder after the
// other. A missing folder or a failing generation is reported and the
// next folder still runs; the action then ends with
// executor.ErrCompletedWithWarnings.
func (t *DevToolsService) UpdateAIContexts(paths []string) error {
	recs, err := requireRegisteredContexts(paths)
	if err != nil {
		return err
	}
	return t.runAction(func(stdout io.Writer) error {
		failed := false
		for i, rec := range recs {
			ui.Step(stdout, i+1, len(recs), rec.Name)
			if !folderExists(rec.Path) {
				ui.Warning(stdout, "Pasta não encontrada, pulada: "+rec.Path)
				failed = true
				continue
			}
			if err := managerai.GenerateSharedFiles(rec.Path, managerai.ModelsNamed(rec.Models), rec.Agents, true, stdout); err != nil {
				ui.Err(stdout, rec.Path+": "+err.Error())
				failed = true
			}
		}
		if failed {
			return executor.ErrCompletedWithWarnings
		}
		return nil
	})
}

// PreviewAIContextRemove tells what RemoveAIContext(path) would remove
// from the registered folder at path, without changing anything: nothing
// for a missing folder, which is only dropped from the list.
func (t *DevToolsService) PreviewAIContextRemove(path string) ([]AIContextChange, error) {
	rec, err := requireRegisteredContext(path)
	if err != nil {
		return nil, err
	}
	if !folderExists(rec.Path) {
		return []AIContextChange{}, nil
	}
	changes, err := managerai.PreviewRemove(rec.Path)
	if err != nil {
		return nil, err
	}
	return toChanges(changes), nil
}

// RemoveAIContext removes, from the registered folder at path, the context
// files Perci generated that were not edited (managerai.RemoveContext),
// then drops the folder from the list. A missing folder is only dropped
// from the list.
func (t *DevToolsService) RemoveAIContext(path string) error {
	rec, err := requireRegisteredContext(path)
	if err != nil {
		return err
	}
	return t.runAction(func(stdout io.Writer) error {
		if folderExists(rec.Path) {
			if err := managerai.RemoveContext(rec.Path, stdout); err != nil {
				return err
			}
		} else {
			ui.Info(stdout, "Pasta não encontrada: só o registro é removido.")
		}
		if err := managerai.ForgetContext(rec.Path); err != nil {
			return err
		}
		ui.Success(stdout, "\""+rec.Name+"\" removido da lista de contextos.")
		return nil
	})
}

// PickAIContextExportPath opens the native "Salvar Como" dialog for the
// AI contexts export file.
func (t *DevToolsService) PickAIContextExportPath() (string, error) {
	return approvePath(t.wailsApp.Dialog.SaveFileWithOptions(&application.SaveFileDialogOptions{
		Title:    "Exportar contextos de IA",
		Filename: "perci-contextos-ia.yaml",
		Filters:  []application.FileFilter{{DisplayName: "YAML (*.yaml)", Pattern: "*.yaml;*.yml"}},
	}).PromptForSingleSelection())
}

// aicWorkspace is the workspace folder relative paths of the export file
// are based on.
func aicWorkspace() (string, error) {
	cfg, err := config.Load()
	if err != nil {
		return "", err
	}
	return cfg.Workspace()
}

// ExportAIContexts writes the registered folders of paths to path (only a
// path chosen in PickAIContextExportPath); folders inside the workspace
// get a path relative to it.
func (t *DevToolsService) ExportAIContexts(path string, paths []string) error {
	if err := requireApprovedPath(path); err != nil {
		return err
	}
	recs, err := requireRegisteredContexts(paths)
	if err != nil {
		return err
	}
	ws, err := aicWorkspace()
	if err != nil {
		return err
	}
	return managerai.WriteTransfer(path, managerai.Exportable(recs, ws))
}

// PickAIContextImportPath opens the native "Selecionar arquivo" dialog
// for an AI contexts export file.
func (t *DevToolsService) PickAIContextImportPath() (string, error) {
	return approvePath(t.wailsApp.Dialog.OpenFile().
		SetTitle("Selecionar arquivo de contextos de IA").
		AddFilter("YAML (*.yaml)", "*.yaml;*.yml").
		PromptForSingleSelection())
}

// loadAIContextImport reads the export file at path (only a path chosen in
// PickAIContextImportPath) and checks that every replacement folder in
// paths came from the folder picker.
func loadAIContextImport(path string, paths []string) (managerai.TransferFile, error) {
	if err := requireApprovedPath(path); err != nil {
		return managerai.TransferFile{}, err
	}
	for _, p := range paths {
		if p != "" && requireApprovedPath(p) != nil {
			return managerai.TransferFile{}, fmt.Errorf("escolha a pasta no seletor: %s", p)
		}
	}
	return managerai.LoadTransfer(path)
}

// pickPaths returns the replacement folders of picks.
func pickPaths(picks []managerai.ImportPick) []string {
	out := make([]string, len(picks))
	for i, p := range picks {
		out[i] = p.Path
	}
	return out
}

// AIContextImportPreview is the Importar checklist: each folder of the
// file checked against this machine, and the names registered here (a
// picked name must not be one of them).
type AIContextImportPreview struct {
	Items []managerai.ImportContext `json:"items"`
	Taken []string                  `json:"taken"`
}

// PreviewAIContextImport reads the export file at path and checks each of
// its folders against this machine, with paths (keyed by the folder's
// name in the file, each chosen in the folder picker) replacing the
// folders the file gives — without applying anything.
func (t *DevToolsService) PreviewAIContextImport(path string, paths map[string]string) (AIContextImportPreview, error) {
	replacements := make([]string, 0, len(paths))
	for _, p := range paths {
		replacements = append(replacements, p)
	}
	file, err := loadAIContextImport(path, replacements)
	if err != nil {
		return AIContextImportPreview{}, err
	}
	cfg, err := config.Load()
	if err != nil {
		return AIContextImportPreview{}, err
	}
	ws, err := cfg.Workspace()
	if err != nil {
		return AIContextImportPreview{}, err
	}
	taken := make([]string, len(cfg.AIContexts))
	for i, c := range cfg.AIContexts {
		taken[i] = c.Name
	}
	return AIContextImportPreview{Items: managerai.PlanImport(file, cfg.AIContexts, ws, paths), Taken: taken}, nil
}

// PreviewAIContextImportChanges tells, for each pick of the export file
// at path, the files ImportAIContexts would change in its folder (none
// for a pending folder), without changing anything.
func (t *DevToolsService) PreviewAIContextImportChanges(path string, picks []managerai.ImportPick) ([]AIContextFolderChanges, error) {
	file, err := loadAIContextImport(path, pickPaths(picks))
	if err != nil {
		return nil, err
	}
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	ws, err := cfg.Workspace()
	if err != nil {
		return nil, err
	}
	plans, changes, err := managerai.PreviewImport(file, picks, cfg.AIContexts, ws)
	if err != nil {
		return nil, err
	}
	out := make([]AIContextFolderChanges, len(plans))
	for i, p := range plans {
		out[i] = AIContextFolderChanges{Name: p.Name, Path: p.Path, Pending: p.State == managerai.ImportPending, Changes: toChanges(changes[i])}
	}
	return out, nil
}

// ImportAIContexts imports the picks of the export file at path
// (managerai.Import), on the Execução tab: existing folders get their
// context and are registered, missing ones are registered as pending. The
// picks are checked before the action starts.
func (t *DevToolsService) ImportAIContexts(path string, picks []managerai.ImportPick) error {
	file, err := loadAIContextImport(path, pickPaths(picks))
	if err != nil {
		return err
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ws, err := cfg.Workspace()
	if err != nil {
		return err
	}
	if err := managerai.CheckImport(file, picks, cfg.AIContexts, ws); err != nil {
		return err
	}
	return t.runAction(func(stdout io.Writer) error {
		return managerai.Import(stdout, file, picks, ws)
	})
}
