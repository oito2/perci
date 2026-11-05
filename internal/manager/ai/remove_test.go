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
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// allAgentsContext generates, in a new folder, a context with a standard
// that covers file types and one that doesn't, for every agent.
func allAgentsContext(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	active := []Model{modelByName(t, "Linux Bash"), modelByName(t, "Website")}
	if err := GenerateSharedFiles(dir, active, AgentIDs(), true, io.Discard); err != nil {
		t.Fatal(err)
	}
	return dir
}

// listFiles returns every file and link under dir, relative to it.
func listFiles(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(dir, func(path string, e os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !e.IsDir() {
			rel, _ := filepath.Rel(dir, path)
			out = append(out, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestRemoveContext_RemovesEverythingGenerated(t *testing.T) {
	dir := allAgentsContext(t)
	if err := RemoveContext(dir, io.Discard); err != nil {
		t.Fatal(err)
	}
	if got := listFiles(t, dir); !slices.Equal(got, []string{projectRulesPath}) {
		t.Errorf("left %v, want only %s", got, projectRulesPath)
	}
	if exists(t, filepath.Join(dir, ".claude")) {
		t.Error(".claude folder left behind")
	}

	// Without PROJECT.md nothing is left, not even .instructions.
	dir = allAgentsContext(t)
	if err := os.Remove(filepath.Join(dir, projectRulesPath)); err != nil {
		t.Fatal(err)
	}
	if err := RemoveContext(dir, io.Discard); err != nil {
		t.Fatal(err)
	}
	if entries, err := os.ReadDir(dir); err != nil || len(entries) != 0 {
		t.Errorf("left %v (%v)", entries, err)
	}
}

func TestRemoveContext_KeepsEditsAndUserFiles(t *testing.T) {
	dir := allAgentsContext(t)
	std := filepath.Join(dir, ".instructions", "WEBSITE.md")
	write(t, std, readFile(t, std)+"\nmine\n")
	agentsMD := filepath.Join(dir, "AGENTS.md")
	write(t, agentsMD, readFile(t, agentsMD)+"\n- Always use tabs.\n")
	settings := filepath.Join(dir, claudeSettingsPath)
	write(t, settings, strings.Replace(readFile(t, settings), `"deny": [`, `"deny": [
      "Read(secret.txt)",`, 1))
	write(t, filepath.Join(dir, openCodeConfig), `{"model": "x", "instructions": [".instructions/PROJECT.md"]}`)

	if err := RemoveContext(dir, io.Discard); err != nil {
		t.Fatal(err)
	}
	for _, kept := range []string{std, agentsMD, filepath.Join(dir, projectRulesPath)} {
		if !exists(t, kept) {
			t.Errorf("%s removed", kept)
		}
	}
	if exists(t, filepath.Join(dir, ".instructions", "BASH.md")) {
		t.Error("pristine standard kept")
	}
	if s := readFile(t, settings); !strings.Contains(s, "Read(secret.txt)") || strings.Count(s, "Read(") != 1 {
		t.Errorf("settings.json = %s", s)
	}
	if s := readFile(t, filepath.Join(dir, openCodeConfig)); strings.Contains(s, "PROJECT.md") || !strings.Contains(s, `"model"`) {
		t.Errorf("opencode.json = %s", s)
	}
}

func TestRemoveContext_OutdatedFilesGo(t *testing.T) {
	bash := modelByName(t, "Linux Bash")
	old := "# Old Bash standard\n"
	sum := sha256.Sum256([]byte(old))
	saved := templateHistory["BASH.md"]
	templateHistory["BASH.md"] = append(slices.Clone(saved), hex.EncodeToString(sum[:]))
	t.Cleanup(func() { templateHistory["BASH.md"] = saved })

	dir := t.TempDir()
	if err := GenerateSharedFiles(dir, []Model{bash}, []string{AgentClaude}, true, io.Discard); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, ".instructions", "BASH.md"), old)
	write(t, filepath.Join(dir, claudeRulesDir, "BASH.md"), claudeRule(bash, old))

	if err := RemoveContext(dir, io.Discard); err != nil {
		t.Fatal(err)
	}
	if got := listFiles(t, dir); !slices.Equal(got, []string{projectRulesPath}) {
		t.Errorf("left %v", got)
	}
}

func TestPreviewRemove(t *testing.T) {
	dir := allAgentsContext(t)
	before := listFiles(t, dir)
	changes, err := PreviewRemove(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(listFiles(t, dir), before) {
		t.Error("PreviewRemove changed the folder")
	}
	var removed []string
	for _, c := range changes {
		if c.Action != "remove" {
			t.Errorf("unexpected change %+v", c)
		}
		removed = append(removed, c.Path)
	}
	want := slices.DeleteFunc(slices.Clone(before), func(p string) bool { return p == projectRulesPath })
	slices.Sort(want)
	if !slices.Equal(removed, want) {
		t.Errorf("preview removes %v, want %v", removed, want)
	}
}
