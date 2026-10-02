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
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oito2/perci/internal/executor"
)

// agentsEnv gives the test its own HOME and fake claude/codex binaries
// that append "<name> <cwd>|<args>" to a log; FAIL_CLAUDE/FAIL_CODEX=1
// make them exit 1.
type agentsEnv struct {
	home, log string
}

func newAgentsEnv(t *testing.T) agentsEnv {
	t.Helper()
	home := t.TempDir()
	bin := t.TempDir()
	log := filepath.Join(bin, "calls.log")
	for _, name := range []string{"claude", "codex"} {
		fail := "FAIL_" + strings.ToUpper(name)
		script := "#!/bin/sh\nprintf '%s %s|%s\\n' " + name + " \"$PWD\" \"$*\" >> '" + log + "'\n" +
			"[ \"${" + fail + ":-}\" = 1 ] && exit 1\nexit 0\n"
		if err := os.WriteFile(filepath.Join(bin, name), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", home)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return agentsEnv{home: home, log: log}
}

func (e agentsEnv) calls(t *testing.T) []string {
	t.Helper()
	b, err := os.ReadFile(e.log)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSpace(string(b)), "\n")
}

// argsOf strips the "<name> <cwd>|" prefix of each call.
func argsOf(calls []string) []string {
	out := make([]string, len(calls))
	for i, c := range calls {
		name, rest, _ := strings.Cut(c, " ")
		_, args, _ := strings.Cut(rest, "|")
		out[i] = name + " " + args
	}
	return out
}

func readServers(t *testing.T, path string) map[string]any {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	servers, _ := doc["mcpServers"].(map[string]any)
	return servers
}

func equalCalls(t *testing.T, got, want []string) {
	t.Helper()
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("calls:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestInstall_FilesystemGlobal(t *testing.T) {
	env := newAgentsEnv(t)
	if err := Install(context.Background(), &executor.Executor{}, io.Discard, "filesystem", true, "", "/data/proj"); err != nil {
		t.Fatal(err)
	}
	equalCalls(t, argsOf(env.calls(t)), []string{
		"claude mcp add --scope user filesystem -- npx -y " + filesystemServerPkg + " /data/proj",
		"codex mcp add filesystem -- npx -y " + filesystemServerPkg + " /data/proj",
	})
	servers := readServers(t, filepath.Join(env.home, ".gemini", "config", "mcp_config.json"))
	entry, _ := servers["filesystem"].(map[string]any)
	if entry["command"] != "npx" {
		t.Errorf("Antigravity entry = %v", entry)
	}
}

// Local scope: Claude Code runs in the project folder, Codex is skipped,
// Antigravity writes .agents/.
func TestInstall_SQLiteLocal(t *testing.T) {
	env := newAgentsEnv(t)
	folder := t.TempDir()
	if err := Install(context.Background(), &executor.Executor{}, io.Discard, "sqlite", false, folder, "/data/db.sqlite"); err != nil {
		t.Fatal(err)
	}
	calls := env.calls(t)
	equalCalls(t, calls, []string{
		"claude " + folder + "|mcp add --scope local sqlite -- uvx " + sqliteServerPkg + " --db-path /data/db.sqlite",
	})
	if _, ok := readServers(t, filepath.Join(folder, ".agents", "mcp_config.json"))["sqlite"]; !ok {
		t.Error("sqlite not registered in the project's Antigravity config")
	}
}

func TestInstall_FailsOnlyWhenNoAgentAccepted(t *testing.T) {
	t.Run("partial success", func(t *testing.T) {
		newAgentsEnv(t)
		t.Setenv("FAIL_CLAUDE", "1")
		if err := Install(context.Background(), &executor.Executor{}, io.Discard, "filesystem", true, "", "/p"); err != nil {
			t.Errorf("one agent accepted, err = %v", err)
		}
	})
	t.Run("nobody accepted", func(t *testing.T) {
		env := newAgentsEnv(t)
		t.Setenv("FAIL_CLAUDE", "1")
		t.Setenv("FAIL_CODEX", "1")
		cfg := filepath.Join(env.home, ".gemini", "config", "mcp_config.json")
		if err := os.MkdirAll(filepath.Dir(cfg), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(cfg, []byte(`{"mcpServers": []}`), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := Install(context.Background(), &executor.Executor{}, io.Discard, "filesystem", true, "", "/p"); err == nil {
			t.Error("expected an error")
		}
	})
}

func TestRemove_GenericKeepsOtherEntries(t *testing.T) {
	env := newAgentsEnv(t)
	cfg := filepath.Join(env.home, ".gemini", "config", "mcp_config.json")
	if err := os.MkdirAll(filepath.Dir(cfg), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg, []byte(`{"mcpServers": {"sqlite": {}, "mine": {"command": "x"}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	// Failures stay warnings: removal still succeeds.
	t.Setenv("FAIL_CODEX", "1")
	if err := Remove(context.Background(), &executor.Executor{}, io.Discard, "sqlite", true, ""); err != nil {
		t.Fatal(err)
	}
	equalCalls(t, argsOf(env.calls(t)), []string{
		"claude mcp remove sqlite --scope user",
		"codex mcp remove sqlite",
	})
	servers := readServers(t, cfg)
	if _, ok := servers["sqlite"]; ok {
		t.Error("sqlite still registered")
	}
	if _, ok := servers["mine"]; !ok {
		t.Error("an unrelated entry was removed")
	}
}

func TestDartFlutter_InstallRemoveUpdate(t *testing.T) {
	ref := dartFlutterPluginRef
	t.Run("install global", func(t *testing.T) {
		env := newAgentsEnv(t)
		if err := Install(context.Background(), &executor.Executor{}, io.Discard, "dart-flutter", true, "", ""); err != nil {
			t.Fatal(err)
		}
		equalCalls(t, argsOf(env.calls(t)), []string{
			"claude plugin marketplace add " + dartFlutterMarketplace,
			"claude plugin install " + ref + " --scope user -y",
			"codex plugin marketplace add " + dartFlutterCodexMarket,
			"codex plugin add " + ref,
		})
	})
	t.Run("install local skips Codex", func(t *testing.T) {
		env := newAgentsEnv(t)
		folder := t.TempDir()
		if err := Install(context.Background(), &executor.Executor{}, io.Discard, "dart-flutter", false, folder, ""); err != nil {
			t.Fatal(err)
		}
		for _, c := range env.calls(t) {
			if strings.HasPrefix(c, "codex ") {
				t.Errorf("Codex must not run at local scope: %q", c)
			}
		}
	})
	// Codex is given the PLUGIN@MARKETPLACE reference, not a bare name.
	t.Run("remove global", func(t *testing.T) {
		env := newAgentsEnv(t)
		if err := Remove(context.Background(), &executor.Executor{}, io.Discard, "dart-flutter", true, ""); err != nil {
			t.Fatal(err)
		}
		equalCalls(t, argsOf(env.calls(t)), []string{
			"claude plugin uninstall " + ref + " --scope user -y",
			"codex plugin remove " + ref,
		})
	})
	t.Run("update global", func(t *testing.T) {
		env := newAgentsEnv(t)
		if err := Update(context.Background(), &executor.Executor{}, io.Discard, "dart-flutter", true, "", ""); err != nil {
			t.Fatal(err)
		}
		equalCalls(t, argsOf(env.calls(t)), []string{
			"claude plugin update " + ref + " --scope user",
			"codex plugin remove " + ref,
			"codex plugin add " + ref,
		})
	})
	t.Run("install fails when neither CLI took it", func(t *testing.T) {
		newAgentsEnv(t)
		t.Setenv("FAIL_CLAUDE", "1")
		t.Setenv("FAIL_CODEX", "1")
		if err := Install(context.Background(), &executor.Executor{}, io.Discard, "dart-flutter", true, "", ""); err == nil {
			t.Error("expected an error")
		}
	})
}

func TestUpdate_Generic(t *testing.T) {
	t.Run("missing parameter removes nothing", func(t *testing.T) {
		env := newAgentsEnv(t)
		if err := Update(context.Background(), &executor.Executor{}, io.Discard, "filesystem", true, "", ""); err == nil {
			t.Fatal("expected an error")
		}
		if calls := env.calls(t); len(calls) != 0 {
			t.Errorf("nothing should run, got %q", calls)
		}
	})
	t.Run("remove then install", func(t *testing.T) {
		env := newAgentsEnv(t)
		if err := Update(context.Background(), &executor.Executor{}, io.Discard, "filesystem", true, "", "/p"); err != nil {
			t.Fatal(err)
		}
		got := argsOf(env.calls(t))
		if len(got) != 4 || !strings.HasPrefix(got[0], "claude mcp remove") || !strings.HasPrefix(got[2], "claude mcp add") {
			t.Errorf("calls = %q", got)
		}
	})
	t.Run("reinstall rejected everywhere", func(t *testing.T) {
		env := newAgentsEnv(t)
		t.Setenv("FAIL_CLAUDE", "1")
		t.Setenv("FAIL_CODEX", "1")
		cfg := filepath.Join(env.home, ".gemini", "config", "mcp_config.json")
		if err := os.MkdirAll(filepath.Dir(cfg), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(cfg, []byte(`{"mcpServers": "broken"}`), 0o600); err != nil {
			t.Fatal(err)
		}
		err := Update(context.Background(), &executor.Executor{}, io.Discard, "sqlite", true, "", "/db.sqlite")
		if !errors.Is(err, ErrNotReinstalled) {
			t.Errorf("err = %v, want ErrNotReinstalled", err)
		}
	})
}

func TestUnsupportedSlugs(t *testing.T) {
	env := newAgentsEnv(t)
	exe := &executor.Executor{}
	for _, slug := range []string{"godot-studio", "nope"} {
		if err := Install(context.Background(), exe, io.Discard, slug, true, "", ""); err == nil {
			t.Errorf("Install(%s): expected an error", slug)
		}
		if err := Remove(context.Background(), exe, io.Discard, slug, true, ""); err == nil {
			t.Errorf("Remove(%s): expected an error", slug)
		}
		if err := Update(context.Background(), exe, io.Discard, slug, true, "", ""); err == nil {
			t.Errorf("Update(%s): expected an error", slug)
		}
	}
	if calls := env.calls(t); len(calls) != 0 {
		t.Errorf("nothing should run, got %q", calls)
	}
	if s, ok := BySlug("godot-studio"); !ok || !s.Manual {
		t.Error("godot-studio should be a manual entry")
	}
	if _, ok := BySlug("nope"); ok {
		t.Error("BySlug found an unknown slug")
	}
}
