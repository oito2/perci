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
	"context"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/oito2/perci/internal/executor"
)

func TestCreatedByPerci(t *testing.T) {
	for _, tc := range []struct {
		slug, command string
		args          []string
		want          bool
	}{
		{"filesystem", "npx", []string{"-y", filesystemServerPkg, "/p"}, true},
		{"filesystem", "npx", []string{"-y", "@modelcontextprotocol/server-filesystem@2020.1.1", "/p"}, true},
		{"filesystem", "npx", []string{"-y", "@modelcontextprotocol/server-filesystem", "/p"}, true},
		{"filesystem", "node", []string{"@modelcontextprotocol/server-filesystem"}, false},
		{"filesystem", "npx", []string{"-y", "some-other-server"}, false},
		{"filesystem", "", nil, false},
		{"sqlite", "uvx", []string{sqliteServerPkg, "--db-path", "/d"}, true},
		{"sqlite", "uvx", []string{"mcp-server-sqlite-evil"}, false},
		{"dart-flutter", "npx", []string{"-y", filesystemServerPkg}, false},
	} {
		if got := createdByPerci(tc.slug, tc.command, tc.args); got != tc.want {
			t.Errorf("createdByPerci(%s, %s, %v) = %v, want %v", tc.slug, tc.command, tc.args, got, tc.want)
		}
	}
}

func TestExpectedAgents(t *testing.T) {
	for _, tc := range []struct {
		slug   string
		global bool
		want   []string
	}{
		{"filesystem", true, []string{AgentClaude, AgentCodex, AgentAntigravity}},
		{"sqlite", false, []string{AgentClaude, AgentAntigravity}},
		{"dart-flutter", true, []string{AgentClaude, AgentCodex}},
		{"dart-flutter", false, []string{AgentClaude}},
	} {
		if got := ExpectedAgents(tc.slug, tc.global); !slices.Equal(got, tc.want) {
			t.Errorf("ExpectedAgents(%s, %v) = %v, want %v", tc.slug, tc.global, got, tc.want)
		}
	}
	if AgentLabel(AgentClaude) != "Claude" || AgentLabel(AgentCodex) != "Codex" || AgentLabel(AgentAntigravity) != "Antigravity" {
		t.Error("agent labels")
	}
}

