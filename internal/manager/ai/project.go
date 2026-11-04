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
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/oito2/perci/internal/fsutil"
	"github.com/oito2/perci/internal/ui"
)

// projectRulesPath is the project's own rules file, relative to the
// project root. Perci creates it once and never rewrites it.
var projectRulesPath = filepath.Join(".instructions", "PROJECT.md")

// legacyTemplates are AGENTS.md rule sets earlier Perci versions
// generated; their lines count as generated when migrating an AGENTS.md.
var legacyTemplates = []string{
	"templates/legacy/BASIC-1.0.md",
	"templates/legacy/BASIC-1.2.md",
}

// legacyStandardsHeaders are the standards-list introductions earlier
// versions wrote into AGENTS.md.
var legacyStandardsHeaders = []string{
	"The standards below are **mandatory** for the tasks they cover. **Before starting a task one of them applies to, read the whole file** (Claude Code already loads them through the `@` imports; Codex, Antigravity and other agents must open the file themselves). When several apply, follow all of them.",
	"The standards below are **mandatory** for the tasks they cover. **Before starting a task one of them applies to, read the whole file** (Claude Code already loads them through the `@` imports; Codex, Antigravity and other agents must open the file themselves). When several apply, follow all of them. They complement the rules above and never override them.",
}

// standardLineRe matches a standards-list entry of any version:
// "- <applies to>: @.instructions/X.md" or "- <applies to>: `.instructions/X.md`…".
var standardLineRe = regexp.MustCompile("^- .+: (@\\.instructions/[A-Za-z0-9-]+\\.md|`\\.instructions/[A-Za-z0-9-]+\\.md`.*)$")

// generatedLines returns every line Perci writes, or wrote, into AGENTS.md.
func generatedLines() (map[string]bool, error) {
	lines := map[string]bool{}
	add := func(text string) {
		for _, l := range strings.Split(text, "\n") {
			lines[strings.TrimRight(l, " \t\r")] = true
		}
	}
	for _, path := range append([]string{"templates/BASIC.md"}, legacyTemplates...) {
		tpl, err := readTpl(path)
		if err != nil {
			return nil, err
		}
		add(tpl)
	}
	add(standardsHeader(false))
	add(standardsHeader(true))
	for _, h := range legacyStandardsHeaders {
		add(h)
	}
	return lines, nil
}

// projectContent returns the lines of the AGENTS.md text agents that no
// Perci version generated — rules the project wrote by hand — with
// consecutive lines kept together and a blank line between groups.
func projectContent(agents string) (string, error) {
	known, err := generatedLines()
	if err != nil {
		return "", err
	}
	var groups [][]string
	var cur []string
	flush := func() {
		if len(cur) > 0 {
			groups = append(groups, cur)
			cur = nil
		}
	}
	for _, raw := range strings.Split(agents, "\n") {
		l := strings.TrimRight(raw, " \t\r")
		switch {
		case strings.TrimSpace(l) == "":
			flush()
		case known[l] || standardLineRe.MatchString(l) || agentsMarkerRe.MatchString(l):
			flush()
		default:
			cur = append(cur, l)
		}
	}
	flush()
	parts := make([]string, len(groups))
	for i, g := range groups {
		parts[i] = strings.Join(g, "\n")
	}
	return strings.Join(parts, "\n\n"), nil
}

// ensureProjectRules creates .instructions/PROJECT.md when it is missing
// and, when AGENTS.md is about to be regenerated (overwrite), moves into
// it the rules a hand-edited AGENTS.md has that Perci did not generate, so
// regenerating loses nothing. An existing PROJECT.md is only appended to,
// never with rules it already holds.
func ensureProjectRules(dir string, overwrite bool, stdout io.Writer) error {
	path := filepath.Join(dir, projectRulesPath)
	current, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("ler %s: %w", path, err)
	}
	exists := err == nil

	moved := ""
	if agents, err := os.ReadFile(filepath.Join(dir, "AGENTS.md")); err == nil && overwrite {
		if moved, err = projectContent(string(agents)); err != nil {
			return err
		}
	}
	if exists && (moved == "" || strings.Contains(string(current), moved)) {
		return nil
	}

	content := string(current)
	if !exists {
		if content, err = readTpl("templates/PROJECT.md"); err != nil {
			return err
		}
	}
	if moved != "" {
		content = strings.TrimRight(content, "\n") + "\n\n## Moved from AGENTS.md (" + time.Now().Format("2006-01-02") + ")\n\n" + moved + "\n"
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := fsutil.WriteGenerated(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("escrever %s: %w", path, err)
	}
	if moved != "" {
		ui.Warning(stdout, "Regras escritas à mão no AGENTS.md foram movidas para "+path+" — confira o arquivo; o AGENTS.md agora é regenerado pelo Perci.")
	} else {
		ui.Info(stdout, "Criado: "+path)
	}
	return nil
}
