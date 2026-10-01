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
	"embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/oito2/perci/internal/config"
	"github.com/oito2/perci/internal/ui"
)

//go:embed all:templates
var templateFS embed.FS

// Model represents a project type with its own instruction file.
type Model struct {
	Name        string
	Instruction string // path inside templateFS
	// AppliesTo says, in English (AGENTS.md's language), which tasks the
	// standard covers — agents without "@" import support (Codex, and
	// Antigravity, which turns a bare "@path" into a path reference only)
	// rely on it to decide which file to read before starting a task.
	AppliesTo string
	// Requires names another model this one only makes sense on top of —
	// GenerateSharedFiles pulls it in automatically when it isn't selected.
	Requires string
	// Legacy lists filenames this model's instruction file had under
	// .instructions/ before being renamed: they still count as "active" in
	// DetectActiveModels, and are deleted once the current file is written
	// (or when the model is removed), so a project migrates on its next Aplicar.
	Legacy []string
}

// docsBase is the base documentation standard every "Documentação: *"
// complement builds on.
const docsBase = "Documentação de projeto"

// Go, MCP Server, Go MCP Server and PHP were removed (user decision) —
// their content was mostly generic language/protocol knowledge (idiomatic
// PSR/PHP 8.x, the MCP protocol itself) already covered by equivalent
// third-party skills now installable via "Dev Tools :: IA: SKILLs" (PHP:
// Specialist/Pro 8.3+, MCP Server Dev/Dev in Go, Go: Code Style/Golang
// Pro) — unlike Moodle/Dart+Flutter-oito2, which stay here because
// they're Perci/oito2's own convention, not something a generic skill
// already covers.
var models = []Model{
	{Name: "Linux Bash", Instruction: "templates/instructions/BASH.md",
		AppliesTo: "Writing or reviewing Bash/shell scripts, CLI tools and installers"},
	{Name: "Moodle", Instruction: "templates/instructions/MOODLE.md",
		AppliesTo: "Developing Moodle plugins (file structure, Moodle APIs, architecture)"},
	{Name: "Dart + Flutter - oito2", Instruction: "templates/instructions/FLUTTER.md",
		AppliesTo: "Building Dart/Flutter apps (oito2 stack: flutter_bloc, shadcn_ui, l10n)"},
	{Name: "MySQL/MariaDB", Instruction: "templates/instructions/MYSQL-MARIADB.md",
		AppliesTo: "MySQL/MariaDB schema design, migrations, query tuning and configuration"},
	{Name: "Landing Page Design", Instruction: "templates/instructions/LANDING-PAGE.md",
		AppliesTo: "Designing or building landing pages"},
	{Name: docsBase, Instruction: "templates/instructions/PROJECT-DOCUMENTATION.md", Legacy: []string{"DOCUMENTATION-SITE.md"},
		AppliesTo: "Creating or updating project documentation (README, docs/, CONTRIBUTING, CODE_OF_CONDUCT, LICENSE, license headers)"},
	{Name: "Documentação: Servidor MCP", Instruction: "templates/instructions/PROJECT-DOCUMENTATION-MCP.md", Requires: docsBase,
		AppliesTo: "Documentation of an MCP server — together with PROJECT-DOCUMENTATION.md"},
	{Name: "Documentação: Plugin Moodle", Instruction: "templates/instructions/PROJECT-DOCUMENTATION-MOODLE.md", Requires: docsBase,
		AppliesTo: "Documentation of a Moodle plugin — together with PROJECT-DOCUMENTATION.md"},
	{Name: "Documentação: Aplicação Desktop", Instruction: "templates/instructions/PROJECT-DOCUMENTATION-DESKTOP.md", Requires: docsBase,
		AppliesTo: "Documentation of a desktop application — together with PROJECT-DOCUMENTATION.md"},
	{Name: "Documentação: Aplicação Mobile", Instruction: "templates/instructions/PROJECT-DOCUMENTATION-MOBILE.md", Requires: docsBase,
		AppliesTo: "Documentation of a mobile application — together with PROJECT-DOCUMENTATION.md"},
	{Name: "Documentação: Aplicação Web", Instruction: "templates/instructions/PROJECT-DOCUMENTATION-WEB.md", Requires: docsBase,
		AppliesTo: "Documentation of a web application — together with PROJECT-DOCUMENTATION.md"},
}

