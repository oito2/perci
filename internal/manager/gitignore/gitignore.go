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

package gitignore

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/oito2/perci/internal/fsutil"
	"github.com/oito2/perci/internal/ui"
)

// section is one titled block of .gitignore patterns.
type section struct {
	Title string
	Lines []string
}

// stack is one entry of the project-type dictionary: how to recognize it
// from the files at the project root, and what it adds to .gitignore.
type stack struct {
	Name    string
	Detect  func(dir string) bool
	Section section
}

var (
	sectionEditors = section{"Editors", []string{".vscode/", ".idea/", "*.swp", "*.swo", "*~"}}
	sectionOS      = section{"OS", []string{".DS_Store", "Thumbs.db"}}

	// genericSections replace the stack sections when no project type is
	// recognized.
	genericSections = []section{
		{"Environment", []string{".env", ".env.*", "!.env.example"}},
		{"Logs / Temp", []string{"*.log", "*.tmp", "*.bak", "logs/"}},
		{"Build / Dependencies", []string{"vendor/", "node_modules/", "dist/", "build/"}},
	}
)

// stacks is the project-type dictionary, in output order. Detection only
// looks at the project root (decided with the user on 2026-09-29): nested
// projects would need patterns relative to their own folder.
var stacks = []stack{
	{"Go", hasFile("go.mod"),
		section{"Go", []string{"*.exe", "*.test", "*.out", "dist/", "vendor/"}}},
	{"Flutter", fileContains("pubspec.yaml", "sdk: flutter"),
		section{"Flutter", []string{
			".dart_tool/", ".flutter-plugins", ".flutter-plugins-dependencies", ".pub-cache/", ".pub/",
			"build/", "coverage/", "*.iml",
			"android/.gradle/", "android/local.properties",
			"ios/Pods/", "ios/Flutter/Generated.xcconfig", "ios/Flutter/flutter_export_environment.sh",
		}}},
	{"Dart", all(hasFile("pubspec.yaml"), not(fileContains("pubspec.yaml", "sdk: flutter"))),
		section{"Dart", []string{".dart_tool/", "build/", "coverage/"}}},
	{"Moodle", fileContains("version.php", "$plugin->component"),
		section{"Moodle", []string{"node_modules/", "vendor/", ".phpunit.result.cache"}}},
	{"PHP", hasFile("composer.json"),
		section{"PHP", []string{"vendor/", ".env", ".env.*", "!.env.example", "*.phar"}}},
	{"Node.js", hasFile("package.json"),
		section{"Node.js", []string{
			"node_modules/", "dist/", "build/", ".env", ".env.*", "!.env.example",
			"npm-debug.log*", "yarn-debug.log*", ".pnp", ".pnp.js",
		}}},
	{"Python", hasFile("pyproject.toml", "requirements.txt", "setup.py"),
		section{"Python", []string{
			"__pycache__/", "*.py[cod]", "*.egg-info/", ".venv/", "venv/", "dist/", "build/", ".pytest_cache/",
		}}},
	{"Ruby", hasFile("Gemfile"),
		section{"Ruby", []string{".bundle/", "vendor/bundle/", "*.gem", ".env", ".env.*", "!.env.example"}}},
	// Cargo.lock stays versioned: Cargo's own guidance is to commit it for
	// every package, libraries included.
	{"Rust", hasFile("Cargo.toml"),
		section{"Rust", []string{"target/"}}},
	{"Java", hasFile("pom.xml", "build.gradle", "build.gradle.kts"),
		section{"Java", []string{"*.class", "*.jar", "!gradle/wrapper/gradle-wrapper.jar", "*.war", "target/", ".gradle/", "build/"}}},
	{"Shell", hasGlob("*.sh"),
		section{"Shell", []string{".env", ".env.*", "!.env.example"}}},
}

