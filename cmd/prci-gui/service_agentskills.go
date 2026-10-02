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

// Bound methods for "Dev Tools :: IA: SKILLs" — a thin layer over
// internal/dev/agentskills (`npx skills add/remove/update`). config.yaml's
// AgentSkills field records which skills are installed, per scope.

import (
	"context"
	"fmt"
	"io"

	"github.com/oito2/perci/internal/config"
	"github.com/oito2/perci/internal/dev/agentskills"
)

// PickAgentSkillsFolder opens the native folder picker for a "Local"
// install target, with CanCreateDirectories enabled.
func (t *DevToolsService) PickAgentSkillsFolder() (string, error) {
	return t.pickFolder("Selecionar pasta do projeto")
}

// AgentSkillRow is one row of "Dev Tools :: IA: SKILLs".
type AgentSkillRow struct {
	Slug      string `json:"slug"`
	Name      string `json:"name"`
	Installed bool   `json:"installed"`
}

// findAgentSkillInstall returns the index of cfg.AgentSkills matching
// (slug, global, folder) — folder is only compared for local (non-global)
// entries.
func findAgentSkillInstall(installs []config.AgentSkillInstall, slug string, global bool, folder string) int {
	return indexOfScoped(installs, agentSkillKey, slug, global, folder)
}

// GetAgentSkillsRows lists the whole catalog with its installed state for
// the given scope (global, or local at folder).
func (t *DevToolsService) GetAgentSkillsRows(global bool, folder string) ([]AgentSkillRow, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	rows := make([]AgentSkillRow, len(agentskills.Catalogue))
	for i, sk := range agentskills.Catalogue {
		installed := findAgentSkillInstall(cfg.AgentSkills, sk.Slug, global, folder) >= 0
		rows[i] = AgentSkillRow{Slug: sk.Slug, Name: sk.Name, Installed: installed}
	}
	return rows, nil
}

// recordAgentSkillInstall updates config.yaml's AgentSkills to reflect
// whether (slug, global, folder) is now installed — replacing any existing
// record for that same key.
func recordAgentSkillInstall(slug string, global bool, folder string, installed bool) error {
	return config.Update(func(cfg *config.Config) error {
		filtered := withoutScoped(cfg.AgentSkills, agentSkillKey, slug, global, folder)
		if installed {
			filtered = append(filtered, config.AgentSkillInstall{Slug: slug, Global: global, Folder: folder})
		}
		cfg.AgentSkills = filtered
		return nil
	})
}

// InstallAgentSkill runs `npx skills add` for slug and records it in
// config.yaml on success.
func (t *DevToolsService) InstallAgentSkill(slug string, global bool, folder string) error {
	if err := requireFolderUnlessGlobal(global, folder); err != nil {
		return err
	}
	return t.runAction(func(stdout io.Writer) error {
		sk, ok := agentskills.BySlug(slug)
		if !ok {
			return fmt.Errorf("skill desconhecida: %s", slug)
		}
		if err := agentskills.Install(context.Background(), t.exe, stdout, sk, global, folder); err != nil {
			return err
		}
		return recordAgentSkillInstall(slug, global, folder, true)
	})
}

// RemoveAgentSkill runs `npx skills remove` for slug and clears its
// config.yaml record on success.
func (t *DevToolsService) RemoveAgentSkill(slug string, global bool, folder string) error {
	if err := requireFolderUnlessGlobal(global, folder); err != nil {
		return err
	}
	return t.runAction(func(stdout io.Writer) error {
		sk, ok := agentskills.BySlug(slug)
		if !ok {
			return fmt.Errorf("skill desconhecida: %s", slug)
		}
		if err := agentskills.Remove(context.Background(), t.exe, stdout, sk, global, folder); err != nil {
			return err
		}
		return recordAgentSkillInstall(slug, global, folder, false)
	})
}

// UpdateAgentSkill runs `npx skills update` for slug — doesn't touch
// config.yaml (an update never changes whether it's installed).
func (t *DevToolsService) UpdateAgentSkill(slug string, global bool, folder string) error {
	if err := requireFolderUnlessGlobal(global, folder); err != nil {
		return err
	}
	return t.runAction(func(stdout io.Writer) error {
		sk, ok := agentskills.BySlug(slug)
		if !ok {
			return fmt.Errorf("skill desconhecida: %s", slug)
		}
		return agentskills.Update(context.Background(), t.exe, stdout, sk, global, folder)
	})
}
