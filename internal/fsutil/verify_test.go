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
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"
)

func TestVerifyFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(p, []byte("abc"), 0o600); err != nil {
		t.Fatal(err)
	}
	const sum = "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	if err := VerifyFile(p, sha256.New(), " "+sum+"\n"); err != nil {
		t.Errorf("matching sum refused: %v", err)
	}
	if err := VerifyFile(p, sha256.New(), "BA7816BF"+sum[8:]); err != nil {
		t.Errorf("case must not matter: %v", err)
	}
	if err := VerifyFile(p, sha256.New(), sum[:63]+"0"); err == nil {
		t.Error("wrong sum accepted")
	}
	if err := VerifyFile(filepath.Join(t.TempDir(), "missing"), sha256.New(), sum); err == nil {
		t.Error("missing file accepted")
	}
}