// withRequired returns active plus every model an active one Requires that
// wasn't selected, in catalog order (so AGENTS.md always references a base
// before its complements), and the names of the models it added.
func withRequired(active []Model) (resolved []Model, added []string) {
	selected := make(map[string]bool, len(active))
	for _, m := range active {
		selected[m.Name] = true
	}
	for _, m := range active {
		if m.Requires != "" && !selected[m.Requires] {
			selected[m.Requires] = true
			added = append(added, m.Requires)
		}
	}
	if len(added) == 0 {
		return active, nil
	}
	for _, m := range models {
		if selected[m.Name] {
			resolved = append(resolved, m)
		}
	}
	return resolved, added
}

// Models returns the list of all supported project models.
func Models() []Model {
	return models
}

// DetectActiveModels checks the .instructions/ directory under dir and
// returns which models already have their instruction file present. dir
// is the folder the user picked (empty means the process's current
// directory — same convention as internal/manager/repo and
// internal/manager/gitignore).
func DetectActiveModels(dir string) map[string]bool {
	present := make(map[string]bool, len(models))
	for _, m := range models {
		for _, name := range append([]string{filepath.Base(m.Instruction)}, m.Legacy...) {
			if _, err := os.Stat(filepath.Join(dir, ".instructions", name)); err == nil {
				present[m.Name] = true
				break
			}
		}
	}
	return present
}

var (
	moodleVersionRe    = regexp.MustCompile(`\$version\s*=\s*([\d.]+)\s*;`)
	moodleReleaseRe    = regexp.MustCompile(`\$release\s*=\s*'([^']+)'\s*;`)
	moodleMajorMinorRe = regexp.MustCompile(`^(\d+\.\d+)`)
)

// maxVersionPHPSize caps how much of version.php is read — it's a project
// file the user controls, not something perci generates, so a corrupted
// file or a symlink pointing somewhere huge shouldn't be read in full before
// the regexes above even run. 1 MiB is generously above any real version.php.
const maxVersionPHPSize = 1 << 20

// readCapped reads at most maxVersionPHPSize bytes from path.
func readCapped(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return io.ReadAll(io.LimitReader(f, maxVersionPHPSize))
}

// detectMoodleVars reads version.php (or public/version.php, the Moodle 5.1+
// layout) from projectDir and extracts the values MOODLE.md's placeholders need.
// ok is false when the file is missing or doesn't match the expected format.
func detectMoodleVars(projectDir string) (vars map[string]string, ok bool) {
	data, err := readCapped(filepath.Join(projectDir, "version.php"))
	if err != nil {
		data, err = readCapped(filepath.Join(projectDir, "public", "version.php"))
		if err != nil {
			return nil, false
		}
	}

	versionMatch := moodleVersionRe.FindSubmatch(data)
	releaseMatch := moodleReleaseRe.FindSubmatch(data)
	if versionMatch == nil || releaseMatch == nil {
		return nil, false
	}

	majorMinor := moodleMajorMinorRe.FindString(string(releaseMatch[1]))
	if majorMinor == "" {
		return nil, false
	}

	return map[string]string{
		"MOODLE_VERSION":     majorMinor,
		"MOODLE_FULLVERSION": string(versionMatch[1]),
		"MOODLE_PATH":        projectDir,
		"WORKSPACE_PATH":     workspacePath(),
	}, true
}

// moodleVars resolves MOODLE.md's placeholders from dir's version.php. When
// it can't be found or parsed, the file goes out with its placeholders
// unsubstituted and the user is told so (the warning used to go to a log
// the GUI never passed, so nobody saw it).
func moodleVars(stdout io.Writer, dir string) map[string]string {
	projectDir := dir
	if projectDir == "" {
		if wd, err := os.Getwd(); err == nil {
			projectDir = wd
		} else {
			projectDir = "."
		}
	}
	if vars, ok := detectMoodleVars(projectDir); ok {
		return vars
	}
	ui.Warning(stdout, "version.php não encontrado — MOODLE.md gerado sem substituir {{MOODLE_VERSION}} e os demais marcadores.")
	return nil
}

