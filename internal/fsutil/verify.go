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
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
	"strings"
)

// VerifyFile reports an error unless the digest h computes over the file
// at path equals want (hex, case-insensitive, surrounding spaces ignored).
// h must be fresh (e.g. sha256.New()). Every downloaded artifact that is
// checked against a published or pinned checksum goes through here.
func VerifyFile(path string, h hash.Hash, want string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("abrir %s para verificação: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("calcular checksum: %w", err)
	}
	if got := hex.EncodeToString(h.Sum(nil)); !strings.EqualFold(got, strings.TrimSpace(want)) {
		return fmt.Errorf("checksum não confere: esperado %s, obtido %s", want, got)
	}
	return nil
}
