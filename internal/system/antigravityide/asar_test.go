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

package antigravityide

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// testdata/app.asar was produced by @electron/asar 3.4.1 from a tree holding
// icon.png ("PNGDATA-icon"), a.js and sub/other.txt.
func TestReadAsarFile_RealArchive(t *testing.T) {
	for name, want := range map[string]string{"icon.png": "PNGDATA-icon", "a.js": "hello world"} {
		got, err := readAsarFile(filepath.Join("testdata", "app.asar"), name)
		if err != nil || string(got) != want {
			t.Errorf("%s = %q, %v; want %q", name, got, err, want)
		}
	}
	for _, name := range []string{"missing.png", "sub"} {
		if _, err := readAsarFile(filepath.Join("testdata", "app.asar"), name); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestReadAsarFile_RejectsGarbage(t *testing.T) {
	p := filepath.Join(t.TempDir(), "bad.asar")
	if err := os.WriteFile(p, []byte(strings.Repeat("x", 64)), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readAsarFile(p, "icon.png"); err == nil {
		t.Error("garbage accepted")
	}
	if err := os.WriteFile(p, []byte("short"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readAsarFile(p, "icon.png"); err == nil {
		t.Error("truncated file accepted")
	}
}
