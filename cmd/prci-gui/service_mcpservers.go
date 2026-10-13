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

// Bound methods for "Dev Tools :: IA: MCPs" — a thin layer over
// internal/dev/mcpservers. config.yaml's MCPServers field records which
// servers are registered, per scope.

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/oito2/perci/internal/config"
	"github.com/oito2/perci/internal/dev/mcpservers"
)

// PickMCPFolder opens the native folder picker for a "Local" install
// target.
func (t *DevToolsService) PickMCPFolder() (string, error) {
	return t.pickFolder("Selecionar pasta do projeto")
}

// PickMCPServerParam opens the right native dialog for slug's extra
// parameter — a folder picker for "filesystem" (allowed directory), a file
// picker filtered to TOML files for "dbhub" (its dbhub.toml). Returns an
// error if slug doesn't take a parameter or left the catalog.
func (t *DevToolsService) PickMCPServerParam(slug string) (string, error) {
	sv, ok := mcpservers.BySlug(slug)
	if !ok || sv.Retired {
		return "", fmt.Errorf("MCP desconhecido: %s", slug)
	}
	switch sv.Param {
	case mcpservers.ParamDirectory:
		return t.pickFolder("Selecionar pasta permitida")
	case mcpservers.ParamFile:
		return approvePath(t.wailsApp.Dialog.OpenFile().
			SetTitle("Selecionar "+sv.ParamLabel).
			AddFilter("TOML (*.toml)", "*.toml").
			PromptForSingleSelection())
	}
	return "", fmt.Errorf("%s não precisa de um parâmetro extra", slug)
}

// MCPServerRow is one row of "Dev Tools :: IA: MCPs". Agents lists, by
// name, the agents that accepted the registration (empty for a record
// written before they were recorded); Partial is set when some agent that
// applies to the scope is missing. Retired marks an entry that left the
// catalog, listed only while registered so it can be removed.
type MCPServerRow struct {
	Slug       string   `json:"slug"`
	Name       string   `json:"name"`
	Note       string   `json:"note"`
	Retired    bool     `json:"retired"`
	Installed  bool     `json:"installed"`
	Partial    bool     `json:"partial"`
	Agents     []string `json:"agents"`
	Param      string   `json:"param"`     // persisted value (if installed) — pre-fills "Update"
	ParamKind  string   `json:"paramKind"` // "" | "directory" | "file"
	ParamLabel string   `json:"paramLabel"`
	Manual     bool     `json:"manual"` // true = Godot Studio: no Install/Update/Remove, just a link/instructions
	ManualURL  string   `json:"manualUrl"`
	ManualNote string   `json:"manualNote"`
}

// findMCPServerInstall is findAgentSkillInstall's counterpart for MCP
// server records.
func findMCPServerInstall(installs []config.MCPServerInstall, slug string, global bool, folder string) int {
	return indexOfScoped(installs, mcpServerKey, slug, global, folder)
}

// GetMCPServerRows lists the whole catalog with its registered state for
// the given scope (global, or local at folder).
func (t *DevToolsService) GetMCPServerRows(global bool, folder string) ([]MCPServerRow, error) {
	if err := requireKnownFolderUnlessGlobal(global, folder); err != nil {
		return nil, err
	}
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	rows := make([]MCPServerRow, 0, len(mcpservers.Catalogue))
	for _, sv := range mcpservers.Catalogue {
		row := MCPServerRow{
			Slug: sv.Slug, Name: sv.Name, Note: sv.Note, Retired: sv.Retired, Agents: []string{},
			ParamKind: string(sv.Param), ParamLabel: sv.ParamLabel,
			Manual: sv.Manual, ManualURL: sv.ManualURL, ManualNote: sv.ManualNote,
		}
		if idx := findMCPServerInstall(cfg.MCPServers, sv.Slug, global, folder); idx >= 0 {
			rec := cfg.MCPServers[idx]
			row.Installed = true
			row.Param = rec.Param
			row.Partial = isPartial(rec.Agents, mcpservers.ExpectedAgents(sv.Slug, global))
			row.Agents = agentLabels(rec.Agents, mcpservers.AgentLabel)
		} else if sv.Retired {
			continue
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// recordMCPServerInstall updates config.yaml's MCPServers to reflect
// whether (slug, global, folder) is now registered, and with which agents —
// replacing any existing record for that same key.
func recordMCPServerInstall(slug string, global bool, folder, param string, agents []string, installed bool) error {
	return config.Update(func(cfg *config.Config) error {
		filtered := withoutScoped(cfg.MCPServers, mcpServerKey, slug, global, folder)
		if installed {
			filtered = append(filtered, config.MCPServerInstall{Slug: slug, Global: global, Folder: folder, Param: param, Agents: agents})
		}
		cfg.MCPServers = filtered
		return nil
	})
}

// InstallMCPServer registers slug with every supported agent and records
// it, with the agents that accepted it, in config.yaml on success.
func (t *DevToolsService) InstallMCPServer(slug string, global bool, folder, param string) error {
	if err := requireKnownFolderUnlessGlobal(global, folder); err != nil {
		return err
	}
	if err := requireMCPParam(slug, global, folder, param); err != nil {
		return err
	}
	return t.runAction(func(stdout io.Writer) error {
		agents, err := mcpservers.Install(context.Background(), t.exe, stdout, slug, global, folder, param)
		if err != nil {
			return err
		}
		return recordMCPServerInstall(slug, global, folder, param, agents, true)
	})
}

// RemoveMCPServer unregisters slug from every supported agent and clears
// its config.yaml record on success.
func (t *DevToolsService) RemoveMCPServer(slug string, global bool, folder string) error {
	if err := requireKnownFolderUnlessGlobal(global, folder); err != nil {
		return err
	}
	return t.runAction(func(stdout io.Writer) error {
		if err := mcpservers.Remove(context.Background(), t.exe, stdout, slug, global, folder); err != nil {
			return err
		}
		return recordMCPServerInstall(slug, global, folder, "", nil, false)
	})
}

// UpdateMCPServer refreshes slug's registration and keeps config.yaml's
// stored Param and agents in sync (in case the user changed the parameter
// before updating).
func (t *DevToolsService) UpdateMCPServer(slug string, global bool, folder, param string) error {
	if err := requireKnownFolderUnlessGlobal(global, folder); err != nil {
		return err
	}
	if err := requireMCPParam(slug, global, folder, param); err != nil {
		return err
	}
	return t.runAction(func(stdout io.Writer) error {
		agents, err := mcpservers.Update(context.Background(), t.exe, stdout, slug, global, folder, param)
		if err != nil {
			if errors.Is(err, mcpservers.ErrNotReinstalled) {
				// Removed everywhere, re-registered nowhere: the list must
				// not keep showing it as installed.
				if recErr := recordMCPServerInstall(slug, global, folder, "", nil, false); recErr != nil {
					return errors.Join(err, recErr)
				}
			}
			return err
		}
		return recordMCPServerInstall(slug, global, folder, param, agents, true)
	})
}
