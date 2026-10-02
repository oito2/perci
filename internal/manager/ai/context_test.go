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
	"bytes"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestDetectActiveModels_EmptyDir(t *testing.T) {
	t.Chdir(t.TempDir())

	got := DetectActiveModels("")
	for _, m := range models {
		if got[m.Name] {
			t.Errorf("expected %q not present in empty directory", m.Name)
		}
	}
}

func TestDetectActiveModels_WithInstructionFile(t *testing.T) {
	tmp := t.TempDir()
	t.Chdir(tmp)

	if err := os.MkdirAll(".instructions", 0o755); err != nil {
		t.Fatal(err)
	}
	// Simulate Go model instruction file present.
	goFile := filepath.Join(".instructions", filepath.Base(models[0].Instruction))
	if err := os.WriteFile(goFile, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := DetectActiveModels("")
	if !got[models[0].Name] {
		t.Errorf("expected %q to be detected", models[0].Name)
	}
	for _, m := range models[1:] {
		if got[m.Name] {
			t.Errorf("expected %q not to be detected", m.Name)
		}
	}
}

func TestDetectMoodleVars_NotFound(t *testing.T) {
	tmp := t.TempDir()

	if _, ok := detectMoodleVars(tmp); ok {
		t.Error("expected ok=false when version.php is missing")
	}
}

func TestDetectMoodleVars_RootVersionPHP(t *testing.T) {
	tmp := t.TempDir()
	content := "<?php\n$version  = 2026042001.04;\n$release  = '5.2.1+ (Build: 20260630)';\n"
	if err := os.WriteFile(filepath.Join(tmp, "version.php"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	vars, ok := detectMoodleVars(tmp)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if vars["MOODLE_VERSION"] != "5.2" {
		t.Errorf("MOODLE_VERSION = %q, want %q", vars["MOODLE_VERSION"], "5.2")
	}
	if vars["MOODLE_FULLVERSION"] != "2026042001.04" {
		t.Errorf("MOODLE_FULLVERSION = %q, want %q", vars["MOODLE_FULLVERSION"], "2026042001.04")
	}
	if vars["MOODLE_PATH"] != tmp {
		t.Errorf("MOODLE_PATH = %q, want %q", vars["MOODLE_PATH"], tmp)
	}
}

func TestDetectMoodleVars_PublicVersionPHP(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "public"), 0o755); err != nil {
		t.Fatal(err)
	}
	content := "<?php\n$version  = 2025083100.00;\n$release  = '4.5.2 (Build: 20250831)';\n"
	if err := os.WriteFile(filepath.Join(tmp, "public", "version.php"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	vars, ok := detectMoodleVars(tmp)
	if !ok {
		t.Fatal("expected ok=true when version.php is under public/")
	}
	if vars["MOODLE_VERSION"] != "4.5" {
		t.Errorf("MOODLE_VERSION = %q, want %q", vars["MOODLE_VERSION"], "4.5")
	}
	// MOODLE_PATH must be the directory the command ran in, not the public/ subdir.
	if vars["MOODLE_PATH"] != tmp {
		t.Errorf("MOODLE_PATH = %q, want %q (cwd, not public/)", vars["MOODLE_PATH"], tmp)
	}
}

func TestGenerateSharedFiles_CreatesExpectedFiles(t *testing.T) {
	t.Chdir(t.TempDir())

	active := []Model{models[0]} // Go
	if err := GenerateSharedFiles("", active, false, io.Discard); err != nil {
		t.Fatalf("GenerateSharedFiles: %v", err)
	}

	for _, name := range []string{"CLAUDE.md", "AGENTS.md"} {
		if _, err := os.Stat(name); err != nil {
			t.Errorf("expected %s to be created: %v", name, err)
		}
	}
	for _, name := range []string{".aiexclude", ".claudeignore"} {
		if _, err := os.Stat(name); err != nil {
			t.Errorf("expected %s to be created: %v", name, err)
		}
	}
	for _, name := range []string{"GEMINI.md", ".windsurfrules", ".cursorrules", ".geminiignore"} {
		if _, err := os.Stat(name); err == nil {
			t.Errorf("expected %s not to be created — no longer a generation target", name)
		}
	}
}

func TestGenerateSharedFiles_ClaudeIsStubOnly(t *testing.T) {
	t.Chdir(t.TempDir())

	active := []Model{models[0]} // Linux Bash
	if err := GenerateSharedFiles("", active, false, io.Discard); err != nil {
		t.Fatalf("GenerateSharedFiles: %v", err)
	}

	data, err := os.ReadFile("CLAUDE.md")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte("BASH.md")) {
		t.Error("CLAUDE.md should be a stub and must not reference BASH.md")
	}
	// Claude Code only follows "@path" imports on their own line — a plain
	// Markdown link would leave AGENTS.md (and every .instructions/*.md it
	// imports) unloaded.
	if !regexp.MustCompile(`(?m)^@AGENTS\.md$`).Match(data) {
		t.Error("CLAUDE.md should import AGENTS.md with an @AGENTS.md line")
	}
}

func TestGenerateSharedFiles_ContainsInstructionReference(t *testing.T) {
	t.Chdir(t.TempDir())

	active := []Model{models[0]} // Linux Bash — instruction: templates/instructions/BASH.md
	if err := GenerateSharedFiles("", active, false, io.Discard); err != nil {
		t.Fatalf("GenerateSharedFiles: %v", err)
	}

	data, err := os.ReadFile("AGENTS.md")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte("BASH.md")) {
		t.Error("AGENTS.md should reference BASH.md")
	}
}

func TestGenerateSharedFiles_MultipleModels(t *testing.T) {
	t.Chdir(t.TempDir())

	active := []Model{models[0], models[1]} // Linux Bash + Moodle
	if err := GenerateSharedFiles("", active, false, io.Discard); err != nil {
		t.Fatalf("GenerateSharedFiles: %v", err)
	}

	data, err := os.ReadFile("AGENTS.md")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte("BASH.md")) {
		t.Error("AGENTS.md should reference BASH.md")
	}
	if !bytes.Contains(data, []byte("MOODLE.md")) {
		t.Error("AGENTS.md should reference MOODLE.md")
	}
}

