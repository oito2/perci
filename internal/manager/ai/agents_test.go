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
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func exists(t *testing.T, path string) bool {
	t.Helper()
	_, err := os.Lstat(path)
	return err == nil
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestAgentSet(t *testing.T) {
	if _, err := agentSet(nil); err == nil {
		t.Error("no agent accepted")
	}
	if _, err := agentSet([]string{"claude", "cursor"}); err == nil {
		t.Error("unknown agent accepted")
	}
	if set, err := agentSet(AgentIDs()); err != nil || len(set) != len(agents) {
		t.Errorf("agentSet(all) = %v, %v", set, err)
	}
}

// Every agent's files, and nothing but AGENTS.md and .instructions/ for
// Codex alone.
func TestGenerateSharedFiles_FilesPerAgent(t *testing.T) {
	bash := modelByName(t, "Linux Bash")
	docs := modelByName(t, "Documentação de projeto")

	dir := t.TempDir()
	if err := GenerateSharedFiles(dir, []Model{bash, docs}, AgentIDs(), true, io.Discard); err != nil {
		t.Fatal(err)
	}
	agentsMD := readFile(t, filepath.Join(dir, "AGENTS.md"))
	if strings.Contains(agentsMD, "@.instructions") {
		t.Errorf("AGENTS.md imports a standard:\n%s", agentsMD)
	}
	if !strings.Contains(agentsMD, "`.claude/rules/perci/`") || !strings.Contains(agentsMD, "<!-- Perci: generated for claude, gemini, codex, opencode, antigravity -->") {
		t.Errorf("AGENTS.md lacks the Claude note or the agents marker:\n%s", agentsMD)
	}
	for _, f := range []string{"CLAUDE.md", "GEMINI.md"} {
		if c := readFile(t, filepath.Join(dir, f)); !strings.Contains(c, "AGENTS.md\n") || !strings.Contains(c, ".instructions/PROJECT.md\n") {
			t.Errorf("%s does not import AGENTS.md and PROJECT.md:\n%s", f, c)
		}
	}
	rule := readFile(t, filepath.Join(dir, ".claude", "rules", "perci", "BASH.md"))
	if !strings.HasPrefix(rule, "---\npaths:\n  - \"**/*.sh\"\n  - \"**/*.bash\"\n---\n\n") || !strings.HasSuffix(rule, readFile(t, filepath.Join(dir, ".instructions", "BASH.md"))) {
		t.Errorf("BASH rule = %.200q", rule)
	}
	if exists(t, filepath.Join(dir, ".claude", "rules", "perci", "PROJECT-DOCUMENTATION.md")) {
		t.Error("a task-type standard got a path-scoped rule")
	}
	for _, f := range []string{".aiexclude", filepath.Join(".claude", "settings.json"), filepath.Join(".instructions", "PROJECT.md")} {
		if !exists(t, filepath.Join(dir, f)) {
			t.Errorf("%s missing", f)
		}
	}
	var oc map[string]any
	if err := json.Unmarshal([]byte(readFile(t, filepath.Join(dir, "opencode.json"))), &oc); err != nil || !reflect.DeepEqual(oc["instructions"], []any{".instructions/PROJECT.md"}) {
		t.Errorf("opencode.json = %v, %v", oc, err)
	}

	codex := t.TempDir()
	if err := GenerateSharedFiles(codex, []Model{bash}, []string{AgentCodex}, true, io.Discard); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"CLAUDE.md", "GEMINI.md", ".aiexclude", ".claude", "opencode.json"} {
		if exists(t, filepath.Join(codex, f)) {
			t.Errorf("%s created for Codex alone", f)
		}
	}
	if c := readFile(t, filepath.Join(codex, "AGENTS.md")); strings.Contains(c, ".claude/rules") {
		t.Error("AGENTS.md mentions Claude rules without Claude Code")
	}
	if got := DetectAgents(codex); !reflect.DeepEqual(got, map[string]bool{AgentCodex: true}) {
		t.Errorf("DetectAgents = %v", got)
	}

	if err := GenerateSharedFiles(dir, nil, nil, true, io.Discard); err == nil {
		t.Error("generation without agents succeeded")
	}
}

