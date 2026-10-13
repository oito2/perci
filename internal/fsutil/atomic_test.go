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

package fsutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFileAtomic_ForcesModeOnExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "export.yaml")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteFileAtomic(path, []byte("secret"), 0o600); err != nil {
		t.Fatalf("WriteFileAtomic: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("mode = %o, want 600", got)
	}
	if data, _ := os.ReadFile(path); string(data) != "secret" {
		t.Errorf("content = %q", data)
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 {
		t.Errorf("temp file left behind: %v", entries)
	}
}

func TestWriteFileAtomic_MissingDir(t *testing.T) {
	if err := WriteFileAtomic(filepath.Join(t.TempDir(), "missing", "f"), nil, 0o600); err == nil {
		t.Fatal("expected an error for a missing directory")
	}
}

// WriteGenerated writes through a symlink and keeps the previous different
// content as .perci-bak; identical content leaves no backup.
func TestWriteGenerated(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "AGENTS.md")
	link := filepath.Join(dir, "CLAUDE.md")
	if err := os.WriteFile(target, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("AGENTS.md", link); err != nil {
		t.Fatal(err)
	}
	if err := WriteGenerated(link, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Lstat(link); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("the symlink was replaced: %v", err)
	}
	if got, _ := os.ReadFile(target); string(got) != "new" {
		t.Errorf("target = %q", got)
	}
	if got, _ := os.ReadFile(target + ".perci-bak"); string(got) != "old" {
		t.Errorf("backup = %q", got)
	}
	if err := os.Remove(target + ".perci-bak"); err != nil {
		t.Fatal(err)
	}
	if err := WriteGenerated(target, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(target + ".perci-bak"); !os.IsNotExist(err) {
		t.Error("identical content must not create a backup")
	}
}

// WriteThroughLink writes the link's target, keeps the link and leaves no
// backup.
func TestWriteThroughLink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "mcp_config.json")
	link := filepath.Join(dir, "link.json")
	if err := os.WriteFile(target, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if err := WriteThroughLink(link, []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Lstat(link); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("the symlink was replaced: %v", err)
	}
	if got, _ := os.ReadFile(target); string(got) != "new" {
		t.Errorf("target = %q", got)
	}
	if _, err := os.Stat(target + ".perci-bak"); !os.IsNotExist(err) {
		t.Error("WriteThroughLink must not leave a backup")
	}
}
