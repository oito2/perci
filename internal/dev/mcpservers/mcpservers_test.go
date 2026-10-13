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

package mcpservers

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

func TestValidateParam(t *testing.T) {
	cases := []struct {
		slug, param string
		ok          bool
	}{
		{"filesystem", "/home/u/proj", true},
		{"filesystem", "", false},
		{"filesystem", "relative/dir", false},
		{"filesystem", "--help", false},
		{"sqlite", "/home/u/db.sqlite", true},
		{"sqlite", "", false},
		{"dbhub", "/home/u/dbhub.toml", true},
		{"dbhub", "", false},
		{"dbhub", "dbhub.toml", false},
		{"context7", "", true},
		{"dart-flutter", "", true},
	}
	for _, tc := range cases {
		if err := validateParam(tc.slug, tc.param); (err == nil) != tc.ok {
			t.Errorf("validateParam(%q, %q) = %v, want ok=%v", tc.slug, tc.param, err, tc.ok)
		}
	}
}

// No agent accepting the registration fails the action; any success wins.
func TestAgentResults(t *testing.T) {
	allFail := &agentResults{stdout: io.Discard}
	allFail.done(AgentClaude, "Claude Code", "", errors.New("not found"))
	allFail.done(AgentAntigravity, "Antigravity", "", errors.New("permission denied"))
	if err := allFail.err("x"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("all failures should fail with the causes, got %v", err)
	}

	partial := &agentResults{stdout: io.Discard}
	partial.done(AgentClaude, "Claude Code", "", errors.New("not found"))
	partial.done(AgentAntigravity, "Antigravity", "ok", nil)
	if err := partial.err("x"); err != nil {
		t.Errorf("a partial success must not fail, got %v", err)
	}
	if !slices.Equal(partial.accepted, []string{AgentAntigravity}) {
		t.Errorf("accepted = %v", partial.accepted)
	}
}

func TestAntigravityConfig_SetKeepsOtherEntriesModeAndHTML(t *testing.T) {
	folder := t.TempDir()
	path := filepath.Join(folder, ".agents", "mcp_config.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	existing := `{"mcpServers":{"other":{"command":"x","env":{"TOKEN":"a<b&c"}}},"extra":1}`
	if err := os.WriteFile(path, []byte(existing), 0o640); err != nil {
		t.Fatal(err)
	}

	if err := antigravitySetServer("filesystem", false, folder, "filesystem", "npx", []string{"-y", "pkg", "/dir"}); err != nil {
		t.Fatalf("antigravitySetServer: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{`"other"`, `"filesystem"`, `"extra"`, `a<b&c`} {
		if !strings.Contains(text, want) {
			t.Errorf("config lost %s:\n%s", want, text)
		}
	}
	if info, _ := os.Stat(path); info.Mode().Perm() != 0o640 {
		t.Errorf("mode = %o, want the file's own 640", info.Mode().Perm())
	}
}

func TestAntigravityConfig_RemoveWithoutFileWritesNothing(t *testing.T) {
	folder := t.TempDir()
	if err := antigravityRemoveServer("filesystem", false, folder, "filesystem"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(folder, ".agents")); !os.IsNotExist(err) {
		t.Error("remove must not create .agents/ when there's nothing to remove")
	}
}

func TestAntigravityConfig_RejectsNonObjectServers(t *testing.T) {
	folder := t.TempDir()
	path := filepath.Join(folder, ".agents", "mcp_config.json")
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	if err := os.WriteFile(path, []byte(`{"mcpServers":[1,2]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := antigravitySetServer("sqlite", false, folder, "sqlite", "uvx", nil); err == nil {
		t.Fatal("a non-object mcpServers must be an error, not silently replaced")
	}
	if data, _ := os.ReadFile(path); string(data) != `{"mcpServers":[1,2]}` {
		t.Errorf("file must be left untouched, got %s", data)
	}
}

// Every automated catalog entry has a launch definition, and every npm/PyPI
// package is pinned to an exact version.
func TestCatalogue_GenericEntriesArePinned(t *testing.T) {
	for _, sv := range Catalogue {
		if sv.Manual || sv.Slug == "dart-flutter" {
			continue
		}
		g, ok := genericServers[sv.Slug]
		if !ok {
			t.Errorf("%s: no launch definition", sv.Slug)
			continue
		}
		if g.command == "gopls" {
			continue
		}
		args := strings.Join(g.args("/p"), " ")
		if !regexp.MustCompile(regexp.QuoteMeta(g.marker) + `@\d+\.\d+\.\d+( |$)`).MatchString(args) {
			t.Errorf("%s: package not pinned to an exact version: %s", sv.Slug, args)
		}
	}
}