func TestGenerateSharedFiles_EmptySelection(t *testing.T) {
	t.Chdir(t.TempDir())

	if err := GenerateSharedFiles("", nil, false, io.Discard); err != nil {
		t.Fatalf("GenerateSharedFiles: %v", err)
	}

	data, err := os.ReadFile("AGENTS.md")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte("Language-Specific Standards")) {
		t.Error("AGENTS.md should not have a Language-Specific Standards section when no model is selected")
	}
}

func TestGenerateSharedFiles_WritesInstructionFiles(t *testing.T) {
	t.Chdir(t.TempDir())

	active := []Model{models[0], models[1]} // Linux Bash + Moodle
	if err := GenerateSharedFiles("", active, false, io.Discard); err != nil {
		t.Fatalf("GenerateSharedFiles: %v", err)
	}

	for _, m := range active {
		dest := filepath.Join(".instructions", filepath.Base(m.Instruction))
		if _, err := os.Stat(dest); err != nil {
			t.Errorf("expected %s to be created by GenerateSharedFiles: %v", dest, err)
		}
	}
}

// TestGenerateSharedFiles_RemovesStaleInstructionForDeselectedModel
// checks that unchecking a previously-active model removes its
// .instructions/*.md: GenerateSharedFiles diffs DetectActiveModels against
// the newly selected list and removes what fell off.
func TestGenerateSharedFiles_RemovesStaleInstructionForDeselectedModel(t *testing.T) {
	t.Chdir(t.TempDir())

	// First run with both active — both instruction files get created.
	both := []Model{models[0], models[1]} // Linux Bash + Moodle
	if err := GenerateSharedFiles("", both, false, io.Discard); err != nil {
		t.Fatalf("GenerateSharedFiles (both): %v", err)
	}
	bashDest := filepath.Join(".instructions", filepath.Base(models[0].Instruction))
	moodleDest := filepath.Join(".instructions", filepath.Base(models[1].Instruction))
	if _, err := os.Stat(bashDest); err != nil {
		t.Fatalf("setup: expected %s to exist: %v", bashDest, err)
	}
	if _, err := os.Stat(moodleDest); err != nil {
		t.Fatalf("setup: expected %s to exist: %v", moodleDest, err)
	}

	// Second run deselecting Moodle — only Linux Bash stays active.
	onlyBash := []Model{models[0]}
	if err := GenerateSharedFiles("", onlyBash, false, io.Discard); err != nil {
		t.Fatalf("GenerateSharedFiles (only bash): %v", err)
	}

	if _, err := os.Stat(bashDest); err != nil {
		t.Errorf("expected %s to still exist (still selected): %v", bashDest, err)
	}
	if _, err := os.Stat(moodleDest); !os.IsNotExist(err) {
		t.Errorf("expected %s to be removed (deselected), stat err = %v", moodleDest, err)
	}
}

