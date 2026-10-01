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

package agentskills

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/oito2/perci/internal/executor"
)

// fakeNPX puts an npx on PATH that appends "<cwd>|<args>" to a log and,
// for `skills list`, prints listJSON (or exits listExit when non-zero).
func fakeNPX(t *testing.T, listJSON string, listExit int) (logPath string) {
	t.Helper()
	dir := t.TempDir()
	logPath = filepath.Join(dir, "npx.log")
	jsonPath := filepath.Join(dir, "list.json")
	if err := os.WriteFile(jsonPath, []byte(listJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	script := `#!/bin/sh
printf '%s|%s\n' "$PWD" "$*" >> "` + logPath + `"
if [ "$3" = "list" ]; then
  [ ` + strconv.Itoa(listExit) + ` -ne 0 ] && exit ` + strconv.Itoa(listExit) + `
  cat "` + jsonPath + `"
fi
`
	if err := os.WriteFile(filepath.Join(dir, "npx"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return logPath
}

// calls returns the logged npx invocations as "<cwd>|<args>" lines.
func calls(t *testing.T, logPath string) []string {
	t.Helper()
	b, err := os.ReadFile(logPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSpace(string(b)), "\n")
}

const agentArgs = "--agent claude-code --agent opencode --agent antigravity --agent codex"

func mustSkill(t *testing.T, slug string) Skill {
	t.Helper()
	sk, ok := BySlug(slug)
	if !ok {
		t.Fatalf("slug %q not in Catalogue", slug)
	}
	return sk
}

func TestCatalogue(t *testing.T) {
	seen := map[string]bool{}
	for _, sk := range Catalogue {
		if sk.Slug == "" || sk.Name == "" || sk.Repo == "" || sk.Arg == "" {
			t.Errorf("incomplete entry: %+v", sk)
		}
		if seen[sk.Slug] {
			t.Errorf("duplicate slug %q", sk.Slug)
		}
		seen[sk.Slug] = true
	}
	if _, ok := BySlug("does-not-exist"); ok {
		t.Error("BySlug found an unknown slug")
	}
}

func TestInstall(t *testing.T) {
	logPath := fakeNPX(t, "[]", 0)
	folder := t.TempDir()
	sk := mustSkill(t, "frontend-design")
	exe := &executor.Executor{}

	if err := Install(context.Background(), exe, io.Discard, sk, false, folder); err != nil {
		t.Fatal(err)
	}
	if err := Install(context.Background(), exe, io.Discard, sk, true, folder); err != nil {
		t.Fatal(err)
	}
	got := calls(t, logPath)
	want := "-y " + skillsCLI + " add https://github.com/anthropics/skills --skill frontend-design " + agentArgs + " --yes"
	if len(got) != 2 || got[0] != folder+"|"+want {
		t.Errorf("local call = %q, want %q", got, folder+"|"+want)
	}
	if len(got) == 2 && (!strings.HasSuffix(got[1], want+" --global") || strings.HasPrefix(got[1], folder+"|")) {
		t.Errorf("global call = %q (must add --global and not run in the folder)", got[1])
	}
}

func TestRemove_NamedSkill(t *testing.T) {
	logPath := fakeNPX(t, "[]", 0)
	folder := t.TempDir()
	if err := Remove(context.Background(), &executor.Executor{}, io.Discard, mustSkill(t, "code-review"), false, folder); err != nil {
		t.Fatal(err)
	}
	got := calls(t, logPath)
	want := folder + "|-y " + skillsCLI + " remove --skill code-review " + agentArgs + " --yes"
	if len(got) != 1 || got[0] != want {
		t.Errorf("calls = %q, want [%q]", got, want)
	}
}

// Regression: an Arg "*" entry ran `skills remove --skill '*'`, which
// removes every skill of those agents in the scope — not just the repo's.
func TestRemove_WildcardEntryRemovesOnlyItsRepo(t *testing.T) {
	list := `[
  {"name": "daisyui", "source": "saadeghi/daisyui"},
  {"name": "daisyui-extra", "source": "Saadeghi/DaisyUI"},
  {"name": "moodle-plugin", "source": "af1ah/moodle-plugin-skills"},
  {"name": "mine", "source": null},
  {"name": "-x", "source": "saadeghi/daisyui"}
]`
	logPath := fakeNPX(t, list, 0)
	if err := Remove(context.Background(), &executor.Executor{}, io.Discard, mustSkill(t, "daisyui"), true, ""); err != nil {
		t.Fatal(err)
	}
	got := calls(t, logPath)
	if len(got) != 2 {
		t.Fatalf("calls = %q", got)
	}
	if !strings.HasSuffix(got[0], "list --json --global") {
		t.Errorf("first call should list the global scope: %q", got[0])
	}
	want := "remove --skill daisyui --skill daisyui-extra " + agentArgs + " --yes --global"
	if !strings.HasSuffix(got[1], want) {
		t.Errorf("remove call = %q, want suffix %q", got[1], want)
	}
	if strings.Contains(got[1], "'*'") || strings.Contains(got[1], " * ") {
		t.Errorf("remove must never pass the wildcard: %q", got[1])
	}
}

func TestRemove_WildcardEntryNothingInstalled(t *testing.T) {
	logPath := fakeNPX(t, `[{"name": "other", "source": "a/b"}]`, 0)
	var out bytes.Buffer
	folder := t.TempDir()
	if err := Remove(context.Background(), &executor.Executor{}, &out, mustSkill(t, "dart-official"), false, folder); err != nil {
		t.Fatal(err)
	}
	got := calls(t, logPath)
	if len(got) != 1 || got[0] != folder+"|-y "+skillsCLI+" list --json" {
		t.Errorf("only the local list should run, got %q", got)
	}
	if !strings.Contains(out.String(), "nada a remover") {
		t.Errorf("expected a warning, got %q", out.String())
	}
}

// A listing that fails or isn't JSON refuses the removal — never falls
// back to the wildcard.
func TestRemove_WildcardEntryListFailureRefuses(t *testing.T) {
	for name, tc := range map[string]struct {
		json string
		exit int
	}{
		"list fails":   {"[]", 1},
		"invalid JSON": {"not json", 0},
	} {
		t.Run(name, func(t *testing.T) {
			logPath := fakeNPX(t, tc.json, tc.exit)
			err := Remove(context.Background(), &executor.Executor{}, io.Discard, mustSkill(t, "flutter-official"), true, "")
			if err == nil {
				t.Fatal("expected an error")
			}
			for _, c := range calls(t, logPath) {
				if strings.Contains(c, " remove ") {
					t.Errorf("remove must not run: %q", c)
				}
			}
		})
	}
}

func TestUpdate(t *testing.T) {
	logPath := fakeNPX(t, "[]", 0)
	folder := t.TempDir()
	sk := mustSkill(t, "godot-master")
	exe := &executor.Executor{}
	if err := Update(context.Background(), exe, io.Discard, sk, false, folder); err != nil {
		t.Fatal(err)
	}
	if err := Update(context.Background(), exe, io.Discard, sk, true, folder); err != nil {
		t.Fatal(err)
	}
	got := calls(t, logPath)
	base := "-y " + skillsCLI + " update godot-master --yes"
	if len(got) != 2 || got[0] != folder+"|"+base+" --project" || !strings.HasSuffix(got[1], base+" --global") {
		t.Errorf("calls = %q", got)
	}
}

func TestRepoKey(t *testing.T) {
	for in, want := range map[string]string{
		"saadeghi/daisyui":                         "saadeghi/daisyui",
		"https://github.com/Anthropics/Skills":     "anthropics/skills",
		"https://github.com/anthropics/skills.git": "anthropics/skills",
		"https://github.com/anthropics/skills/":    "anthropics/skills",
		" dart-lang/skills ":                       "dart-lang/skills",
	} {
		if got := repoKey(in); got != want {
			t.Errorf("repoKey(%q) = %q, want %q", in, got, want)
		}
	}
}

// Regression: `skills update '*'` matched a skill literally named "*" and
// updated nothing; wildcard entries now pass the installed names.
func TestUpdate_WildcardEntryUpdatesItsRepoSkills(t *testing.T) {
	list := `[{"name": "dart-a", "source": "dart-lang/skills"}, {"name": "dart-b", "source": "dart-lang/skills"}, {"name": "x", "source": "o/r"}]`
	logPath := fakeNPX(t, list, 0)
	folder := t.TempDir()
	if err := Update(context.Background(), &executor.Executor{}, io.Discard, mustSkill(t, "dart-official"), false, folder); err != nil {
		t.Fatal(err)
	}
	got := calls(t, logPath)
	want := folder + "|-y " + skillsCLI + " update dart-a dart-b --yes --project"
	if len(got) != 2 || got[1] != want {
		t.Errorf("calls = %q, want second %q", got, want)
	}
}

func TestUpdate_WildcardEntryNothingInstalledOrListFails(t *testing.T) {
	t.Run("nothing installed", func(t *testing.T) {
		logPath := fakeNPX(t, "[]", 0)
		var out bytes.Buffer
		if err := Update(context.Background(), &executor.Executor{}, &out, mustSkill(t, "daisyui"), true, ""); err != nil {
			t.Fatal(err)
		}
		if got := calls(t, logPath); len(got) != 1 || !strings.Contains(out.String(), "nada a atualizar") {
			t.Errorf("calls = %q, output = %q", got, out.String())
		}
	})
	t.Run("list fails", func(t *testing.T) {
		logPath := fakeNPX(t, "[]", 1)
		if err := Update(context.Background(), &executor.Executor{}, io.Discard, mustSkill(t, "daisyui"), true, ""); err == nil {
			t.Error("expected an error")
		}
		for _, c := range calls(t, logPath) {
			if strings.Contains(c, " update ") {
				t.Errorf("update must not run: %q", c)
			}
		}
	})
}