// workspacePath is the configured workspace folder (config.Workspace:
// ~/workspace when unset), or "" if the config can't be loaded.
func workspacePath() string {
	cfg, err := config.Load()
	if err != nil {
		return ""
	}
	ws, err := cfg.Workspace()
	if err != nil {
		return ""
	}
	return ws
}

// standardsHeader opens AGENTS.md's list of active standards. Only Claude
// Code resolves "@path" imports into content, so the list also has to work
// as plain instructions: each entry says which tasks it covers, and agents
// are told to read the file themselves before such a task. Inlining the
// files instead isn't viable — it would overflow Codex's 32 KiB AGENTS.md
// budget and Antigravity's 24 KB per-rule cap with just two standards.
const standardsHeader = `

## Language-Specific Standards

The standards below are **mandatory** for the tasks they cover. **Before starting a task one of them applies to, read the whole file** (Claude Code already loads them through the ` + "`@`" + ` imports; Codex, Antigravity and other agents must open the file themselves). When several apply, follow all of them.
`

// GenerateSharedFiles writes CLAUDE.md, AGENTS.md and the ignore/exclude files
// referencing all active models, and physically writes each active model's
// instruction file to dir/.instructions/. If overwrite is true, existing
// files are replaced without confirmation. dir empty means the current
// directory (same convention as the rest of this file). Progress and
// warnings go to stdout (the GUI's terminal panel).
func GenerateSharedFiles(dir string, active []Model, overwrite bool, stdout io.Writer) error {
	active, added := withRequired(active)
	for _, name := range added {
		ui.Info(stdout, "Contexto \""+name+"\" incluído automaticamente (necessário para os complementos selecionados).")
	}

	rawBasic, err := readTpl("templates/BASIC.md")
	if err != nil {
		return err
	}
	claudeStub, err := readTpl("templates/CLAUDE-STUB.md")
	if err != nil {
		return err
	}

	// Build the @-reference block (one line per active model) and write each
	// referenced instruction file to disk — without this, AGENTS.md ends up
	// pointing at .instructions/*.md files that were never actually created.
	var refBlock strings.Builder
	refBlock.WriteString(standardsHeader)
	for _, m := range active {
		fmt.Fprintf(&refBlock, "\n- %s: @.instructions/%s", m.AppliesTo, filepath.Base(m.Instruction))

		var vars map[string]string
		if m.Name == "Moodle" {
			vars = moodleVars(stdout, dir)
		}
		if err := WriteInstruction(dir, m, overwrite, stdout, vars); err != nil {
			return err
		}
	}

	agentsContent := rawBasic
	if len(active) > 0 {
		agentsContent += refBlock.String()
	}

	type entry struct {
		filename string
		content  string
	}
	files := []entry{
		{"CLAUDE.md", claudeStub},
		{"AGENTS.md", agentsContent},
	}
	for _, f := range files {
		if err := WriteFile(filepath.Join(dir, f.filename), f.content, overwrite, stdout); err != nil {
			return err
		}
	}

	// Ignore files (shared, always regenerated). .geminiignore is intentionally
	// not generated: Antigravity ignores it and Gemini CLI is no longer a
	// generation target now that GEMINI.md doesn't exist.
	aiexclude, err := readTpl("templates/.aiexclude")
	if err != nil {
		return err
	}
	for _, name := range []string{".aiexclude", ".claudeignore"} {
		if err := WriteFile(filepath.Join(dir, name), aiexclude, overwrite, stdout); err != nil {
			return err
		}
	}

	// Remove instruction files for models that were active before this run
	// but aren't selected anymore — otherwise unchecking a model left its
	// .instructions/*.md orphaned (unreferenced from AGENTS.md, but never
	// deleted). Only a copy still identical to what Perci generates is
	// deleted: DetectActiveModels goes by filename, so a hand-written or
	// edited BASH.md is "active" too, and deleting it would lose the user's
	// work.
	detected := DetectActiveModels(dir)
	activeNow := make(map[string]bool, len(active))
	for _, m := range active {
		activeNow[m.Name] = true
	}
	for _, m := range models {
		if !detected[m.Name] || activeNow[m.Name] {
			continue
		}
		current := filepath.Join(dir, ".instructions", filepath.Base(m.Instruction))
		if _, err := os.Stat(current); err == nil && !isPristine(dir, m) {
			ui.Warning(stdout, ".instructions/"+filepath.Base(m.Instruction)+" foi editado e foi mantido — apague-o manualmente se não precisar mais dele.")
			if overwrite {
				removeLegacy(dir, m, stdout)
			}
			continue
		}
		if _, err := os.Stat(current); err != nil && !overwrite {
			// Only an old-named copy is left (Model.Legacy): same rule as
			// WriteInstruction — removed only on an overwrite run.
			continue
		}
		RemoveInstruction(dir, m, stdout)
	}

	return nil
}

