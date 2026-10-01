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

package shellrc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRemoveEntry_PreservesExistingPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".bashrc")
	if err := os.WriteFile(path, []byte("keep\n# Go\nexport X=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	RemoveEntry([]string{path}, "# Go", "export X=1")

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("permission after RemoveEntry = %o, want %o (original mode must survive)", got, 0o600)
	}
}

func TestRemoveEntry_WritesExactContentWithoutTempFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".bashrc")
	if err := os.WriteFile(path, []byte("first\n# Go\nexport X=1\nthird"), 0o644); err != nil {
		t.Fatal(err)
	}
	RemoveEntry([]string{path}, "# Go", "export X=1")

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "first\nthird" {
		t.Errorf("content = %q, want %q", got, "first\nthird")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("directory has %d entries after RemoveEntry, want 1 (leftover temp file?)", len(entries))
	}
}

func TestAppendIfMissing_AppendsWhenAbsent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".bashrc")
	if err := os.WriteFile(path, []byte("existing line\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	appended, err := AppendIfMissing(path, "# Go", `export PATH=$PATH:/usr/local/go/bin`)
	if err != nil {
		t.Fatalf("AppendIfMissing: %v", err)
	}
	if !appended {
		t.Error("appended = false, want true")
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "existing line\n\n# Go\nexport PATH=$PATH:/usr/local/go/bin\n"
	if string(got) != want {
		t.Errorf("content = %q, want %q", got, want)
	}
}

func TestAppendIfMissing_SkipsWhenAlreadyPresent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".bashrc")
	entry := `export PATH=$PATH:/usr/local/go/bin`
	if err := os.WriteFile(path, []byte("# Go\n"+entry+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	appended, err := AppendIfMissing(path, "# Go", entry)
	if err != nil {
		t.Fatalf("AppendIfMissing: %v", err)
	}
	if appended {
		t.Error("appended = true, want false (entry already present)")
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "# Go\n"+entry+"\n" {
		t.Errorf("content changed unexpectedly: %q", got)
	}
}

func TestAppendIfMissing_CreatesFileWhenMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".zshrc")

	appended, err := AppendIfMissing(path, "", `export FOO=bar`)
	if err != nil {
		t.Fatalf("AppendIfMissing: %v", err)
	}
	if !appended {
		t.Error("appended = false, want true")
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "\nexport FOO=bar\n" {
		t.Errorf("content = %q, want %q", got, "\nexport FOO=bar\n")
	}
}

func TestRemoveEntry_RemovesCommentAndEntryLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".bashrc")
	entry := `export PATH=$PATH:/usr/local/go/bin`
	content := "before\n# Go\n" + entry + "\nafter\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	results := RemoveEntry([]string{path}, "# Go", entry)
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	if results[0].Err != nil {
		t.Fatalf("RemoveEntry: %v", results[0].Err)
	}
	if !results[0].Changed {
		t.Error("Changed = false, want true")
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "before\nafter\n"
	if string(got) != want {
		t.Errorf("content = %q, want %q", got, want)
	}
}

func TestRemoveEntry_SkipsFilesWithoutEntry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".bashrc")
	if err := os.WriteFile(path, []byte("untouched\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	results := RemoveEntry([]string{path, filepath.Join(dir, "missing")}, "# Go", "export PATH=whatever")
	if len(results) != 0 {
		t.Errorf("results = %v, want empty (nothing matched, one file doesn't exist)", results)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "untouched\n" {
		t.Errorf("content changed unexpectedly: %q", got)
	}
}

func TestDedup(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{"no duplicates", []string{"a", "b", "c"}, []string{"a", "b", "c"}},
		{"consecutive duplicates", []string{"a", "a", "b"}, []string{"a", "b"}},
		{"non-consecutive duplicates", []string{"a", "b", "a", "c", "b"}, []string{"a", "b", "c"}},
		{"empty", nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Dedup(tt.in)
			if len(got) != len(tt.want) {
				t.Fatalf("Dedup(%v) = %v, want %v", tt.in, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("Dedup(%v)[%d] = %q, want %q", tt.in, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestFile(t *testing.T) {
	home := "/home/user"
	cases := []struct {
		shell string
		want  string
	}{
		{"/bin/zsh", filepath.Join(home, ".zshrc")},
		{"/usr/bin/fish", filepath.Join(home, ".config", "fish", "config.fish")},
		{"/bin/bash", filepath.Join(home, ".bashrc")},
		{"", filepath.Join(home, ".bashrc")}, // unset $SHELL falls back to bash
	}
	for _, tc := range cases {
		t.Run(tc.shell, func(t *testing.T) {
			t.Setenv("SHELL", tc.shell)
			if got := File(home); got != tc.want {
				t.Errorf("File(%q) with SHELL=%q = %q, want %q", home, tc.shell, got, tc.want)
			}
		})
	}
}

// Regression: an empty comment used to match — and delete — every blank line.
func TestRemoveEntry_EmptyCommentKeepsBlankLines(t *testing.T) {
	rc := filepath.Join(t.TempDir(), ".bashrc")
	content := "alias a=1\n\nalias b=2\n\neval \"$(starship init bash)\"\n"
	if err := os.WriteFile(rc, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	RemoveEntry([]string{rc}, "", `eval "$(starship init bash)"`)
	got, _ := os.ReadFile(rc)
	if want := "alias a=1\n\nalias b=2\n\n"; string(got) != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// A symlinked rc file (stow/chezmoi) stays a symlink: the target is edited.
func TestAppendAndRemove_KeepSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "dotfiles", "bashrc")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("export A=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, ".bashrc")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	if _, err := AppendIfMissing(link, "# perci", "export B=2"); err != nil {
		t.Fatal(err)
	}
	RemoveEntry([]string{link}, "", "export A=1")

	info, err := os.Lstat(link)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("the rc symlink was replaced by a regular file")
	}
	got, _ := os.ReadFile(target)
	if !strings.Contains(string(got), "export B=2") || strings.Contains(string(got), "export A=1") {
		t.Errorf("edits didn't reach the symlink target: %q", got)
	}
}
