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

package selfupdate

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/oito2/perci/internal/executor"
	"github.com/oito2/perci/packaging"
)

// hicolorDir is the system-wide hicolor icon theme root holding the
// application menu icon (the desktop entry's Icon=perci resolves against
// it).
const hicolorDir = "/usr/share/icons/hicolor"

// legacyIconPath is the single non-square PNG, referenced by absolute
// path, that older installs used as the menu icon; Uninstall and the
// install paths still remove it.
const legacyIconPath = "/usr/share/pixmaps/perci.png"

// menuIconPaths returns every installed hicolor icon file, one per size.
func menuIconPaths() []string {
	paths := make([]string, len(packaging.IconSizes))
	for i, size := range packaging.IconSizes {
		paths[i] = filepath.Join(hicolorDir, packaging.IconRelPath(size))
	}
	return paths
}

// MenuIconsInstalled reports whether the application menu icon set is
// installed system-wide — false for a binary run straight from the build
// folder, where there's nothing to replace.
func MenuIconsInstalled() bool {
	_, err := os.Stat(menuIconPaths()[0])
	return err == nil
}

// InstallMenuIcons replaces the system-wide application menu icon set with
// the embedded one for color ("blue"/"pink") — every size in one
// privileged batch, so pkexec asks for the password once. The PNGs go to
// root as an in-memory tar archive on stdin (pkexec forwards stdin), never
// as files in a user-writable temp dir: those could be swapped for a
// symlink to a root-only file while the password dialog is open, and root
// would copy its content into /usr/share/icons. Refreshing the icon cache
// is best-effort: the files are already in place if it fails, the desktop
// just may take longer (or a new login) to show them.
func InstallMenuIcons(ctx context.Context, exe *executor.Executor, stdout io.Writer, color string) error {
	archive, err := menuIconsTar(color)
	if err != nil {
		return err
	}
	steps := []executor.PrivilegedStep{
		{
			Name: "tar",
			Args: []string{"-x", "-f", "-", "-C", hicolorDir, "--no-same-owner", "--no-same-permissions", "--no-overwrite-dir"},
		},
		iconCacheStep(),
	}
	return exe.RunSudoSequence(ctx, executor.Options{Stdin: archive, Stdout: stdout, Stderr: stdout}, steps)
}

// menuIconsTar builds the tar archive InstallMenuIcons streams to root:
// <size>x<size>/apps/perci.png entries (plus their directories), relative
// to hicolorDir, regular files only, mode 0644.
func menuIconsTar(color string) (*bytes.Buffer, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, size := range packaging.IconSizes {
		data, err := packaging.Icon(color, size)
		if err != nil {
			return nil, fmt.Errorf("ícone %s %dpx: %w", color, size, err)
		}
		rel := packaging.IconRelPath(size)
		for _, dir := range []string{filepath.Dir(filepath.Dir(rel)), filepath.Dir(rel)} {
			if err := tw.WriteHeader(&tar.Header{Name: dir + "/", Mode: 0o755, Typeflag: tar.TypeDir}); err != nil {
				return nil, err
			}
		}
		if err := tw.WriteHeader(&tar.Header{Name: rel, Mode: 0o644, Size: int64(len(data)), Typeflag: tar.TypeReg}); err != nil {
			return nil, err
		}
		if _, err := tw.Write(data); err != nil {
			return nil, err
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	return &buf, nil
}

// iconCacheStep refreshes the hicolor icon cache — Soft: a missing or
// failing gtk-update-icon-cache never fails the batch it's part of.
func iconCacheStep() executor.PrivilegedStep {
	return executor.PrivilegedStep{
		Soft:        true,
		WarnMessage: "Não foi possível atualizar o cache de ícones (gtk-update-icon-cache).",
		Name:        "gtk-update-icon-cache",
		Args:        []string{"-q", "-t", "-f", hicolorDir},
	}
}