// WriteFile writes content to path, returning an error if it fails.
// If overwrite is false and the file exists, it skips writing it and does not error.
func WriteFile(path, content string, overwrite bool, stdout io.Writer) error {
	if _, err := os.Stat(path); err == nil && !overwrite {
		return nil
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("escrever %s: %w", path, err)
	}
	ui.Info(stdout, "Criado: "+path)
	return nil
}

// WriteInstruction writes the instruction file for a given model under
// dir/.instructions/.
func WriteInstruction(dir string, model Model, overwrite bool, stdout io.Writer, vars map[string]string) error {
	instrDir := filepath.Join(dir, ".instructions")
	if err := os.MkdirAll(instrDir, 0o755); err != nil {
		return err
	}
	dest := filepath.Join(instrDir, filepath.Base(model.Instruction))
	content, err := readTpl(model.Instruction)
	if err != nil {
		return err
	}
	for k, v := range vars {
		content = strings.ReplaceAll(content, "{{"+k+"}}", v)
	}
	if err := WriteFile(dest, content, overwrite, stdout); err != nil {
		return err
	}
	// Without overwrite the caller asked not to clobber customizations, and
	// a legacy copy may be one — leave it for an explicit overwrite run.
	if overwrite {
		removeLegacy(dir, model, stdout)
	}
	return nil
}

// removeLegacy deletes the pre-rename copies of model's instruction file
// under dir/.instructions/ (see Model.Legacy).
func removeLegacy(dir string, model Model, stdout io.Writer) {
	for _, name := range model.Legacy {
		dest := filepath.Join(dir, ".instructions", name)
		if err := os.Remove(dest); err == nil {
			ui.Info(stdout, "Removido (nome antigo): "+dest)
		}
	}
}

// RemoveInstruction deletes the instruction file of a model under dir/.instructions/.
func RemoveInstruction(dir string, model Model, stdout io.Writer) {
	dest := filepath.Join(dir, ".instructions", filepath.Base(model.Instruction))
	if err := os.Remove(dest); err != nil && !os.IsNotExist(err) {
		ui.Warning(stdout, "Falha ao remover "+dest+": "+err.Error())
	} else if err == nil {
		ui.Info(stdout, "Removido: "+dest)
	}
	removeLegacy(dir, model, stdout)
}

// isPristine reports whether dir's copy of model's instruction file is
// still exactly what Perci writes: the template itself, or — for MOODLE.md
// — the template with this folder's version.php values substituted.
func isPristine(dir string, model Model) bool {
	got, err := os.ReadFile(filepath.Join(dir, ".instructions", filepath.Base(model.Instruction)))
	if err != nil {
		return false
	}
	tpl, err := readTpl(model.Instruction)
	if err != nil {
		return false
	}
	if string(got) == tpl {
		return true
	}
	if model.Name == "Moodle" {
		projectDir := dir
		if projectDir == "" {
			projectDir = "."
		}
		if vars, ok := detectMoodleVars(projectDir); ok {
			for k, v := range vars {
				tpl = strings.ReplaceAll(tpl, "{{"+k+"}}", v)
			}
			return string(got) == tpl
		}
	}
	return false
}

func readTpl(path string) (string, error) {
	data, err := templateFS.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("template %s não encontrado: %w", path, err)
	}
	return string(data), nil
}
