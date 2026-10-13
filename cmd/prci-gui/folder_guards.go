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

// Guards for folders and parameters that reach the services of the
// "Dev Tools" category from the frontend.

import (
	"fmt"
	"path/filepath"

	"github.com/oito2/perci/internal/config"
	"github.com/oito2/perci/internal/dev/mcpservers"
)

// requireKnownFolder is requireFolder plus: the folder was picked in a
// native dialog this session, or the user registered it in an earlier one
// (a remembered repository folder, or the folder of a local SKILL or MCP
// record). A folder that is neither is refused, so nothing running in the
// webview can point an action at $HOME or any other directory it names.
func requireKnownFolder(p string) error {
	if err := requireFolder(p); err != nil {
		return err
	}
	if requireApprovedPath(p) == nil {
		return nil
	}
	clean := filepath.Clean(p)
	if cfg, err := config.Load(); err == nil {
		for _, f := range cfg.RepoFolders {
			if filepath.Clean(f) == clean {
				return nil
			}
		}
		for _, r := range cfg.AgentSkills {
			if !r.Global && filepath.Clean(r.Folder) == clean {
				return nil
			}
		}
		for _, r := range cfg.MCPServers {
			if !r.Global && filepath.Clean(r.Folder) == clean {
				return nil
			}
		}
	}
	return fmt.Errorf("pasta não escolhida no seletor: %s", p)
}

// requireKnownFolderUnlessGlobal is requireKnownFolder for actions with a
// Global/Local scope — the folder only matters for Local.
func requireKnownFolderUnlessGlobal(global bool, p string) error {
	if global {
		return nil
	}
	return requireKnownFolder(p)
}

// requireMCPParam accepts the extra parameter of an MCP server (the folder
// Filesystem may read, the SQLite file) only when it was picked in a native
// dialog this session or is the value already recorded for (slug, global,
// folder). An empty parameter passes: the install refuses it with its own
// message.
func requireMCPParam(slug string, global bool, folder, param string) error {
	sv, ok := mcpservers.BySlug(slug)
	if !ok || sv.Param == mcpservers.ParamNone || param == "" || requireApprovedPath(param) == nil {
		return nil
	}
	if cfg, err := config.Load(); err == nil {
		if i := findMCPServerInstall(cfg.MCPServers, slug, global, folder); i >= 0 && cfg.MCPServers[i].Param == param {
			return nil
		}
	}
	return fmt.Errorf("caminho não escolhido no seletor de arquivos: %s", param)
}
