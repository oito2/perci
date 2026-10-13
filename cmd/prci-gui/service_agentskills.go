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
// internal/dev/agentskills (`npx skills add/remove`). config.yaml's
// AgentSkills field records which skills are installed, per scope.

import (
	"context"
	"fmt"
	"io"
	"slices"

	"github.com/oito2/perci/internal/config"
	"github.com/oito2/perci/internal/dev/agentskills"
)

// PickAgentSkillsFolder opens the native folder picker for a "Local"
// install target, with CanCreateDirectories enabled.
func (t *DevToolsService) PickAgentSkillsFolder() (string, error) {
	return t.pickFolder("Selecionar pasta do projeto")
}

// AgentSkillRow is one row of "Dev Tools :: IA: SKILLs". Agents lists, by
// name, the agents that accepted the skill (empty for a record written
// before they were recorded); Partial is set when some agent is missing.
type AgentSkillRow struct {
	Slug      string   `json:"slug"`
	Name      string   `json:"name"`
	Installed bool     `json:"installed"`
	Partial   bool     `json:"partial"`
	Agents    []string `json:"agents"`
}

// findAgentSkillInstall returns the index of cfg.AgentSkills matching
// (slug, global, folder) — folder is only compared for local (non-global)
// entries.
func findAgentSkillInstall(installs []config.AgentSkillInstall, slug string, global bool, folder string) int {
	return indexOfScoped(installs, agentSkillKey, slug, global, folder)
}

// agentLabels names each agent ID with label.
func agentLabels(ids []string, label func(string) string) []string {
	names := make([]string, len(ids))
	for i, id := range ids {
		names[i] = label(id)
	}
	return names
}

// isPartial reports whether a record that names its agents lacks any of
// expected. A record without agents (older than the field) never counts as
// partial.
func isPartial(recorded, expected []string) bool {
	if len(recorded) == 0 {
		return false
	}
	for _, id := range expected {
		if !slices.Contains(recorded, id) {
			return true
		}
	}
	return false
}

// GetAgentSkillsRows lists the whole catalog with its installed state for
// the given scope (global, or local at folder).
func (t *DevToolsService) GetAgentSkillsRows(global bool, folder string) ([]AgentSkillRow, error) {
	if err := requireKnownFolderUnlessGlobal(global, folder); err != nil {
		return nil, err
	}
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	rows := make([]AgentSkillRow, len(agentskills.Catalogue))
	for i, sk := range agentskills.Catalogue {
		rows[i] = AgentSkillRow{Slug: sk.Slug, Name: sk.Name, Agents: []string{}}
		if idx := findAgentSkillInstall(cfg.AgentSkills, sk.Slug, global, folder); idx >= 0 {
			rec := cfg.AgentSkills[idx]
			rows[i].Installed = true
			rows[i].Partial = isPartial(rec.Agents, agentskills.TargetAgents())
			rows[i].Agents = agentLabels(rec.Agents, agentskills.AgentLabel)
		}
	}
	return rows, nil
}

// recordAgentSkillInstall updates config.yaml's AgentSkills to reflect
// whether (slug, global, folder) is now installed, and by which agents —
// replacing any existing record for that same key.
func recordAgentSkillInstall(slug string, global bool, folder string, agents []string, installed bool) error {
	return config.Update(func(cfg *config.Config) error {
		filtered := withoutScoped(cfg.AgentSkills, agentSkillKey, slug, global, folder)
		if installed {
			filtered = append(filtered, config.AgentSkillInstall{Slug: slug, Global: global, Folder: folder, Agents: agents})
		}
		cfg.AgentSkills = filtered
		return nil
	})
}

// InstallAgentSkill runs `npx skills add` for slug and records it, with the
// agents that accepted it, in config.yaml on success.
func (t *DevToolsService) InstallAgentSkill(slug string, global bool, folder string) error {
	if err := requireKnownFolderUnlessGlobal(global, folder); err != nil {
		return err
	}
	return t.runAction(func(stdout io.Writer) error {
		sk, ok := agentskills.BySlug(slug)
		if !ok {
			return fmt.Errorf("skill desconhecida: %s", slug)
		}
		agents, err := agentskills.Install(context.Background(), t.exe, stdout, sk, global, folder)
		if err != nil {
			return err
		}
		return recordAgentSkillInstall(slug, global, folder, agents, true)
	})
}

// RemoveAgentSkill runs `npx skills remove` for slug and clears its
// config.yaml record on success.
func (t *DevToolsService) RemoveAgentSkill(slug string, global bool, folder string) error {
	if err := requireKnownFolderUnlessGlobal(global, folder); err != nil {
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
		return recordAgentSkillInstall(slug, global, folder, nil, false)
	})
}

// UpdateAgentSkill reinstalls slug at its pinned commit for the same agents
// as the install and refreshes the agents recorded for it.
func (t *DevToolsService) UpdateAgentSkill(slug string, global bool, folder string) error {
	if err := requireKnownFolderUnlessGlobal(global, folder); err != nil {
		return err
	}
	return t.runAction(func(stdout io.Writer) error {
		sk, ok := agentskills.BySlug(slug)
		if !ok {
			return fmt.Errorf("skill desconhecida: %s", slug)
		}
		agents, err := agentskills.Update(context.Background(), t.exe, stdout, sk, global, folder)
		if err != nil {
			return err
		}
		return recordAgentSkillInstall(slug, global, folder, agents, true)
	})
}
