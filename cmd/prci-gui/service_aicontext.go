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
// the shell's current folder, the GUI passes the folder the user picked.
//
// Bypasses RunOptions/ModelsBySlugs/RunContext (the CLI's
// `--models=slug,slug` flow): the GUI works with a Model's full display
// name (e.g. "Linux Bash") straight from the checked checkbox.
// GenerateSharedFiles/DetectActiveModels do the actual work.

import (
	"io"

	managerai "github.com/oito2/perci/internal/manager/ai"
)

// PickAIContextFolder opens the native folder picker for the working
// folder this screen operates on.
func (t *DevToolsService) PickAIContextFolder() (string, error) {
	return t.pickFolder("Selecionar pasta de trabalho")
}

// GetAIContextItems lists every catalog Model with whether its instruction
// file is already present under folder/.instructions/, as the
// MultiSelectItemInfo {id,label,installed} shape.
func (t *DevToolsService) GetAIContextItems(folder string) []MultiSelectItemInfo {
	detected := managerai.DetectActiveModels(folder)
	all := managerai.Models()
	items := make([]MultiSelectItemInfo, len(all))
	for i, m := range all {
		items[i] = MultiSelectItemInfo{ID: m.Name, Label: m.Name, Installed: detected[m.Name]}
	}
	return items
}

// ApplyAIContext (re)generates CLAUDE.md/AGENTS.md/ignore files and each
// checked model's .instructions/*.md file in folder, and removes the
// .instructions/*.md of any model that was active and got unchecked (see
// GenerateSharedFiles' own doc comment). "Aplicar" always overwrites
// (overwrite=true) with no extra confirmation — the button click is
// itself the confirmation, same convention as every non-interactive GUI
// action.
func (t *DevToolsService) ApplyAIContext(folder string, selectedNames []string) error {
	if err := requireFolder(folder); err != nil {
		return err
	}
	return t.runAction(func(stdout io.Writer) error {
		selected := make(map[string]bool, len(selectedNames))
		for _, n := range selectedNames {
			selected[n] = true
		}
		all := managerai.Models()
		active := make([]managerai.Model, 0, len(all))
		for _, m := range all {
			if selected[m.Name] {
				active = append(active, m)
			}
		}
		return managerai.GenerateSharedFiles(folder, active, true, stdout)
	})
}
