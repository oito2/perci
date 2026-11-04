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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// Agent IDs.
const (
	AgentClaude      = "claude"
	AgentGemini      = "gemini"
	AgentCodex       = "codex"
	AgentOpenCode    = "opencode"
	AgentAntigravity = "antigravity"
)

// Agent is a coding agent the generated context targets.
type Agent struct {
	ID    string
	Label string
}

// agents is the catalog of supported agents, in display order.
var agents = []Agent{
	{ID: AgentClaude, Label: "Claude Code"},
	{ID: AgentGemini, Label: "Gemini (CLI e Code Assist)"},
	{ID: AgentCodex, Label: "ChatGPT CLI (Codex)"},
	{ID: AgentOpenCode, Label: "OpenCode"},
	{ID: AgentAntigravity, Label: "Antigravity"},
}

// Agents returns the supported agents.
func Agents() []Agent {
	return agents
}

// AgentIDs returns the ID of every supported agent.
func AgentIDs() []string {
	ids := make([]string, len(agents))
	for i, a := range agents {
		ids[i] = a.ID
	}
	return ids
}

// agentSet validates ids: at least one, every one known.
func agentSet(ids []string) (map[string]bool, error) {
	if len(ids) == 0 {
		return nil, errors.New("marque ao menos um agente de IA")
	}
	set := make(map[string]bool, len(ids))
	for _, id := range ids {
		if !slices.ContainsFunc(agents, func(a Agent) bool { return a.ID == id }) {
			return nil, fmt.Errorf("agente desconhecido: %q", id)
		}
		set[id] = true
	}
	return set, nil
}

// agentsMarkerRe matches the comment AGENTS.md ends with, naming the
// agents it was generated for.
var agentsMarkerRe = regexp.MustCompile(`(?m)^<!-- Perci: generated for ([a-z, ]+) -->$`)

// agentsMarker is the comment AGENTS.md ends with: the IDs of set, in
// catalog order.
func agentsMarker(set map[string]bool) string {
	var ids []string
	for _, a := range agents {
		if set[a.ID] {
			ids = append(ids, a.ID)
		}
	}
	return "<!-- Perci: generated for " + strings.Join(ids, ", ") + " -->"
}

// DetectAgents tells which agents the context in dir was generated for:
// the ones AGENTS.md names; for an AGENTS.md from an older version, Codex
// and Antigravity (which read it as is) plus the agents whose files are
// present; none without an AGENTS.md.
func DetectAgents(dir string) map[string]bool {
	found := map[string]bool{}
	data, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		return found
	}
	if m := agentsMarkerRe.FindSubmatch(data); m != nil {
		for _, id := range strings.Split(string(m[1]), ",") {
			if id = strings.TrimSpace(id); id != "" {
				found[id] = true
			}
		}
		return found
	}
	exists := func(name string) bool {
		_, err := os.Lstat(filepath.Join(dir, name))
		return err == nil
	}
	found[AgentCodex] = true
	found[AgentAntigravity] = true
	found[AgentClaude] = exists("CLAUDE.md")
	found[AgentGemini] = exists("GEMINI.md")
	found[AgentOpenCode] = openCodeHasProjectRules(dir)
	return found
}
