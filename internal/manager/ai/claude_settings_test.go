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
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestDenyRules_FromTheIgnoreTemplate(t *testing.T) {
	tpl, err := readTpl("templates/ignore/patterns")
	if err != nil {
		t.Fatal(err)
	}
	rules := denyRules(tpl)
	for _, want := range []string{"Read(.env)", "Read(.env.*)", "Read(*.pem)", "Read(node_modules/**)", "Read(.git/**)", "Read(*.min.js)", "Read(.gemini/**)"} {
		if !slices.Contains(rules, want) {
			t.Errorf("missing %s in %v", want, rules)
		}
	}
	// Images, PDFs and lockfiles stay readable: agents look at screenshots
	// and check dependency versions.
	for _, readable := range []string{"Read(*.png)", "Read(*.svg)", "Read(*.pdf)", "Read(package-lock.json)", "Read(composer.lock)"} {
		if slices.Contains(rules, readable) {
			t.Errorf("%s must not be generated", readable)
		}
	}
	for _, r := range rules {
		if strings.Contains(r, ".claude/") {
			t.Errorf("the project's own Claude configuration must stay readable: %s", r)
		}
		if strings.Contains(r, "//") || strings.HasPrefix(r, "Read(#") {
			t.Errorf("malformed rule %q", r)
		}
	}
	if n := len(rules); n != len(slices.Compact(slices.Sorted(slices.Values(rules)))) {
		t.Errorf("duplicate rules among %d", n)
	}
}

func TestMergeDenyRules_KeepsEveryOtherKeyAndNeverDuplicates(t *testing.T) {
	existing := []byte(`{"model":"opus","big":12345678901234567890,"permissions":{"allow":["Bash(ls)"],"deny":["Read(.env)","Bash(rm *)"]},"hooks":{"x":[1]}}`)
	rules := []string{"Read(.env)", "Read(*.pem)"}

	out, changed, err := mergeDenyRules(existing, rules, nil)
	if err != nil || !changed {
		t.Fatalf("merge: changed=%v err=%v", changed, err)
	}
	var doc map[string]any
	dec := json.NewDecoder(bytes.NewReader(out))
	dec.UseNumber()
	if err := dec.Decode(&doc); err != nil {
		t.Fatal(err)
	}
	perms := doc["permissions"].(map[string]any)
	if got, _ := json.Marshal(perms["deny"]); string(got) != `["Read(.env)","Bash(rm *)","Read(*.pem)"]` {
		t.Errorf("deny = %s", got)
	}
	if got, _ := json.Marshal(perms["allow"]); string(got) != `["Bash(ls)"]` {
		t.Errorf("allow lost: %s", got)
	}
	for _, key := range []string{"model", "big", "hooks"} {
		if _, ok := doc[key]; !ok {
			t.Errorf("key %q lost", key)
		}
	}
	if !strings.Contains(string(out), "12345678901234567890") {
		t.Errorf("a number lost precision:\n%s", out)
	}

	again, changed, err := mergeDenyRules(out, rules, nil)
	if err != nil || changed || !bytes.Equal(again, out) {
		t.Errorf("a second merge must change nothing: changed=%v err=%v", changed, err)
	}
}

func TestMergeDenyRules_FromNothing(t *testing.T) {
	out, changed, err := mergeDenyRules(nil, []string{"Read(.env)"}, nil)
	if err != nil || !changed {
		t.Fatal(changed, err)
	}
	if !strings.Contains(string(out), `"deny": [`) || !strings.HasSuffix(string(out), "}\n") {
		t.Errorf("out = %s", out)
	}
}

