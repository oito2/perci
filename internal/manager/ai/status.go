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
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

// States of a standard in a project (GuideStatus).
const (
	StatusApplied  = "applied"  // identical to what this version writes
	StatusOutdated = "outdated" // what an earlier Perci version wrote
	StatusEdited   = "edited"   // changed by hand (Aplicar replaces it, keeping a .perci-bak)
)

// GuideStatus tells the state of model's standard in dir: "" when it is
// not there, else StatusApplied, StatusOutdated or StatusEdited. A copy
// under a previous file name (Model.Legacy) counts as outdated.
func GuideStatus(dir string, model Model) string {
	data, err := os.ReadFile(filepath.Join(dir, ".instructions", filepath.Base(model.Instruction)))
	if err != nil {
		for _, name := range model.Legacy {
			if _, err := os.Stat(filepath.Join(dir, ".instructions", name)); err == nil {
				return StatusOutdated
			}
		}
		return ""
	}
	if current, err := renderInstruction(dir, model); err == nil && string(data) == current {
		return StatusApplied
	}
	sum := sha256.Sum256(data)
	if slices.Contains(templateHistory[filepath.Base(model.Instruction)], hex.EncodeToString(sum[:])) {
		return StatusOutdated
	}
	return StatusEdited
}

// SuggestPreset names the project type (Presets) the files in dir point
// to, or "" when none does.
func SuggestPreset(dir string) string {
	read := func(name string) string {
		data, _ := readCapped(filepath.Join(dir, name))
		return string(data)
	}
	isDir := func(name string) bool {
		info, err := os.Stat(filepath.Join(dir, name))
		return err == nil && info.IsDir()
	}
	goMod := read("go.mod")
	switch {
	case strings.Contains(read("version.php"), "$plugin->component"):
		return "Plugin Moodle"
	case isDir("src-tauri"):
		return "Aplicativo Rust + Tauri v2"
	case read("project.godot") != "":
		return "Jogo Godot (Android)"
	case strings.Contains(goMod, "github.com/wailsapp/wails/v3"):
		return "Aplicativo Go + Wails"
	case strings.Contains(goMod, "github.com/modelcontextprotocol/go-sdk"):
		return "Servidor MCP em Go"
	case strings.Contains(read("composer.json"), `"slim/slim"`):
		return "Sistema PHP + Slim"
	}
	return ""
}

// Change is one file an Aplicar would create, update or remove, with its
// path relative to the project folder.
type Change struct {
	Path   string
	Action string // "create", "update" or "remove"
}

// previewFiles are the files a generation reads or writes outside the
// previewDirs folders.
var previewFiles = []string{
	"AGENTS.md", "CLAUDE.md", "GEMINI.md", ".aiexclude", ".geminiignore", ".claudeignore",
	openCodeConfig, "opencode.jsonc", claudeSettingsPath,
	"version.php", filepath.Join("public", "version.php"),
}

// previewDirs are the folders whose files a generation writes.
var previewDirs = []string{".instructions", claudeRulesDir}

// snapshot reads the files of root a generation reads or writes, keyed by
// their path relative to root; links are followed.
func snapshot(root string) map[string][]byte {
	files := map[string][]byte{}
	for _, rel := range previewFiles {
		if data, err := os.ReadFile(filepath.Join(root, rel)); err == nil {
			files[rel] = data
		}
	}
	for _, d := range previewDirs {
		_ = filepath.WalkDir(filepath.Join(root, d), func(path string, e fs.DirEntry, err error) error {
			if err != nil || e.IsDir() {
				return nil
			}
			if rel, err := filepath.Rel(root, path); err == nil {
				if data, err := os.ReadFile(path); err == nil {
					files[rel] = data
				}
			}
			return nil
		})
	}
	return files
}

// Preview tells what GenerateSharedFiles(dir, active, agentIDs, true, …)
// would change in dir, without touching it: the generation runs on a
// temporary copy of the files it reads or writes, with git's file list
// taken from dir. Links are copied as their content, so nothing they point
// to is written; a CLAUDE.md or GEMINI.md linked to AGENTS.md is left out
// of the result (it changes with AGENTS.md).
func Preview(dir string, active []Model, agentIDs []string) ([]Change, error) {
	if _, err := agentSet(agentIDs); err != nil {
		return nil, err
	}
	return previewRun(dir, func(tmp string) error {
		return generate(tmp, dir, active, agentIDs, true, io.Discard)
	})
}

// previewRun tells what run changes when it works on a temporary copy of
// the files of dir a generation reads or writes (see Preview).
func previewRun(dir string, run func(tmp string) error) ([]Change, error) {
	tmp, err := os.MkdirTemp("", "perci-preview-*")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(tmp) }()

	before := snapshot(dir)
	for rel, data := range before {
		dest := filepath.Join(tmp, rel)
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(dest, data, 0o644); err != nil {
			return nil, err
		}
	}
	if err := run(tmp); err != nil {
		return nil, err
	}
	after := snapshot(tmp)
	// Backups of replaced files are not changes of their own (the
	// confirmation says replaced files keep one).
	for rel := range after {
		if strings.HasSuffix(rel, ".perci-bak") {
			delete(after, rel)
			delete(before, rel)
		}
	}
	// A stub linked to AGENTS.md changes with it, not on its own.
	for _, stub := range []string{"CLAUDE.md", "GEMINI.md"} {
		if sameFile(filepath.Join(dir, stub), filepath.Join(dir, "AGENTS.md")) {
			delete(before, stub)
			delete(after, stub)
		}
	}

	var changes []Change
	for rel, data := range after {
		old, ok := before[rel]
		switch {
		case !ok:
			changes = append(changes, Change{Path: rel, Action: "create"})
		case !bytes.Equal(old, data):
			changes = append(changes, Change{Path: rel, Action: "update"})
		}
	}
	for rel := range before {
		if _, ok := after[rel]; !ok {
			changes = append(changes, Change{Path: rel, Action: "remove"})
		}
	}
	sort.Slice(changes, func(i, j int) bool { return changes[i].Path < changes[j].Path })
	return changes, nil
}