// Unchecking agents removes the files Perci wrote for them, keeping edited
// ones and everything else in a shared config file.
func TestGenerateSharedFiles_UncheckedAgentFilesRemoved(t *testing.T) {
	bash := modelByName(t, "Linux Bash")
	dir := t.TempDir()
	if err := GenerateSharedFiles(dir, []Model{bash}, AgentIDs(), true, io.Discard); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, "GEMINI.md"), "# mine\n")
	var oc map[string]any
	_ = json.Unmarshal([]byte(readFile(t, filepath.Join(dir, "opencode.json"))), &oc)
	oc["model"] = "anthropic/claude"
	data, _ := json.Marshal(oc)
	write(t, filepath.Join(dir, "opencode.json"), string(data))

	var out bytes.Buffer
	if err := GenerateSharedFiles(dir, []Model{bash}, []string{AgentCodex}, true, &out); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"CLAUDE.md", ".aiexclude", filepath.Join(".claude", "rules", "perci")} {
		if exists(t, filepath.Join(dir, f)) {
			t.Errorf("%s left behind", f)
		}
	}
	if readFile(t, filepath.Join(dir, "GEMINI.md")) != "# mine\n" || !strings.Contains(out.String(), "GEMINI.md não foi gerado pelo Perci ou foi editado") {
		t.Errorf("the edited GEMINI.md was not kept with a warning:\n%s", out.String())
	}
	if c := readFile(t, filepath.Join(dir, "opencode.json")); strings.Contains(c, "PROJECT.md") || !strings.Contains(c, "anthropic/claude") {
		t.Errorf("opencode.json = %s", c)
	}

	// A config file holding only what Perci added goes away.
	only := t.TempDir()
	if err := GenerateSharedFiles(only, nil, []string{AgentOpenCode}, true, io.Discard); err != nil {
		t.Fatal(err)
	}
	if err := GenerateSharedFiles(only, nil, []string{AgentCodex}, true, io.Discard); err != nil {
		t.Fatal(err)
	}
	if exists(t, filepath.Join(only, "opencode.json")) {
		t.Error("opencode.json with only Perci's entry was kept")
	}
}

// Unchecking a standard removes its Claude rule unless it was edited.
func TestGenerateSharedFiles_ClaudeRuleOfDeselectedModel(t *testing.T) {
	bash := modelByName(t, "Linux Bash")
	goModel := modelByName(t, "Go")
	dir := t.TempDir()
	if err := GenerateSharedFiles(dir, []Model{bash, goModel}, []string{AgentClaude}, true, io.Discard); err != nil {
		t.Fatal(err)
	}
	goRule := filepath.Join(dir, ".claude", "rules", "perci", "GO.md")
	write(t, goRule, readFile(t, goRule)+"\n- mine\n")
	if err := GenerateSharedFiles(dir, nil, []string{AgentClaude}, true, io.Discard); err != nil {
		t.Fatal(err)
	}
	if exists(t, filepath.Join(dir, ".claude", "rules", "perci", "BASH.md")) {
		t.Error("the rule of an unchecked standard was kept")
	}
	if !exists(t, goRule) {
		t.Error("an edited rule was removed")
	}
}

