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

package ai

import (
	"io"
	"os"
	"path/filepath"

	"github.com/oito2/perci/internal/ui"
)

// RemoveContext removes from dir the context files Perci generated that
// are still what some Perci version wrote: AGENTS.md (unless it holds
// rules written by hand), each standard under .instructions/ that is
// applied or outdated (GuideStatus), and every agent's files (stubs,
// Claude Code rules and read rules, Gemini ignore files, the OpenCode
// project rules entry). Edited files and .instructions/PROJECT.md stay,
// with a warning for each edited file. Progress goes to stdout.
func RemoveContext(dir string, stdout io.Writer) error {
	return removeContext(dir, dir, stdout)
}

// PreviewRemove tells what RemoveContext(dir, …) would change in dir,
// without touching it.
func PreviewRemove(dir string) ([]Change, error) {
	return previewRun(dir, func(tmp string) error {
		return removeContext(tmp, dir, io.Discard)
	})
}

// removeContext is RemoveContext working in dir while taking the list of
// files git tracks from gitDir (the project itself when dir is a preview
// copy of it).
func removeContext(dir, gitDir string, stdout io.Writer) error {
	agentsPath := filepath.Join(dir, "AGENTS.md")
	if info, err := os.Lstat(agentsPath); err == nil {
		data, err := os.ReadFile(agentsPath)
		if err != nil {
			return err
		}
		own, err := projectContent(string(data))
		if err != nil {
			return err
		}
		if own != "" || !info.Mode().IsRegular() {
			ui.Warning(stdout, agentsPath+" tem regras escritas à mão ou não é um arquivo comum — mantido; apague-o manualmente se não precisar mais dele.")
		} else if err := os.Remove(agentsPath); err != nil {
			ui.Warning(stdout, "Falha ao remover "+agentsPath+": "+err.Error())
		} else {
			ui.Info(stdout, "Removido: "+agentsPath)
		}
	}

	for _, m := range models {
		switch GuideStatus(dir, m) {
		case StatusApplied, StatusOutdated:
			RemoveInstruction(dir, m, stdout)
		case StatusEdited:
			ui.Warning(stdout, ".instructions/"+filepath.Base(m.Instruction)+" foi editado e foi mantido — apague-o manualmente se não precisar mais dele.")
		}
	}

	if err := writeAgentFiles(dir, gitDir, map[string]bool{}, nil, nil, true, stdout); err != nil {
		return err
	}
	// Only an empty folder goes; PROJECT.md keeps it.
	_ = os.Remove(filepath.Join(dir, ".instructions"))
	return nil
}
