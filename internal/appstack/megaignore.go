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

package appstack

// MEGA sync exclusions for Moodle plugin mounts. MEGAsync identifies files
// by inode, and a bind mount shows the development folder's inodes at a
// second path. A mount destination inside a MEGA-synced folder is therefore
// excluded in that sync's .megaignore before it is mounted, and the
// exclusion is removed with the mount.

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/oito2/perci/internal/fsutil"
	"github.com/oito2/perci/internal/ui"
)

// megaIgnoreName is the file MEGAsync reads its exclusion rules from.
const megaIgnoreName = ".megaignore"

// megaIgnoreFor returns the .megaignore file nearest to dest among the
// folders above it, or "" when none of them has one.
func megaIgnoreFor(dest string) string {
	dir := filepath.Dir(filepath.Clean(dest))
	for {
		p := filepath.Join(dir, megaIgnoreName)
		if info, err := os.Stat(p); err == nil && info.Mode().IsRegular() {
			return p
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// megaIgnoreRule is the .megaignore line excluding the folder dest: a
// folder ("d") path rule ("p") relative to the folder of file, matched as a
// glob ("G"). Paths with glob characters are refused.
func megaIgnoreRule(file, dest string) (string, error) {
	rel, err := filepath.Rel(filepath.Dir(file), dest)
	if err != nil || !filepath.IsLocal(rel) {
		return "", fmt.Errorf("%s não fica dentro da pasta de %s", dest, file)
	}
	if strings.ContainsAny(rel, `*?[]\`) {
		return "", fmt.Errorf("o caminho %s tem caracteres especiais e não pode ser excluído da sincronização do MEGA", rel)
	}
	return "-dpG:" + filepath.ToSlash(rel), nil
}

// addMegaIgnoreRule appends rule to file, keeping its mode, unless a line
// equal to it is already there. added reports whether file was changed.
func addMegaIgnoreRule(file, rule string) (added bool, err error) {
	info, err := os.Stat(file)
	if err != nil {
		return false, err
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return false, err
	}
	text := string(data)
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) == rule {
			return false, nil
		}
	}
	if text != "" && !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	text += rule + "\n"
	if err := fsutil.WriteThroughLink(file, []byte(text), info.Mode().Perm()); err != nil {
		return false, err
	}
	return true, nil
}

// removeMegaIgnoreRule removes the lines equal to rule from file. A file
// that no longer exists, or no longer has the rule, is left as it is; a
// file not named .megaignore is refused.
func removeMegaIgnoreRule(file, rule string) error {
	if filepath.Base(file) != megaIgnoreName {
		return fmt.Errorf("%s não é um arquivo %s", file, megaIgnoreName)
	}
	info, err := os.Stat(file)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	lines := strings.Split(string(data), "\n")
	kept := lines[:0]
	for _, line := range lines {
		if strings.TrimSpace(line) != rule {
			kept = append(kept, line)
		}
	}
	if len(kept) == len(lines) {
		return nil
	}
	return fsutil.WriteThroughLink(file, []byte(strings.Join(kept, "\n")), info.Mode().Perm())
}

// excludeFromMega adds the exclusion of dest to the nearest .megaignore
// above it and returns the file and the rule it added. Both are empty when
// no folder above dest has a .megaignore or the rule was already there.
func excludeFromMega(stdout io.Writer, dest string) (file, rule string, err error) {
	file = megaIgnoreFor(dest)
	if file == "" {
		return "", "", nil
	}
	rule, err = megaIgnoreRule(file, dest)
	if err != nil {
		return "", "", fmt.Errorf("o destino fica numa pasta sincronizada pelo MEGA (%s) e não pode ser excluído dela: %w", file, err)
	}
	added, err := addMegaIgnoreRule(file, rule)
	if err != nil {
		return "", "", fmt.Errorf("o destino fica numa pasta sincronizada pelo MEGA e a exclusão não pôde ser gravada em %s: %w", file, err)
	}
	if !added {
		return "", "", nil
	}
	ui.Info(stdout, "Destino excluído da sincronização do MEGA: "+rule+" em "+file)
	return file, rule, nil
}
