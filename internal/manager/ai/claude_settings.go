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
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

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
