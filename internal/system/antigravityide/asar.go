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
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

const (
	// maxAsarHeader and maxAsarFile bound what readAsarFile allocates.
	maxAsarHeader = 64 << 20
	maxAsarFile   = 32 << 20
)

// asarEntry is one node of an asar archive's JSON header: a directory
// (Files) or a file (Size, Offset). Unpacked files live outside the archive.
type asarEntry struct {
	Files    map[string]asarEntry `json:"files"`
	Size     int64                `json:"size"`
	Offset   string               `json:"offset"`
	Unpacked bool                 `json:"unpacked"`
}

// readAsarFile returns the contents of the file name at the top level of the
// asar archive at archivePath. The archive starts with four little-endian
// uint32 (pickle size 4, header size, payload size, JSON length), then the
// JSON header; file data begins right after the header, at 8+header size.
func readAsarFile(archivePath, name string) ([]byte, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	var prefix [16]byte
	if _, err := io.ReadFull(f, prefix[:]); err != nil {
		return nil, fmt.Errorf("ler cabeçalho do app.asar: %w", err)
	}
	headerSize := int64(binary.LittleEndian.Uint32(prefix[4:8]))
	jsonLen := int64(binary.LittleEndian.Uint32(prefix[12:16]))
	if binary.LittleEndian.Uint32(prefix[0:4]) != 4 || jsonLen <= 0 || jsonLen > headerSize || headerSize > maxAsarHeader {
		return nil, fmt.Errorf("app.asar com formato inesperado")
	}
	raw := make([]byte, jsonLen)
	if _, err := io.ReadFull(f, raw); err != nil {
		return nil, fmt.Errorf("ler índice do app.asar: %w", err)
	}
	var root asarEntry
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, fmt.Errorf("índice do app.asar inválido: %w", err)
	}
	entry, ok := root.Files[name]
	if !ok || entry.Files != nil || entry.Unpacked {
		return nil, fmt.Errorf("app.asar não continha /%s", name)
	}
	var offset int64
	if _, err := fmt.Sscanf(entry.Offset, "%d", &offset); err != nil || offset < 0 || entry.Size < 0 || entry.Size > maxAsarFile {
		return nil, fmt.Errorf("entrada /%s do app.asar inválida", name)
	}
	data := make([]byte, entry.Size)
	if _, err := f.ReadAt(data, 8+headerSize+offset); err != nil {
		return nil, fmt.Errorf("ler /%s do app.asar: %w", name, err)
	}
	return data, nil
}
