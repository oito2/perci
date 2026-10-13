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
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/oito2/perci/internal/executor"
	"github.com/oito2/perci/internal/ui"
)

// ErrUninstalled is returned by Uninstall after the binary is successfully removed.
var ErrUninstalled = errors.New("perci desinstalado")

// uninstallMenuPaths lists the application menu entry and icons Uninstall
// removes — a variable so tests never touch the real system paths.
var uninstallMenuPaths = func() []string {
	return append([]string{"/usr/share/applications/perci.desktop", legacyIconPath}, menuIconPaths()...)
}

// Uninstall removes the perci binary and optionally its config directory.
func Uninstall(ctx context.Context, exe *executor.Executor, stdout io.Writer, removeConfig bool) error {
	currentExe, err := executablePath()
	if err != nil {
		return fmt.Errorf("localizar binário: %w", err)
	}

	ui.Info(stdout, "Removendo binário: "+currentExe)

	// Everything that cannot be removed without privilege is collected into
	// ONE privileged batch, so the password is asked at most once: the binary
	// (a failure there aborts the uninstall), then the 'perci' alias and the
	// menu entry/icons (Soft: a failure only warns).
	var steps []executor.PrivilegedStep
	binaryNeedsRoot := false
	if err := os.Remove(currentExe); err != nil {
		ui.Info(stdout, "Não foi possível remover o binário diretamente ("+err.Error()+"). Será usado privilégio de administrador.")
		binaryNeedsRoot = true
		steps = append(steps, executor.PrivilegedStep{Name: "rm", Args: []string{"-f", "--", currentExe}})
	}

	// 'perci' alias: removed only when it is a symlink to the binary above,
	// so no broken symlink is left behind and an unrelated file or directory
	// named 'perci' is never touched. os.Executable() resolves to the
	// symlink's real target, so the alias sits right next to currentExe.
	if aliasPath := filepath.Join(filepath.Dir(currentExe), "perci"); aliasPath != currentExe && isOurAlias(aliasPath, currentExe) {
		if err := os.Remove(aliasPath); err != nil && !os.IsNotExist(err) {
			steps = append(steps, executor.PrivilegedStep{
				Soft:        true,
				WarnMessage: "Falha ao remover o alias 'perci'.",
				Name:        "rm",
				Args:        []string{"-f", "--", aliasPath},
			})
		}
	}

	// Application menu entry + icons, so no .desktop entry pointing at the
	// removed binary is left behind as a "ghost" in the menu. While the
	// binary still needs the privileged batch, nothing is removed directly:
	// a canceled password prompt then leaves the binary and its menu entry
	// together.
	var privileged []string
	for _, path := range uninstallMenuPaths() {
		if binaryNeedsRoot {
			if _, statErr := os.Lstat(path); statErr == nil {
				privileged = append(privileged, path)
			}
			continue
		}
		if rmErr := os.Remove(path); rmErr != nil && !os.IsNotExist(rmErr) {
			privileged = append(privileged, path)
		}
	}
	if len(privileged) > 0 {
		steps = append(steps,
			executor.PrivilegedStep{
				Soft:        true,
				WarnMessage: "Falha ao remover a entrada do menu de aplicativos.",
				Name:        "rm",
				Args:        append([]string{"-f", "--"}, privileged...),
			},
			iconCacheStep(), // drop the removed icons from the cache in the same prompt
		)
	}

	if len(steps) > 0 {
		if sudoErr := exe.RunSudoSequence(ctx, executor.Options{Stdout: stdout, Stderr: stdout}, steps); sudoErr != nil {
			if binaryNeedsRoot {
				return fmt.Errorf("remover binário: %w", sudoErr)
			}
			ui.Warning(stdout, "Falha ao remover itens que exigem privilégio: "+sudoErr.Error())
		}
	}

	if removeConfig {
		if home, homeErr := os.UserHomeDir(); homeErr == nil {
			cfgDir := filepath.Join(home, ".perci")
			if rmErr := os.RemoveAll(cfgDir); rmErr != nil {
				ui.Warning(stdout, "Falha ao remover configurações: "+rmErr.Error())
			} else {
				ui.Info(stdout, "Configurações removidas (~/.perci).")
			}
		}
	}

	ui.Success(stdout, "PERCI // O GENIAL desinstalado com sucesso.")
	return ErrUninstalled
}

// isOurAlias reports whether aliasPath is a symlink pointing at target.
func isOurAlias(aliasPath, target string) bool {
	fi, err := os.Lstat(aliasPath)
	if err != nil || fi.Mode()&os.ModeSymlink == 0 {
		return false
	}
	dest, err := os.Readlink(aliasPath)
	if err != nil {
		return false
	}
	if !filepath.IsAbs(dest) {
		dest = filepath.Join(filepath.Dir(aliasPath), dest)
	}
	return resolvedDirPath(dest) == resolvedDirPath(target)
}

// resolvedDirPath is p with its directory's symlinks resolved; the last
// element is kept as is, since the file itself may already be removed.
func resolvedDirPath(p string) string {
	dir := filepath.Dir(filepath.Clean(p))
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
	return filepath.Join(dir, filepath.Base(p))
}