func TestProjectRules_CreatedOnceAndMigrated(t *testing.T) {
	dir := t.TempDir()
	if err := GenerateSharedFiles(dir, nil, []string{AgentCodex}, true, io.Discard); err != nil {
		t.Fatal(err)
	}
	project := filepath.Join(dir, ".instructions", "PROJECT.md")
	tpl, _ := readTpl("templates/PROJECT.md")
	if readFile(t, project) != tpl {
		t.Fatalf("PROJECT.md = %q", readFile(t, project))
	}
	write(t, project, tpl+"\n- Use tabs.\n")
	if err := GenerateSharedFiles(dir, nil, []string{AgentCodex}, true, io.Discard); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(readFile(t, project), "- Use tabs.\n") {
		t.Error("PROJECT.md was rewritten")
	}

	// An AGENTS.md from 1.0 with hand-written rules: a new section, an
	// extra line in a generated one, and the old "@" entries.
	old, _ := readTpl("templates/legacy/BASIC-1.0.md")
	old = strings.Replace(old, "## Language\n", "## Language\n\nCode identifiers in Portuguese.\n", 1)
	old += "\n## Deploy\n\nRun `make deploy` after merging.\n\n## Language-Specific Standards\n\n" + legacyStandardsHeaders[0] +
		"\n\n- Writing Go code: @.instructions/GO.md\n"
	legacy := t.TempDir()
	write(t, filepath.Join(legacy, "AGENTS.md"), old)
	var out bytes.Buffer
	if err := GenerateSharedFiles(legacy, nil, []string{AgentCodex}, true, &out); err != nil {
		t.Fatal(err)
	}
	got := readFile(t, filepath.Join(legacy, ".instructions", "PROJECT.md"))
	if !strings.Contains(got, "## Moved from AGENTS.md (") ||
		!strings.Contains(got, "Code identifiers in Portuguese.\n\n## Deploy\n\nRun `make deploy` after merging.\n") ||
		strings.Contains(got, "GO.md") || strings.Contains(got, "Rule Priority") {
		t.Errorf("PROJECT.md after the migration:\n%s", got)
	}
	if !strings.Contains(out.String(), "foram movidas") {
		t.Errorf("no migration warning:\n%s", out.String())
	}
	if strings.Contains(readFile(t, filepath.Join(legacy, "AGENTS.md")), "make deploy") {
		t.Error("AGENTS.md still holds the project rules")
	}
	// Running again neither moves nor duplicates anything.
	if err := GenerateSharedFiles(legacy, nil, []string{AgentCodex}, true, io.Discard); err != nil {
		t.Fatal(err)
	}
	if strings.Count(readFile(t, filepath.Join(legacy, ".instructions", "PROJECT.md")), "make deploy") != 1 {
		t.Error("the moved rules were duplicated")
	}
}

// Without overwrite AGENTS.md stays as it is, so nothing moves out of it;
// rules PROJECT.md already holds are never added twice.
func TestProjectRules_NoMoveWithoutOverwrite(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "AGENTS.md"), "# AGENTS.md\n\n## Deploy\n\nRun `make deploy`.\n")
	if err := GenerateSharedFiles(dir, nil, []string{AgentCodex}, false, io.Discard); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(readFile(t, filepath.Join(dir, ".instructions", "PROJECT.md")), "make deploy") {
		t.Error("rules moved although AGENTS.md was kept")
	}
	if err := ensureProjectRules(dir, true, io.Discard); err != nil {
		t.Fatal(err)
	}
	if err := ensureProjectRules(dir, true, io.Discard); err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(readFile(t, filepath.Join(dir, ".instructions", "PROJECT.md")), "make deploy"); n != 1 {
		t.Errorf("the rules were added %d times", n)
	}
}

// A current generated AGENTS.md has nothing to move.
func TestProjectContent_GeneratedHasNothing(t *testing.T) {
	dir := t.TempDir()
	if err := GenerateSharedFiles(dir, models, AgentIDs(), true, io.Discard); err != nil {
		t.Fatal(err)
	}
	moved, err := projectContent(readFile(t, filepath.Join(dir, "AGENTS.md")))
	if err != nil || moved != "" {
		t.Errorf("projectContent of a generated AGENTS.md = %q, %v", moved, err)
	}
	for _, f := range []string{"templates/legacy/BASIC-1.0.md", "templates/legacy/BASIC-1.2.md"} {
		tpl, _ := readTpl(f)
		if moved, _ := projectContent(tpl); moved != "" {
			t.Errorf("%s: %q", f, moved)
		}
	}
}

// AGENTS.md from before the agents marker: the agents are told by their files.
func TestDetectAgents_WithoutMarker(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "AGENTS.md"), "# AGENTS.md\n")
	write(t, filepath.Join(dir, "CLAUDE.md"), "@AGENTS.md\n")
	if got := DetectAgents(t.TempDir()); len(got) != 0 {
		t.Errorf("DetectAgents without AGENTS.md = %v", got)
	}
	if got := DetectAgents(dir); !got[AgentClaude] || !got[AgentCodex] || !got[AgentAntigravity] || got[AgentGemini] || got[AgentOpenCode] {
		t.Errorf("DetectAgents = %v", got)
	}
}

