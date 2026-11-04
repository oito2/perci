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
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestGuideStatus(t *testing.T) {
	bash := modelByName(t, "Linux Bash")
	dir := t.TempDir()
	if got := GuideStatus(dir, bash); got != "" {
		t.Errorf("missing standard: %q", got)
	}
	if err := GenerateSharedFiles(dir, []Model{bash}, []string{AgentCodex}, true, io.Discard); err != nil {
		t.Fatal(err)
	}
	if got := GuideStatus(dir, bash); got != StatusApplied {
		t.Errorf("after Aplicar: %q", got)
	}
	path := filepath.Join(dir, ".instructions", "BASH.md")
	write(t, path, "old version\n")
	sum := sha256.Sum256([]byte("old version\n"))
	saved := templateHistory["BASH.md"]
	templateHistory["BASH.md"] = append(slices.Clone(saved), hex.EncodeToString(sum[:]))
	t.Cleanup(func() { templateHistory["BASH.md"] = saved })
	if got := GuideStatus(dir, bash); got != StatusOutdated {
		t.Errorf("an earlier version: %q", got)
	}
	write(t, path, "mine\n")
	if got := GuideStatus(dir, bash); got != StatusEdited {
		t.Errorf("an edited copy: %q", got)
	}

	docs := modelByName(t, "Documentação de projeto")
	write(t, filepath.Join(dir, ".instructions", "DOCUMENTATION-SITE.md"), "x")
	if got := GuideStatus(dir, docs); got != StatusOutdated {
		t.Errorf("a copy under the old name: %q", got)
	}
}

// Every committed version of a standard is known, so a project's copy of
// it shows as outdated rather than edited.
func TestTemplateHistory(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	if out, err := exec.CommandContext(t.Context(), "git", "rev-parse", "--is-inside-work-tree").Output(); err != nil || strings.TrimSpace(string(out)) != "true" {
		t.Skip("not a git checkout")
	}
	for _, m := range models {
		rel := filepath.ToSlash(m.Instruction)
		out, err := exec.CommandContext(t.Context(), "git", "log", "--format=%H", "--", rel).Output()
		if err != nil {
			t.Fatal(err)
		}
		for _, commit := range strings.Fields(string(out)) {
			data, err := exec.CommandContext(t.Context(), "git", "show", commit+":./"+rel).Output()
			if err != nil {
				continue
			}
			sum := sha256.Sum256(data)
			h := hex.EncodeToString(sum[:])
			if !slices.Contains(templateHistory[filepath.Base(rel)], h) {
				t.Errorf("templateHistory[%q] lacks %s (version of %s)", filepath.Base(rel), h, commit[:7])
			}
		}
	}
}

func TestSuggestPreset(t *testing.T) {
	cases := map[string]map[string]string{
		"Plugin Moodle":              {"version.php": "<?php\n$plugin->component = 'local_x';\n"},
		"Aplicativo Rust + Tauri v2": {"src-tauri/tauri.conf.json": "{}"},
		"Jogo Godot (Android)":       {"project.godot": "config_version=5\n"},
		"Aplicativo Go + Wails":      {"go.mod": "module x\nrequire github.com/wailsapp/wails/v3 v3.0.0\n"},
		"Servidor MCP em Go":         {"go.mod": "module x\nrequire github.com/modelcontextprotocol/go-sdk v1.8.0\n"},
		"Sistema PHP + Slim":         {"composer.json": `{"require":{"slim/slim":"^4.15"}}`},
		"":                           {"go.mod": "module x\n", "README.md": "x"},
	}
	presetNames := map[string]bool{"": true}
	for _, p := range presets {
		presetNames[p.Name] = true
	}
	for want, files := range cases {
		if !presetNames[want] {
			t.Errorf("%q is not a preset", want)
		}
		dir := t.TempDir()
		for name, content := range files {
			write(t, filepath.Join(dir, name), content)
		}
		if got := SuggestPreset(dir); got != want {
			t.Errorf("SuggestPreset(%v) = %q, want %q", files, got, want)
		}
	}
}

