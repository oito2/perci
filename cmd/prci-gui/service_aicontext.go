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
	"io"
	"slices"

	managerai "github.com/oito2/perci/internal/manager/ai"
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
// own doc comment). "Aplicar" always overwrites (overwrite=true) with no
// extra confirmation — the button click is itself the confirmation, same
// convention as every non-interactive GUI action.
func (t *DevToolsService) ApplyAIContext(folder string, selectedNames, agentIDs []string) error {
	if err := requireKnownFolder(folder); err != nil {
		return err
	}
	if len(agentIDs) == 0 {
		return errors.New("marque ao menos um agente de IA")
	}
	return t.runAction(func(stdout io.Writer) error {
		return managerai.GenerateSharedFiles(folder, selectedModels(selectedNames), agentIDs, true, stdout)
	})
}
