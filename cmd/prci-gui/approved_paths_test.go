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

package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRequireApprovedPath(t *testing.T) {
	dir := t.TempDir()
	picked := filepath.Join(dir, "export.yaml")

	if err := requireApprovedPath(picked); err == nil {
		t.Fatal("a path never returned by a dialog must be rejected")
	}
	if err := requireApprovedPath(""); err == nil {
		t.Fatal("an empty path must be rejected")
	}

	if _, err := approvePath(picked, nil); err != nil {
		t.Fatal(err)
	}
	if err := requireApprovedPath(filepath.Join(dir, ".", "export.yaml")); err != nil {
		t.Errorf("an equivalent (cleaned) path should be accepted: %v", err)
	}

	// Cancelled or failed dialogs approve nothing.
	cancelled := filepath.Join(dir, "cancelled")
	_, _ = approvePath("", nil)
	_, _ = approvePath(cancelled, errors.New("dialog error"))
	if err := requireApprovedPath(cancelled); err == nil {
		t.Error("a path from a failed dialog must not be approved")
	}
}

func TestSaveTextFile_OnlyApprovedPathsAndMode0600(t *testing.T) {
	s := &DockerService{}
	target := filepath.Join(t.TempDir(), "logs.txt")

	if err := s.SaveTextFile(target, "log"); err == nil {
		t.Fatal("SaveTextFile must refuse a path not picked in a dialog")
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatal("nothing should be written for a refused path")
	}

	if err := os.WriteFile(target, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _ = approvePath(target, nil)
	if err := s.SaveTextFile(target, "log"); err != nil {
		t.Fatalf("SaveTextFile: %v", err)
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("mode = %o, want 600 even over an existing 0644 file", got)
	}
}
