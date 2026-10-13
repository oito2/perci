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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/oito2/perci/internal/fsutil"
	"github.com/oito2/perci/internal/ui"
)

// claudeSettingsPath is Claude Code's project settings file, relative to
// the project root.
var claudeSettingsPath = filepath.Join(".claude", "settings.json")

// denyRules turns the patterns of an ignore file into Claude Code
// `permissions.deny` read rules: a directory pattern ("dir/") becomes
// "Read(dir/**)" and any other pattern "Read(pattern)". The .claude/
// directory is left out — denying it would keep Claude Code from reading
// its own project configuration.
func denyRules(ignoreFile string) []string {
	var rules []string
	for _, line := range strings.Split(ignoreFile, "\n") {
		p := strings.TrimSpace(line)
		if p == "" || strings.HasPrefix(p, "#") || p == ".claude/" {
			continue
		}
		rule := "Read(" + p + ")"
		if dirPattern, isDir := strings.CutSuffix(p, "/"); isDir {
			rule = "Read(" + dirPattern + "/**)"
		}
		if !slices.Contains(rules, rule) {
			rules = append(rules, rule)
		}
	}
	return rules
}

// sourceAmbiguous lists the ignore patterns that name build output or media
// in most projects but may hold versioned source in others (e.g. a frontend
// committed under dist/, screenshots under docs/img/).
var sourceAmbiguous = []string{"dist/", "build/", "out/", "*.png", "*.jpg", "*.jpeg", "*.gif", "*.webp"}

// gitTrackedFiles lists the files git tracks in dir, as slash-separated
// paths relative to the repository root. It returns nil when git is
// missing, dir is not inside a repository, or the listing fails.
func gitTrackedFiles(ctx context.Context, dir string) []string {
	gitPath, err := exec.LookPath("git")
	if err != nil {
		return nil
	}
	cmd := exec.CommandContext(ctx, gitPath, "-C", dir, "ls-files", "-z") //nolint:gosec // fixed argument list, dir is the project folder
	cmd.WaitDelay = 5 * time.Second
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	var files []string
	for f := range strings.SplitSeq(string(out), "\x00") {
		if f != "" {
			files = append(files, f)
		}
	}
	return files
}

// matchesTracked reports whether an ignore pattern covers any of the
// tracked files: a directory pattern ("dist/") matches a file below a
// directory of that name at any depth, any other pattern matches a file's
// base name.
func matchesTracked(pattern string, tracked []string) bool {
	dirPattern, isDir := strings.CutSuffix(pattern, "/")
	for _, f := range tracked {
		parts := strings.Split(f, "/")
		if isDir {
			for _, p := range parts[:len(parts)-1] {
				if ok, _ := path.Match(dirPattern, p); ok {
					return true
				}
			}
			continue
		}
		if ok, _ := path.Match(pattern, parts[len(parts)-1]); ok {
			return true
		}
	}
	return false
}

// withoutTrackedSource removes from ignoreFile the sourceAmbiguous patterns
// that cover files tracked in tracked, so the read rules built from it do
// not hide versioned source code or images from Claude Code.
func withoutTrackedSource(ignoreFile string, tracked []string) string {
	if len(tracked) == 0 {
		return ignoreFile
	}
	lines := strings.Split(ignoreFile, "\n")
	kept := lines[:0]
	for _, line := range lines {
		p := strings.TrimSpace(line)
		if slices.Contains(sourceAmbiguous, p) && matchesTracked(p, tracked) {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

// claudeDenyRules builds the read rules for the project at dir from
// ignoreFile, keeping the build-output and media patterns readable where
// git tracks files under them.
func claudeDenyRules(dir, ignoreFile string) []string {
	ctx, cancel := context.WithTimeout(context.TODO(), 10*time.Second)
	defer cancel()
	return denyRules(withoutTrackedSource(ignoreFile, gitTrackedFiles(ctx, dir)))
}

// mergeDenyRules adds rules to the permissions.deny list of the settings
// document in existing (empty for none) and returns the new document.
// Every other key is kept and a rule already listed is not added again;
// changed is false when nothing had to be added. A document that is not a
// JSON object, or whose permissions or deny have another type, is an error
// rather than being replaced.
func mergeDenyRules(existing []byte, rules []string) (merged []byte, changed bool, err error) {
	doc := map[string]any{}
	if len(bytes.TrimSpace(existing)) > 0 {
		dec := json.NewDecoder(bytes.NewReader(existing))
		dec.UseNumber() // keeps numbers exactly as written
		if err := dec.Decode(&doc); err != nil {
			return nil, false, fmt.Errorf("não é um JSON válido: %w", err)
		}
		if doc == nil {
			return nil, false, fmt.Errorf("não contém um objeto JSON")
		}
	}

	permissions := map[string]any{}
	if raw, ok := doc["permissions"]; ok {
		if permissions, ok = raw.(map[string]any); !ok {
			return nil, false, fmt.Errorf(`"permissions" não é um objeto`)
		}
	}
	var deny []any
	if raw, ok := permissions["deny"]; ok {
		if deny, ok = raw.([]any); !ok {
			return nil, false, fmt.Errorf(`"permissions.deny" não é uma lista`)
		}
	}
	for _, rule := range rules {
		if !slices.Contains(deny, any(rule)) {
			deny = append(deny, rule)
			changed = true
		}
	}
	if !changed {
		return existing, false, nil
	}
	permissions["deny"] = deny
	doc["permissions"] = permissions

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(doc); err != nil {
		return nil, false, err
	}
	return buf.Bytes(), true, nil
}

// writeClaudeSettings makes dir's .claude/settings.json deny Claude Code
// the given read rules, keeping everything else the file holds. An
// existing file that cannot be merged is left untouched and reported.
func writeClaudeSettings(dir string, rules []string, stdout io.Writer) error {
	path := filepath.Join(dir, claudeSettingsPath)
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("ler %s: %w", path, err)
	}
	merged, changed, err := mergeDenyRules(existing, rules)
	if err != nil {
		ui.Warning(stdout, path+": "+err.Error()+" — as regras de leitura do Claude Code não foram adicionadas; corrija o arquivo manualmente.")
		return nil
	}
	if !changed {
		ui.Info(stdout, "Já existente: "+path)
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("criar %s: %w", filepath.Dir(path), err)
	}
	if err := fsutil.WriteGenerated(path, merged, 0o644); err != nil {
		return fmt.Errorf("escrever %s: %w", path, err)
	}
	ui.Info(stdout, "Atualizado: "+path)
	return nil
}