func TestMergeDenyRules_RefusesWhatItCannotMerge(t *testing.T) {
	for name, doc := range map[string]string{
		"not json":            "{oops",
		"null":                "null",
		"array":               "[]",
		"permissions string":  `{"permissions":"x"}`,
		"deny not a list":     `{"permissions":{"deny":"Read(.env)"}}`,
		"permissions in list": `{"permissions":[1]}`,
	} {
		if _, _, err := mergeDenyRules([]byte(doc), []string{"Read(.env)"}, nil); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestGenerateSharedFiles_ClaudeSettingsInsteadOfClaudeignore(t *testing.T) {
	dir := t.TempDir()
	settings := filepath.Join(dir, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settings), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settings, []byte(`{"model":"opus","permissions":{"deny":["Bash(rm *)"]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".claudeignore"), []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := GenerateSharedFiles(dir, []Model{models[0]}, AgentIDs(), true, &out); err != nil {
		t.Fatal(err)
	}
	if err := GenerateSharedFiles(dir, []Model{models[0]}, AgentIDs(), true, &out); err != nil { // idempotent
		t.Fatal(err)
	}

	data, _ := os.ReadFile(settings)
	var doc struct {
		Model       string `json:"model"`
		Permissions struct {
			Deny []string `json:"deny"`
		} `json:"permissions"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Model != "opus" || !slices.Contains(doc.Permissions.Deny, "Bash(rm *)") || !slices.Contains(doc.Permissions.Deny, "Read(.env)") {
		t.Errorf("settings = %s", data)
	}
	if n := strings.Count(string(data), `"Read(.env)"`); n != 1 {
		t.Errorf("Read(.env) appears %d times", n)
	}
	if !strings.Contains(out.String(), ".claudeignore não tem efeito") {
		t.Errorf("no warning about the old file:\n%s", out.String())
	}
	if _, err := os.Stat(filepath.Join(dir, ".claudeignore.perci-bak")); err == nil {
		t.Error(".claudeignore must not be rewritten")
	}
}

// An unreadable settings file is left alone with a warning, and the rest
// of the generation still happens.
func TestGenerateSharedFiles_BrokenClaudeSettingsIsKept(t *testing.T) {
	dir := t.TempDir()
	settings := filepath.Join(dir, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settings), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settings, []byte("{broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := GenerateSharedFiles(dir, []Model{models[0]}, AgentIDs(), true, &out); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(settings); string(data) != "{broken" {
		t.Errorf("settings changed: %s", data)
	}
	if !strings.Contains(out.String(), "não foram adicionadas") {
		t.Errorf("no warning:\n%s", out.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "AGENTS.md")); err != nil {
		t.Error("AGENTS.md not generated")
	}
}

// Generated files go through WriteGenerated: a different previous version
// is kept as .perci-bak and a symlink is written through.
func TestGenerateSharedFiles_BackupAndSymlink(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("my own rules\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	elsewhere := filepath.Join(t.TempDir(), "claude-shared.md")
	if err := os.WriteFile(elsewhere, []byte("shared\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(elsewhere, filepath.Join(dir, "CLAUDE.md")); err != nil {
		t.Fatal(err)
	}

	if err := GenerateSharedFiles(dir, []Model{models[0]}, AgentIDs(), true, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if bak, err := os.ReadFile(filepath.Join(dir, "AGENTS.md.perci-bak")); err != nil || string(bak) != "my own rules\n" {
		t.Errorf("AGENTS.md backup = %q, %v", bak, err)
	}
	if info, err := os.Lstat(filepath.Join(dir, "CLAUDE.md")); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("CLAUDE.md symlink replaced (err=%v)", err)
	}
	if data, _ := os.ReadFile(elsewhere); !strings.Contains(string(data), "@AGENTS.md") {
		t.Errorf("the link target was not written: %q", data)
	}
}

// A CLAUDE.md linked to AGENTS.md is kept: AGENTS.md gets the generated
// rules and its previous content survives in the backup.
func TestGenerateSharedFiles_ClaudeLinkedToAgentsIsKept(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("my own rules\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("AGENTS.md", filepath.Join(dir, "CLAUDE.md")); err != nil {
		t.Fatal(err)
	}
	if err := GenerateSharedFiles(dir, []Model{models[0]}, AgentIDs(), true, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	agents, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if strings.Contains(string(agents), "@AGENTS.md") || len(agents) < 100 {
		t.Errorf("AGENTS.md was replaced by the CLAUDE.md stub: %q", agents)
	}
	if bak, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md.perci-bak")); string(bak) != "my own rules\n" {
		t.Errorf("AGENTS.md backup = %q, want the original rules", bak)
	}
	if info, err := os.Lstat(filepath.Join(dir, "CLAUDE.md")); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("CLAUDE.md link replaced (err=%v)", err)
	}
}

// The ignore file says what it does not do.
func TestIgnoreHeadersAreHonest(t *testing.T) {
	for _, path := range []string{"templates/ignore/aiexclude-header", "templates/ignore/geminiignore-header"} {
		tpl, err := readTpl(path)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(tpl, "Shield") || strings.Contains(tpl, "Stops Prompt Injection") {
			t.Errorf("%s still claims protection it does not give", path)
		}
		if !strings.Contains(tpl, "not a security boundary") || !strings.Contains(tpl, ".claude/settings.json") || !strings.Contains(tpl, "read") {
			t.Errorf("%s must say it is not a security boundary and where the other agents get the list", path)
		}
	}
}

// Perci's own rules follow the canonical list: rules an older version
// generated go away, the user's stay, and a file left with nothing else is
// emptied.
func TestMergeDenyRules_RemovesRulesPerciNoLongerGenerates(t *testing.T) {
	managed, err := managedDenyRules()
	if err != nil {
		t.Fatal(err)
	}
	for _, old := range []string{"Read(*.png)", "Read(package-lock.json)", "Read(dist/**)", "Read(vendor/**)"} {
		if !slices.Contains(managed, old) {
			t.Errorf("%s is not known as a Perci rule", old)
		}
	}
	existing := []byte(`{"permissions":{"deny":["Read(*.png)","Read(.env)","Bash(rm *)","Read(package-lock.json)"]}}`)
	out, changed, err := mergeDenyRules(existing, []string{"Read(.env)", "Read(*.pem)"}, managed)
	if err != nil || !changed {
		t.Fatal(changed, err)
	}
	var doc struct {
		Permissions struct {
			Deny []string `json:"deny"`
		} `json:"permissions"`
	}
	_ = json.Unmarshal(out, &doc)
	if !slices.Equal(doc.Permissions.Deny, []string{"Read(.env)", "Bash(rm *)", "Read(*.pem)"}) {
		t.Errorf("deny = %v", doc.Permissions.Deny)
	}

	out, _, err = mergeDenyRules([]byte(`{"permissions":{"deny":["Read(.env)"]}}`), nil, managed)
	if err != nil || strings.TrimSpace(string(out)) != "{}" {
		t.Errorf("only Perci rules, removed: %s, %v", out, err)
	}
}

func TestWithoutTrackedSource_KeepsVersionedSourceReadable(t *testing.T) {
	ignore := "dist/\nbuild/\nvendor/\nenv/\nnode_modules/\n.env\n*.pem\n"
	tracked := []string{"cmd/app/frontend/dist/index.html", "cmd/app/frontend/dist/vendor/lucide/icons.js", "node_modules/x/index.js", ".env", "certs/a.pem"}
	rules := denyRules(withoutTrackedSource(ignore, tracked))
	for _, gone := range []string{"Read(dist/**)", "Read(vendor/**)"} {
		if slices.Contains(rules, gone) {
			t.Errorf("%s hides tracked files: %v", gone, rules)
		}
	}
	for _, kept := range []string{"Read(build/**)", "Read(env/**)", "Read(node_modules/**)", "Read(.env)", "Read(*.pem)"} {
		if !slices.Contains(rules, kept) {
			t.Errorf("missing %s in %v", kept, rules)
		}
	}
	if got := denyRules(withoutTrackedSource(ignore, nil)); !slices.Contains(got, "Read(dist/**)") {
		t.Errorf("without a git listing every rule stays: %v", got)
	}
}

func TestMatchesTracked_DirectoryNeedsAPathComponent(t *testing.T) {
	if matchesTracked("dist/", []string{"dist"}) {
		t.Error("a file named dist is not a dist/ directory")
	}
	if !matchesTracked("dist/", []string{"a/dist/b.js"}) {
		t.Error("a nested dist/ directory must match")
	}
	if matchesTracked("*.png", []string{"img.png/readme.md"}) {
		t.Error("a file pattern matches base names only")
	}
}

func TestClaudeDenyRules_ReadsTheGitListing(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, ".config"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	if err := os.MkdirAll(filepath.Join(dir, "web", "dist"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "web", "dist", "app.js"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "web/dist/app.js"}} {
		if out, err := exec.CommandContext(t.Context(), "git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	patterns, err := ignorePatterns(dir)
	if err != nil {
		t.Fatal(err)
	}
	rules := denyRules(patterns)
	if slices.Contains(rules, "Read(dist/**)") || !slices.Contains(rules, "Read(build/**)") || !slices.Contains(rules, "Read(vendor/**)") {
		t.Errorf("rules = %v", rules)
	}
}
