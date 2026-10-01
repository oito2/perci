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
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readGitignore(t *testing.T, dir string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestDetectStacks(t *testing.T) {
	cases := []struct {
		name  string
		files map[string]string
		want  []string
	}{
		{"go", map[string]string{"go.mod": "module x"}, []string{"Go"}},
		{"flutter", map[string]string{"pubspec.yaml": "dependencies:\n  flutter:\n    sdk: flutter\n"}, []string{"Flutter"}},
		{"dart only", map[string]string{"pubspec.yaml": "name: x\n"}, []string{"Dart"}},
		{"moodle plugin", map[string]string{"version.php": "<?php\n$plugin->component = 'local_x';\n"}, []string{"Moodle"}},
		{"php without moodle", map[string]string{"composer.json": "{}", "version.php": "<?php\n$version = 1;\n"}, []string{"PHP"}},
		{"moodle + node", map[string]string{"version.php": "$plugin->component = 'x';", "package.json": "{}"}, []string{"Moodle", "Node.js"}},
		{"python", map[string]string{"requirements.txt": ""}, []string{"Python"}},
		{"java gradle kts", map[string]string{"build.gradle.kts": ""}, []string{"Java"}},
		{"shell", map[string]string{"install.sh": "#!/bin/sh"}, []string{"Shell"}},
		{"nothing", map[string]string{"README.md": "# x"}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, content := range tc.files {
				writeFile(t, dir, name, content)
			}
			got, err := DetectStacks(dir)
			if err != nil {
				t.Fatalf("DetectStacks: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("DetectStacks = %v, want %v", got, tc.want)
			}
		})
	}
}

// Detection only looks at the root: a marker one level down doesn't count.
func TestDetectStacks_IgnoresSubfolders(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "backend"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "backend"), "go.mod", "module x")
	// A directory named like a marker file isn't a marker either.
	if err := os.MkdirAll(filepath.Join(dir, "package.json"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := DetectStacks(dir)
	if err != nil {
		t.Fatalf("DetectStacks: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected nothing detected, got %v", got)
	}
}

func TestDetectStacks_InvalidDir(t *testing.T) {
	if _, err := DetectStacks(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("expected an error for a missing folder")
	}
}

func TestGenerate_NewFileDeduplicatesAcrossSections(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "composer.json", "{}")
	writeFile(t, dir, "package.json", "{}")

	if _, err := Generate(io.Discard, dir); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	got := readGitignore(t, dir)

	for _, want := range []string{".vscode/", ".DS_Store", "vendor/", "*.phar", "node_modules/"} {
		if !strings.Contains(got, want+"\n") {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	// .env is in both the PHP and Node.js sections — written once.
	if n := strings.Count(got, "\n.env\n"); n != 1 {
		t.Errorf(".env appears %d times, want 1:\n%s", n, got)
	}
}

func TestGenerate_GenericWhenNothingDetected(t *testing.T) {
	dir := t.TempDir()
	if _, err := Generate(io.Discard, dir); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	got := readGitignore(t, dir)
	for _, want := range []string{"Environment", "Logs / Temp", "*.log"} {
		if !strings.Contains(got, want) {
			t.Errorf("generic .gitignore missing %q:\n%s", want, got)
		}
	}
}

// An existing .gitignore is kept as is; only missing patterns are
// appended, and a second run adds nothing.
func TestGenerate_MergesIntoExistingFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module x")
	custom := "# mine\nsecret.txt\n*.exe\n.vscode/"
	writeFile(t, dir, ".gitignore", custom)

	if _, err := Generate(io.Discard, dir); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	first := readGitignore(t, dir)

	if !strings.HasPrefix(first, custom+"\n") {
		t.Errorf("existing content was not preserved:\n%s", first)
	}
	for _, line := range []string{"*.exe", ".vscode/"} {
		if n := strings.Count(first, line+"\n"); n != 1 {
			t.Errorf("%q appears %d times, want 1:\n%s", line, n, first)
		}
	}
	if !strings.Contains(first, "*.test\n") || !strings.Contains(first, ".DS_Store\n") {
		t.Errorf("missing patterns were not appended:\n%s", first)
	}

	if _, err := Generate(io.Discard, dir); err != nil {
		t.Fatalf("second Generate: %v", err)
	}
	if second := readGitignore(t, dir); second != first {
		t.Errorf("second run changed the file:\n--- first\n%s\n--- second\n%s", first, second)
	}
}

func TestSectionHeaderWidth(t *testing.T) {
	h := strings.TrimSuffix(sectionHeader("Go"), "\n")
	if n := len([]rune(h)); n != 80 {
		t.Errorf("header is %d runes, want 80: %q", n, h)
	}
}