// opencode.jsonc and a broken opencode.json are never rewritten.
func TestOpenCodeConfig_LeftAloneWhenItCannotBeMerged(t *testing.T) {
	jsonc := t.TempDir()
	write(t, filepath.Join(jsonc, "opencode.jsonc"), "{ // mine\n}\n")
	var out bytes.Buffer
	if err := GenerateSharedFiles(jsonc, nil, []string{AgentOpenCode}, true, &out); err != nil {
		t.Fatal(err)
	}
	if exists(t, filepath.Join(jsonc, "opencode.json")) || !strings.Contains(out.String(), "opencode.jsonc") {
		t.Errorf("opencode.jsonc not respected:\n%s", out.String())
	}

	broken := t.TempDir()
	write(t, filepath.Join(broken, "opencode.json"), "{ not json")
	out.Reset()
	if err := GenerateSharedFiles(broken, nil, []string{AgentOpenCode}, true, &out); err != nil {
		t.Fatal(err)
	}
	if readFile(t, filepath.Join(broken, "opencode.json")) != "{ not json" || !strings.Contains(out.String(), "não é um objeto JSON") {
		t.Errorf("broken opencode.json touched:\n%s", out.String())
	}
}

// Every path-scoped standard has quoted, non-empty globs.
func TestModels_PathsAreValid(t *testing.T) {
	for _, m := range models {
		for _, p := range m.Paths {
			if p == "" || strings.ContainsAny(p, "\"\n") {
				t.Errorf("%s: bad path %q", m.Name, p)
			}
		}
	}
}

// Gemini gets both ignore files; unchecking Gemini and Claude Code removes
// them, an old-version .aiexclude, and Perci's read rules — keeping the
// user's own rules.
func TestReadRules_PerAgent(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, ".claude", "settings.json"), `{"permissions":{"deny":["Bash(rm *)","Read(*.png)"]}}`)
	if err := GenerateSharedFiles(dir, nil, []string{AgentClaude, AgentGemini}, true, io.Discard); err != nil {
		t.Fatal(err)
	}
	gi := readFile(t, filepath.Join(dir, ".geminiignore"))
	if !strings.HasPrefix(gi, "# ===") || !strings.Contains(gi, "\n.env\n") || strings.Contains(gi, "*.png") {
		t.Errorf(".geminiignore = %.300q", gi)
	}
	settings := readFile(t, filepath.Join(dir, ".claude", "settings.json"))
	if strings.Contains(settings, "Read(*.png)") || !strings.Contains(settings, "Bash(rm *)") || !strings.Contains(settings, "Read(.env)") {
		t.Errorf("settings.json = %s", settings)
	}

	if err := GenerateSharedFiles(dir, nil, []string{AgentCodex}, true, io.Discard); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{".geminiignore", ".aiexclude"} {
		if exists(t, filepath.Join(dir, f)) {
			t.Errorf("%s left behind", f)
		}
	}
	if settings := readFile(t, filepath.Join(dir, ".claude", "settings.json")); strings.Contains(settings, "Read(") || !strings.Contains(settings, "Bash(rm *)") {
		t.Errorf("settings.json after unchecking Claude Code = %s", settings)
	}

	// A settings file holding only Perci's rules, and a 1.0 .aiexclude, go away.
	only := t.TempDir()
	if err := GenerateSharedFiles(only, nil, []string{AgentClaude}, true, io.Discard); err != nil {
		t.Fatal(err)
	}
	old, _ := readTpl("templates/legacy/aiexclude-1.0")
	write(t, filepath.Join(only, ".aiexclude"), old)
	if err := GenerateSharedFiles(only, nil, []string{AgentCodex}, true, io.Discard); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{".claude", ".aiexclude"} {
		if exists(t, filepath.Join(only, f)) {
			t.Errorf("%s left behind", f)
		}
	}
}

// AGENTS.md stays within the agents' limits with every standard checked:
// Antigravity truncates a rule file at 24,000 bytes, Codex reads at most
// 32 KiB of AGENTS.md files.
func TestAgentsMD_Size(t *testing.T) {
	dir := t.TempDir()
	if err := GenerateSharedFiles(dir, models, AgentIDs(), true, io.Discard); err != nil {
		t.Fatal(err)
	}
	if n := len(readFile(t, filepath.Join(dir, "AGENTS.md"))); n >= 24000 {
		t.Errorf("AGENTS.md has %d bytes, the limit is 24000", n)
	}
}