func TestPreview(t *testing.T) {
	bash := modelByName(t, "Linux Bash")
	dir := t.TempDir()
	changes, err := Preview(dir, []Model{bash}, []string{AgentClaude})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, c := range changes {
		got[c.Path] = c.Action
	}
	for _, want := range []string{"AGENTS.md", "CLAUDE.md", filepath.Join(".instructions", "BASH.md"), filepath.Join(".instructions", "PROJECT.md"), claudeSettingsPath, filepath.Join(claudeRulesDir, "BASH.md")} {
		if got[want] != "create" {
			t.Errorf("%s: %q, want create (all: %v)", want, got[want], got)
		}
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("Preview wrote into the folder: %v", entries)
	}

	if err := GenerateSharedFiles(dir, []Model{bash}, []string{AgentClaude}, true, io.Discard); err != nil {
		t.Fatal(err)
	}
	if changes, err := Preview(dir, []Model{bash}, []string{AgentClaude}); err != nil || len(changes) != 0 {
		t.Errorf("Preview after the same Aplicar = %v, %v", changes, err)
	}
	changes, err = Preview(dir, nil, []string{AgentCodex})
	if err != nil {
		t.Fatal(err)
	}
	got = map[string]string{}
	for _, c := range changes {
		got[c.Path] = c.Action
	}
	for path := range got {
		if strings.HasSuffix(path, ".perci-bak") {
			t.Errorf("a backup is listed as a change: %s", path)
		}
	}
	if got["CLAUDE.md"] != "remove" || got[filepath.Join(".instructions", "BASH.md")] != "remove" || got["AGENTS.md"] != "update" || got[filepath.Join(".instructions", "PROJECT.md")] != "" {
		t.Errorf("switching to Codex without standards: %v", got)
	}
	if _, err := Preview(dir, nil, nil); err == nil {
		t.Error("Preview without agents succeeded")
	}
}

// A preview never writes through a link to a file outside the folder, and
// a stub linked to AGENTS.md is not reported on its own.
func TestPreview_Links(t *testing.T) {
	dir, dotfiles := t.TempDir(), t.TempDir()
	shared := filepath.Join(dotfiles, "AGENTS.md")
	write(t, shared, "# shared rules\n")
	if err := os.Symlink(shared, filepath.Join(dir, "AGENTS.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("AGENTS.md", filepath.Join(dir, "CLAUDE.md")); err != nil {
		t.Fatal(err)
	}
	changes, err := Preview(dir, nil, []string{AgentClaude})
	if err != nil {
		t.Fatal(err)
	}
	if readFile(t, shared) != "# shared rules\n" {
		t.Error("the preview wrote through the AGENTS.md link")
	}
	for _, c := range changes {
		if c.Path == "CLAUDE.md" {
			t.Errorf("the linked CLAUDE.md was reported: %v", changes)
		}
	}
	if !slices.ContainsFunc(changes, func(c Change) bool { return c.Path == "AGENTS.md" && c.Action == "update" }) {
		t.Errorf("AGENTS.md not reported: %v", changes)
	}
}

// The preview takes git's file list from the project, as Aplicar does: a
// tracked vendor/ stays readable in both.
func TestPreview_UsesTheProjectGitListing(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, ".config"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	write(t, filepath.Join(dir, "web", "vendor", "lib.js"), "x")
	for _, args := range [][]string{{"init", "-q"}, {"add", "web/vendor/lib.js"}} {
		if out, err := exec.CommandContext(t.Context(), "git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	if err := GenerateSharedFiles(dir, nil, []string{AgentClaude}, true, io.Discard); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(readFile(t, filepath.Join(dir, claudeSettingsPath)), "Read(vendor/**)") {
		t.Fatal("a tracked vendor/ was denied")
	}
	if changes, err := Preview(dir, nil, []string{AgentClaude}); err != nil || len(changes) != 0 {
		t.Errorf("Preview = %v, %v", changes, err)
	}
}

// Replacing an edited standard makes a .perci-bak, which is not listed.
func TestPreview_HidesBackups(t *testing.T) {
	bash := modelByName(t, "Linux Bash")
	dir := t.TempDir()
	if err := GenerateSharedFiles(dir, []Model{bash}, []string{AgentCodex}, true, io.Discard); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, ".instructions", "BASH.md"), "mine\n")
	changes, err := Preview(dir, []Model{bash}, []string{AgentCodex})
	if err != nil || len(changes) != 1 || changes[0] != (Change{Path: filepath.Join(".instructions", "BASH.md"), Action: "update"}) {
		t.Errorf("Preview = %v, %v", changes, err)
	}
}
