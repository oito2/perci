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
	"testing"

	"github.com/oito2/perci/internal/config"
)

func TestFindAgentSkillInstall(t *testing.T) {
	installs := []config.AgentSkillInstall{
		{Slug: "a", Global: true},
		{Slug: "b", Global: false, Folder: "/x"},
	}
	if idx := findAgentSkillInstall(installs, "b", false, "/x"); idx != 1 {
		t.Errorf("esperava índice 1, veio %d", idx)
	}
	if idx := findAgentSkillInstall(installs, "b", false, "/outra"); idx != -1 {
		t.Errorf("pasta diferente não deveria casar, veio %d", idx)
	}
	if idx := findAgentSkillInstall(installs, "a", true, ""); idx != 0 {
		t.Errorf("esperava índice 0 (global), veio %d", idx)
	}
	if idx := findAgentSkillInstall(installs, "nope", false, ""); idx != -1 {
		t.Errorf("slug inexistente deveria retornar -1, veio %d", idx)
	}
}

func TestFindMCPServerInstall(t *testing.T) {
	installs := []config.MCPServerInstall{
		{Slug: "a", Global: true, Param: "p1"},
		{Slug: "b", Global: false, Folder: "/x"},
	}
	if idx := findMCPServerInstall(installs, "b", false, "/x"); idx != 1 {
		t.Errorf("esperava índice 1, veio %d", idx)
	}
	if idx := findMCPServerInstall(installs, "b", false, "/outra"); idx != -1 {
		t.Errorf("pasta diferente não deveria casar, veio %d", idx)
	}
	if idx := findMCPServerInstall(installs, "a", true, ""); idx != 0 {
		t.Errorf("esperava índice 0 (global), veio %d", idx)
	}
}

func TestIsAppKind(t *testing.T) {
	for _, k := range []string{config.AppTypeMoodle, config.AppTypePHP, config.AppTypeGeneric, config.AppTypeNode, config.AppTypePHPNode} {
		if !isAppKind(k) {
			t.Errorf("%s deveria ser reconhecido como app kind", k)
		}
	}
	for _, k := range []string{"nginx", "mariadb", "", "unknown"} {
		if isAppKind(k) {
			t.Errorf("%s não deveria ser reconhecido como app kind", k)
		}
	}
}

// A local record matches its folder with or without a trailing slash.
func TestIndexOfScoped_CleansFolder(t *testing.T) {
	installs := []config.AgentSkillInstall{{Slug: "a", Folder: "/proj/x/"}}
	if idx := findAgentSkillInstall(installs, "a", false, "/proj/x"); idx != 0 {
		t.Errorf("idx = %d, want 0", idx)
	}
	if got := withoutScoped(installs, agentSkillKey, "a", false, "/proj/x"); len(got) != 0 {
		t.Errorf("record not removed: %v", got)
	}
}

func TestOneOf(t *testing.T) {
	if oneOf("b", []string{"a", "b"}, "a") != "b" || oneOf("z", []string{"a", "b"}, "a") != "a" {
		t.Error("oneOf must keep a valid name and fall back otherwise")
	}
}