func TestGenerateSharedFiles_MoodleVarsSubstituted(t *testing.T) {
	tmp := t.TempDir()
	t.Chdir(tmp)

	content := "<?php\n$version  = 2026042001.04;\n$release  = '5.2.1+ (Build: 20260630)';\n"
	if err := os.WriteFile(filepath.Join(tmp, "version.php"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	var moodle Model
	for _, m := range models {
		if m.Name == "Moodle" {
			moodle = m
		}
	}

	// interactive=false: detectMoodleVars should still be tried first; only the
	// "not found" fallback differs between interactive and non-interactive.
	if err := GenerateSharedFiles("", []Model{moodle}, false, io.Discard); err != nil {
		t.Fatalf("GenerateSharedFiles: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(".instructions", "MOODLE.md"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte("{{MOODLE_VERSION}}")) {
		t.Error("MOODLE_VERSION placeholder should have been substituted")
	}
	if !bytes.Contains(data, []byte("5.2")) {
		t.Error("expected substituted MOODLE_VERSION (5.2) in MOODLE.md")
	}
}

func TestGenerateSharedFiles_MoodleVarsMissing_NonInteractive(t *testing.T) {
	t.Chdir(t.TempDir())

	var moodle Model
	for _, m := range models {
		if m.Name == "Moodle" {
			moodle = m
		}
	}

	var out strings.Builder
	if err := GenerateSharedFiles("", []Model{moodle}, false, &out); err != nil {
		t.Fatalf("GenerateSharedFiles: %v", err)
	}

	// The warning reaches the GUI's terminal.
	if !strings.Contains(out.String(), "version.php não encontrado") {
		t.Errorf("expected a warning when version.php is missing, got %q", out.String())
	}

	data, err := os.ReadFile(filepath.Join(".instructions", "MOODLE.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte("{{MOODLE_VERSION}}")) {
		t.Error("expected MOODLE_VERSION placeholder to remain literal when version.php can't be found")
	}
}

func TestWriteInstruction_CreatesFile(t *testing.T) {
	t.Chdir(t.TempDir())

	m := models[0] // Go
	if err := WriteInstruction("", m, false, io.Discard, nil); err != nil {
		t.Fatalf("WriteInstruction: %v", err)
	}

	dest := filepath.Join(".instructions", filepath.Base(m.Instruction))
	if _, err := os.Stat(dest); err != nil {
		t.Errorf("expected %s to be created: %v", dest, err)
	}
}

func modelByName(t *testing.T, name string) Model {
	t.Helper()
	for _, m := range models {
		if m.Name == name {
			return m
		}
	}
	t.Fatalf("model %q not in catalog", name)
	return Model{}
}

func TestModels_InstructionsEmbeddedAndRequiresValid(t *testing.T) {
	names := make(map[string]bool, len(models))
	files := make(map[string]string, len(models))
	for _, m := range models {
		names[m.Name] = true
		if _, err := readTpl(m.Instruction); err != nil {
			t.Errorf("%s: %v", m.Name, err)
		}
		for _, base := range append([]string{filepath.Base(m.Instruction)}, m.Legacy...) {
			if other, dup := files[base]; dup {
				t.Errorf("%s and %s share instruction filename %s — DetectActiveModels matches by filename", other, m.Name, base)
			}
			files[base] = m.Name
		}
	}
	for _, m := range models {
		if strings.TrimSpace(m.AppliesTo) == "" {
			t.Errorf("%s has no AppliesTo — agents without @ imports need it to know when to read the file", m.Name)
		}
		if m.Requires != "" && !names[m.Requires] {
			t.Errorf("%s requires unknown model %q", m.Name, m.Requires)
		}
	}
}

func TestWithRequired(t *testing.T) {
	base := modelByName(t, docsBase)
	mcp := modelByName(t, "Documentação: Servidor MCP")
	web := modelByName(t, "Documentação: Aplicação Web")
	bash := modelByName(t, "Linux Bash")

	got, added := withRequired([]Model{bash, mcp, web})
	if len(added) != 1 || added[0] != docsBase {
		t.Errorf("added = %v, want [%s]", added, docsBase)
	}
	var gotNames []string
	for _, m := range got {
		gotNames = append(gotNames, m.Name)
	}
	want := []string{bash.Name, base.Name, mcp.Name, web.Name} // catalog order
	if strings.Join(gotNames, "|") != strings.Join(want, "|") {
		t.Errorf("resolved = %v, want %v", gotNames, want)
	}

	if _, added := withRequired([]Model{base, mcp}); len(added) != 0 {
		t.Errorf("base already selected: added = %v, want none", added)
	}
	if _, added := withRequired([]Model{bash}); len(added) != 0 {
		t.Errorf("no complements: added = %v, want none", added)
	}
}

func TestGenerateSharedFiles_ComplementPullsInBase(t *testing.T) {
	t.Chdir(t.TempDir())

	var out bytes.Buffer
	active := []Model{modelByName(t, "Documentação: Plugin Moodle")}
	if err := GenerateSharedFiles("", active, false, &out); err != nil {
		t.Fatalf("GenerateSharedFiles: %v", err)
	}

	for _, name := range []string{"PROJECT-DOCUMENTATION.md", "PROJECT-DOCUMENTATION-MOODLE.md"} {
		if _, err := os.Stat(filepath.Join(".instructions", name)); err != nil {
			t.Errorf("expected .instructions/%s: %v", name, err)
		}
	}
	data, err := os.ReadFile("AGENTS.md")
	if err != nil {
		t.Fatal(err)
	}
	baseIdx := bytes.Index(data, []byte("@.instructions/PROJECT-DOCUMENTATION.md"))
	complIdx := bytes.Index(data, []byte("@.instructions/PROJECT-DOCUMENTATION-MOODLE.md"))
	if baseIdx < 0 || complIdx < 0 || baseIdx > complIdx {
		t.Errorf("AGENTS.md should reference the base before the complement (base at %d, complement at %d)", baseIdx, complIdx)
	}
	if !strings.Contains(out.String(), "incluído automaticamente") {
		t.Errorf("expected an auto-include notice, got %q", out.String())
	}
}

func TestLegacyInstruction_DetectedAndMigratedOnOverwrite(t *testing.T) {
	t.Chdir(t.TempDir())

	legacy := filepath.Join(".instructions", "DOCUMENTATION-SITE.md")
	current := filepath.Join(".instructions", "PROJECT-DOCUMENTATION.md")
	if err := os.MkdirAll(".instructions", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	if !DetectActiveModels("")[docsBase] {
		t.Fatalf("%s should be detected as active from its legacy filename", docsBase)
	}

	base := modelByName(t, docsBase)

	// Without overwrite, the legacy copy (possibly customized) is left alone.
	if err := WriteInstruction("", base, false, io.Discard, nil); err != nil {
		t.Fatalf("WriteInstruction: %v", err)
	}
	if _, err := os.Stat(legacy); err != nil {
		t.Errorf("legacy file should survive a non-overwrite run: %v", err)
	}

	if err := GenerateSharedFiles("", []Model{base}, true, io.Discard); err != nil {
		t.Fatalf("GenerateSharedFiles: %v", err)
	}
	if _, err := os.Stat(current); err != nil {
		t.Errorf("expected %s after migration: %v", current, err)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Errorf("legacy %s should be removed after an overwrite run, stat err = %v", legacy, err)
	}
}

func TestLegacyInstruction_RemovedWhenModelDeselected(t *testing.T) {
	t.Chdir(t.TempDir())

	legacy := filepath.Join(".instructions", "DOCUMENTATION-SITE.md")
	if err := os.MkdirAll(".instructions", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := GenerateSharedFiles("", nil, true, io.Discard); err != nil {
		t.Fatalf("GenerateSharedFiles: %v", err)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Errorf("legacy %s should be removed when its model is deselected, stat err = %v", legacy, err)
	}
}

func TestGenerateSharedFiles_StandardsIndexIsAgentAgnostic(t *testing.T) {
	t.Chdir(t.TempDir())

	bash := modelByName(t, "Linux Bash")
	if err := GenerateSharedFiles("", []Model{bash}, false, io.Discard); err != nil {
		t.Fatalf("GenerateSharedFiles: %v", err)
	}
	data, err := os.ReadFile("AGENTS.md")
	if err != nil {
		t.Fatal(err)
	}
	// Codex has no import mechanism: the explicit read instruction and the
	// per-entry "when it applies" text are what make it open the file.
	if !bytes.Contains(data, []byte("read the whole file")) {
		t.Error("AGENTS.md should tell agents to read the applicable standard before starting")
	}
	want := "\n- " + bash.AppliesTo + ": @.instructions/BASH.md"
	if !bytes.Contains(data, []byte(want)) {
		t.Errorf("AGENTS.md should contain entry %q", want)
	}
}

// Unchecking a model deletes its instruction file only if nobody edited it.
func TestGenerateSharedFiles_DeselectKeepsEditedInstruction(t *testing.T) {
	var bash Model
	for _, m := range models {
		if m.Name == "Linux Bash" {
			bash = m
		}
	}
	if bash.Name == "" {
		t.Skip("Linux Bash model not in catalog")
	}
	path := filepath.Join(".instructions", filepath.Base(bash.Instruction))

	for _, tc := range []struct {
		name   string
		edit   bool
		exists bool
	}{{"pristine is removed", false, false}, {"edited is kept", true, true}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			if err := GenerateSharedFiles("", []Model{bash}, true, io.Discard); err != nil {
				t.Fatal(err)
			}
			if tc.edit {
				if err := os.WriteFile(path, []byte("# my own rules\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			var out strings.Builder
			if err := GenerateSharedFiles("", nil, true, &out); err != nil {
				t.Fatal(err)
			}
			_, err := os.Stat(path)
			if exists := err == nil; exists != tc.exists {
				t.Errorf("%s exists = %v, want %v (output: %s)", path, exists, tc.exists, out.String())
			}
		})
	}
}
