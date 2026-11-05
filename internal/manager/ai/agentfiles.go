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

// claudeRulesDir holds the path-scoped Claude Code rules Perci writes, one
// per standard that covers file types; Claude Code loads each only when it
// works on a file its paths match.
var claudeRulesDir = filepath.Join(".claude", "rules", "perci")

// claudeRule is the rule file of model: its paths as YAML frontmatter,
// then the standard itself (content, already rendered).
func claudeRule(model Model, content string) string {
	var b strings.Builder
	b.WriteString("---\npaths:\n")
	for _, p := range model.Paths {
		fmt.Fprintf(&b, "  - %q\n", p)
	}
	b.WriteString("---\n\n")
	b.WriteString(content)
	return b.String()
}

// syncClaudeRules writes the rule of every model of active that covers
// file types, and removes the rules of the other models while they are
// still exactly what Perci wrote (an edited rule is kept, with a warning).
// rendered holds each active model's standard as written to .instructions/.
// With active nil every rule Perci wrote is removed.
func syncClaudeRules(dir string, active []Model, rendered map[string]string, overwrite bool, stdout io.Writer) error {
	rulesDir := filepath.Join(dir, claudeRulesDir)
	keep := map[string]bool{}
	for _, m := range active {
		if len(m.Paths) == 0 {
			continue
		}
		name := filepath.Base(m.Instruction)
		keep[name] = true
		if err := os.MkdirAll(rulesDir, 0o755); err != nil {
			return err
		}
		if err := WriteFile(filepath.Join(rulesDir, name), claudeRule(m, rendered[m.Name]), overwrite, stdout); err != nil {
			return err
		}
	}
	for _, m := range models {
		name := filepath.Base(m.Instruction)
		if keep[name] || len(m.Paths) == 0 {
			continue
		}
		path := filepath.Join(rulesDir, name)
		if _, err := os.Lstat(path); err != nil {
			continue
		}
		content, err := renderInstruction(dir, m)
		if err != nil {
			return err
		}
		generated := []string{claudeRule(m, content)}
		if data, err := os.ReadFile(path); err == nil && earlierRule(m, data) {
			generated = append(generated, string(data))
		}
		removeIfPristine(path, stdout, generated...)
	}
	// Leaves the folder only when nothing else lives in it.
	_ = os.Remove(rulesDir)
	return nil
}

// earlierRule reports whether data is model's rule as an earlier Perci
// version wrote it: a paths frontmatter followed by a standard whose
// SHA-256 is in templateHistory.
func earlierRule(model Model, data []byte) bool {
	rest, ok := bytes.CutPrefix(data, []byte("---\npaths:\n"))
	if !ok {
		return false
	}
	_, body, ok := bytes.Cut(rest, []byte("\n---\n\n"))
	if !ok {
		return false
	}
	sum := sha256.Sum256(body)
	return slices.Contains(templateHistory[filepath.Base(model.Instruction)], hex.EncodeToString(sum[:]))
}

// removeIfPristine deletes the regular file at path when its content is
// exactly one of generated; an edited copy is kept, with a warning.
func removeIfPristine(path string, stdout io.Writer, generated ...string) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	if !slices.Contains(generated, string(data)) {
		ui.Warning(stdout, path+" não foi gerado pelo Perci ou foi editado — mantido; apague-o manualmente se não precisar mais dele.")
		return
	}
	if err := os.Remove(path); err != nil {
		ui.Warning(stdout, "Falha ao remover "+path+": "+err.Error())
		return
	}
	ui.Info(stdout, "Removido: "+path)
}

// writeStub writes the stub at name (CLAUDE.md, GEMINI.md) unless it
// already resolves to AGENTS.md, which gives the agent the rules as is —
// writing the stub through it would replace AGENTS.md.
func writeStub(dir, name, content string, overwrite bool, stdout io.Writer) error {
	if sameFile(filepath.Join(dir, name), filepath.Join(dir, "AGENTS.md")) {
		ui.Info(stdout, name+" é um link para AGENTS.md — mantido.")
		return nil
	}
	return WriteFile(filepath.Join(dir, name), content, overwrite, stdout)
}

// openCodeConfig is OpenCode's project configuration file and
// openCodeSchema the schema a new one declares.
const (
	openCodeConfig = "opencode.json"
	openCodeSchema = "https://opencode.ai/config.json"
)