// An Antigravity entry of the same name that Perci did not create is
// neither overwritten nor removed.
func TestAntigravity_ForeignEntryIsKept(t *testing.T) {
	folder := t.TempDir()
	path := filepath.Join(folder, ".agents", "mcp_config.json")
	foreign := `{"mcpServers":{"filesystem":{"command":"my-server","args":["--root","/"]}}}`
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(foreign), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := antigravitySetServer("filesystem", false, folder, "filesystem", "npx", []string{"-y", filesystemServerPkg, "/p"}); err == nil {
		t.Error("set over a foreign entry must fail")
	}
	if err := antigravityRemoveServer("filesystem", false, folder, "filesystem"); err == nil {
		t.Error("remove of a foreign entry must fail")
	}
	if data, _ := os.ReadFile(path); string(data) != foreign {
		t.Errorf("the foreign entry was touched: %s", data)
	}

	// Perci's own entry, whatever its version or parameter, is replaced.
	ours := `{"mcpServers":{"filesystem":{"command":"npx","args":["-y","@modelcontextprotocol/server-filesystem@1.0.0","/old"]}}}`
	if err := os.WriteFile(path, []byte(ours), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := antigravitySetServer("filesystem", false, folder, "filesystem", "npx", []string{"-y", filesystemServerPkg, "/new"}); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(path); !strings.Contains(string(data), "/new") || strings.Contains(string(data), "/old") {
		t.Errorf("own entry not replaced: %s", data)
	}
}

func TestAntigravity_WritesThroughSymlinkAndKeepsNumbers(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "dotfiles", "mcp_config.json")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte(`{"big":12345678901234567890,"ratio":1.10,"mcpServers":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	folder := filepath.Join(dir, "proj")
	link := filepath.Join(folder, ".agents", "mcp_config.json")
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	if err := antigravitySetServer("sqlite", false, folder, "sqlite", "uvx", []string{sqliteServerPkg}); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("the symlink was replaced (err=%v)", err)
	}
	data, _ := os.ReadFile(target)
	for _, want := range []string{`12345678901234567890`, `1.10`, `"sqlite"`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("target lost %s:\n%s", want, data)
		}
	}
}

func TestAntigravity_RejectsNullDocument(t *testing.T) {
	folder := t.TempDir()
	path := filepath.Join(folder, ".agents", "mcp_config.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("null"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := antigravitySetServer("sqlite", false, folder, "sqlite", "uvx", nil); err == nil {
		t.Fatal("a null document must be an error, not a panic or a rewrite")
	}
	if data, _ := os.ReadFile(path); string(data) != "null" {
		t.Errorf("file touched: %s", data)
	}
}

// `claude mcp remove` and `codex mcp remove` only run for entries Perci
// created.
func TestRemove_ForeignOrMissingEntriesAreNotRemovedByName(t *testing.T) {
	t.Run("foreign entry", func(t *testing.T) {
		env := newAgentsEnv(t)
		env.registerEverywhere(t, "filesystem", true, "", "my-server", "--root", "/")
		if err := Remove(context.Background(), &executor.Executor{}, io.Discard, "filesystem", true, ""); err != nil {
			t.Fatal(err)
		}
		for _, c := range argsOf(env.calls(t)) {
			if strings.Contains(c, "remove") {
				t.Errorf("a foreign entry was removed: %q", c)
			}
		}
	})
	t.Run("not registered", func(t *testing.T) {
		env := newAgentsEnv(t)
		if err := Remove(context.Background(), &executor.Executor{}, io.Discard, "filesystem", true, ""); err != nil {
			t.Fatal(err)
		}
		equalCalls(t, argsOf(env.calls(t)), []string{"codex mcp get filesystem --json"})
	})
	t.Run("local scope reads the project", func(t *testing.T) {
		env := newAgentsEnv(t)
		folder := t.TempDir()
		env.registerEverywhere(t, "sqlite", false, folder, "uvx", sqliteServerPkg, "--db-path", "/d")
		if err := Remove(context.Background(), &executor.Executor{}, io.Discard, "sqlite", false, folder); err != nil {
			t.Fatal(err)
		}
		equalCalls(t, argsOf(env.calls(t)), []string{"claude mcp remove sqlite --scope local"})
	})
}

// Codex's `mcp add` replaces an entry silently, so a foreign one is refused
// before it runs.
func TestInstall_CodexForeignEntryIsNotReplaced(t *testing.T) {
	env := newAgentsEnv(t)
	writeJSONFile(t, filepath.Join(env.home, "codex-filesystem.json"), map[string]any{
		"transport": map[string]any{"command": "my-server", "args": []string{"--root", "/"}},
	})
	agents, err := Install(context.Background(), &executor.Executor{}, io.Discard, "filesystem", true, "", "/p")
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(agents, AgentCodex) {
		t.Errorf("Codex must not count as accepted: %v", agents)
	}
	for _, c := range argsOf(env.calls(t)) {
		if strings.HasPrefix(c, "codex mcp add") {
			t.Errorf("codex mcp add ran over a foreign entry: %q", c)
		}
	}
}

// Launcher missing: nothing is registered or removed.
func TestInstallUpdate_RefuseWithoutLauncher(t *testing.T) {
	for slug, tool := range map[string]string{"filesystem": "npx", "dbhub": "npx", "gopls": "gopls"} {
		env := newAgentsEnv(t)
		if err := os.Remove(filepath.Join(env.bin, tool)); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		bash, err := lookBash()
		if err != nil {
			t.Skip("bash not available")
		}
		if err := os.Symlink(bash, filepath.Join(env.bin, "bash")); err != nil {
			t.Fatal(err)
		}
		t.Setenv("PATH", env.bin)

		_, err = Install(context.Background(), &executor.Executor{}, io.Discard, slug, true, "", "/p/x")
		if err == nil || !strings.Contains(err.Error(), tool+" não encontrado") {
			t.Errorf("%s install: err = %v", slug, err)
		}
		_, err = Update(context.Background(), &executor.Executor{}, io.Discard, slug, true, "", "/p/x")
		if err == nil || !strings.Contains(err.Error(), tool+" não encontrado") {
			t.Errorf("%s update: err = %v", slug, err)
		}
		if calls := env.calls(t); len(calls) != 0 {
			t.Errorf("%s: nothing should run, got %q", slug, calls)
		}
	}
}

func lookBash() (string, error) {
	p, ok := executor.New(nil, nil).Which("bash")
	if !ok {
		return "", os.ErrNotExist
	}
	return p, nil
}

// The Codex marketplace is added at a fixed commit.
func TestDartFlutter_CodexMarketplaceIsPinned(t *testing.T) {
	if !regexp.MustCompile(`^flutter/agent-plugins@[0-9a-f]{40}$`).MatchString(dartFlutterCodexMarket) {
		t.Errorf("codex marketplace = %q, want owner/repo@<full sha>", dartFlutterCodexMarket)
	}
}