// Generate creates dir/.gitignore, or merges into an existing one: every
// pattern already present stays untouched and only the missing ones are
// appended, under their section title — running it again is harmless.
// The project type is detected from files at dir's root (see stacks);
// nothing recognized means the generic sections.
func Generate(stdout io.Writer, dir string) (string, error) {
	names, err := DetectStacks(dir)
	if err != nil {
		return "", fmt.Errorf("detectar tipo de projeto: %w", err)
	}

	sections := []section{sectionEditors, sectionOS}
	if len(names) == 0 {
		ui.Info(stdout, "Nenhum tipo de projeto reconhecido — usando o .gitignore genérico.")
		sections = append(sections, genericSections...)
	} else {
		ui.Info(stdout, "Tipo de projeto detectado: "+strings.Join(names, ", "))
		for _, s := range stacks {
			if slices.Contains(names, s.Name) {
				sections = append(sections, s.Section)
			}
		}
	}

	dest := filepath.Join(dir, ".gitignore")
	existing, err := os.ReadFile(dest)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("ler .gitignore: %w", err)
	}

	addition, added := buildAddition(string(existing), sections)
	if added == 0 {
		ui.Success(stdout, ".gitignore já contém todos os padrões — nada a acrescentar.")
		return dest, nil
	}

	content := string(existing)
	if content != "" {
		if !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		content += "\n"
	}
	content += addition

	if err := fsutil.WriteFileAtomic(dest, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("escrever .gitignore: %w", err)
	}

	if len(existing) == 0 {
		ui.Success(stdout, ".gitignore criado em "+dest)
	} else {
		ui.Success(stdout, fmt.Sprintf(".gitignore atualizado em %s (%d padrão(ões) acrescentado(s)).", dest, added))
	}
	return dest, nil
}

// DetectStacks returns the names of every project type recognized at
// dir's root, in dictionary order.
func DetectStacks(dir string) ([]string, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s não é uma pasta", dir)
	}

	var names []string
	for _, s := range stacks {
		if s.Detect(dir) {
			names = append(names, s.Name)
		}
	}
	return names, nil
}

// buildAddition renders the sections, skipping every pattern already in
// existing (compared line by line, trimmed) or emitted by an earlier
// section, and every section left with no pattern. Returns the text and
// how many patterns it adds.
func buildAddition(existing string, sections []section) (string, int) {
	seen := map[string]bool{}
	for _, line := range strings.Split(existing, "\n") {
		seen[strings.TrimSpace(line)] = true
	}

	var sb strings.Builder
	added := 0
	for _, sec := range sections {
		var missing []string
		for _, line := range sec.Lines {
			if !seen[line] {
				seen[line] = true
				missing = append(missing, line)
			}
		}
		if len(missing) == 0 {
			continue
		}
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(sectionHeader(sec.Title))
		for _, line := range missing {
			sb.WriteString(line + "\n")
		}
		added += len(missing)
	}
	return sb.String(), added
}

// sectionHeader renders "# ─── Title ───…" padded to 80 columns.
func sectionHeader(title string) string {
	h := "# ─── " + title + " "
	if n := 80 - utf8.RuneCountInString(h); n > 0 {
		h += strings.Repeat("─", n)
	}
	return h + "\n"
}

// ── Detection helpers ─────────────────────────────────────────────────────────

// hasFile matches when any of names exists as a regular file at the root.
func hasFile(names ...string) func(string) bool {
	return func(dir string) bool {
		for _, n := range names {
			if info, err := os.Stat(filepath.Join(dir, n)); err == nil && info.Mode().IsRegular() {
				return true
			}
		}
		return false
	}
}

// fileContains matches when name exists at the root and contains substr.
func fileContains(name, substr string) func(string) bool {
	return func(dir string) bool {
		data, err := os.ReadFile(filepath.Join(dir, name))
		return err == nil && strings.Contains(string(data), substr)
	}
}

// hasGlob matches when pattern matches at least one regular file at the root.
func hasGlob(pattern string) func(string) bool {
	return func(dir string) bool {
		matches, _ := filepath.Glob(filepath.Join(dir, pattern))
		for _, m := range matches {
			if info, err := os.Stat(m); err == nil && info.Mode().IsRegular() {
				return true
			}
		}
		return false
	}
}

func all(fns ...func(string) bool) func(string) bool {
	return func(dir string) bool {
		for _, fn := range fns {
			if !fn(dir) {
				return false
			}
		}
		return true
	}
}

func not(fn func(string) bool) func(string) bool {
	return func(dir string) bool { return !fn(dir) }
}