// openCodeRulesEntry is the instructions entry that makes OpenCode load
// the project rules (it does not follow AGENTS.md references by itself).
var openCodeRulesEntry = filepath.ToSlash(projectRulesPath)

// readOpenCodeConfig parses dir's opencode.json: nil (and no error) when
// it does not exist.
func readOpenCodeConfig(dir string) (map[string]any, error) {
	data, err := os.ReadFile(filepath.Join(dir, openCodeConfig))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	doc := map[string]any{}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber() // keeps numbers exactly as written
	if err := dec.Decode(&doc); err != nil || doc == nil {
		return nil, fmt.Errorf("%s não é um objeto JSON válido", openCodeConfig)
	}
	return doc, nil
}

// instructionsOf returns doc's "instructions" list, or an error when it
// has another type.
func instructionsOf(doc map[string]any) ([]any, error) {
	raw, ok := doc["instructions"]
	if !ok {
		return nil, nil
	}
	list, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf(`"instructions" de %s não é uma lista`, openCodeConfig)
	}
	return list, nil
}

// writeOpenCodeConfig writes doc as dir's opencode.json.
func writeOpenCodeConfig(dir string, doc map[string]any, stdout io.Writer) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(doc); err != nil {
		return err
	}
	path := filepath.Join(dir, openCodeConfig)
	if err := fsutil.WriteGenerated(path, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("escrever %s: %w", path, err)
	}
	ui.Info(stdout, "Atualizado: "+path)
	return nil
}

// addOpenCodeProjectRules adds the project rules to the instructions of
// dir's opencode.json, creating the file when it is missing. Every other
// key is kept; a file that can't be merged is left alone with a warning.
func addOpenCodeProjectRules(dir string, stdout io.Writer) error {
	if _, err := os.Stat(filepath.Join(dir, "opencode.jsonc")); err == nil {
		if _, err := os.Stat(filepath.Join(dir, openCodeConfig)); os.IsNotExist(err) {
			ui.Warning(stdout, "opencode.jsonc não é alterado pelo Perci (pode ter comentários): adicione \""+openCodeRulesEntry+"\" a \"instructions\" manualmente.")
			return nil
		}
	}
	doc, err := readOpenCodeConfig(dir)
	if err == nil && doc == nil {
		doc = map[string]any{"$schema": openCodeSchema}
	}
	var list []any
	if err == nil {
		list, err = instructionsOf(doc)
	}
	if err != nil {
		ui.Warning(stdout, err.Error()+" — as regras do projeto não foram adicionadas ao OpenCode; corrija o arquivo manualmente.")
		return nil
	}
	if slices.Contains(list, any(openCodeRulesEntry)) {
		ui.Info(stdout, "Já existente: "+filepath.Join(dir, openCodeConfig))
		return nil
	}
	doc["instructions"] = append(list, openCodeRulesEntry)
	return writeOpenCodeConfig(dir, doc, stdout)
}

// removeOpenCodeProjectRules takes the project rules out of the
// instructions of dir's opencode.json, and removes the file when nothing
// else is left in it.
func removeOpenCodeProjectRules(dir string, stdout io.Writer) error {
	doc, err := readOpenCodeConfig(dir)
	if err != nil || doc == nil {
		return nil
	}
	list, err := instructionsOf(doc)
	if err != nil || !slices.Contains(list, any(openCodeRulesEntry)) {
		return nil
	}
	list = slices.DeleteFunc(list, func(v any) bool { return v == any(openCodeRulesEntry) })
	if len(list) == 0 {
		delete(doc, "instructions")
	} else {
		doc["instructions"] = list
	}
	if len(doc) == 0 || (len(doc) == 1 && doc["$schema"] == openCodeSchema) {
		path := filepath.Join(dir, openCodeConfig)
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("remover %s: %w", path, err)
		}
		ui.Info(stdout, "Removido: "+path)
		return nil
	}
	return writeOpenCodeConfig(dir, doc, stdout)
}

// openCodeHasProjectRules reports whether dir's opencode.json loads the
// project rules.
func openCodeHasProjectRules(dir string) bool {
	doc, err := readOpenCodeConfig(dir)
	if err != nil || doc == nil {
		return false
	}
	list, err := instructionsOf(doc)
	return err == nil && slices.Contains(list, any(openCodeRulesEntry))
}
