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
	if err := os.Remove(currentExe); err != nil {
		ui.Info(stdout, "Não foi possível remover o binário diretamente ("+err.Error()+"). Tentando com sudo...")
		if sudoErr := exe.Run(ctx,
			executor.Options{RequiresSudo: true, Stdout: stdout, Stderr: stdout},
			"rm", "-f", "--", currentExe,
		); sudoErr != nil {
			return fmt.Errorf("remover binário: %w", sudoErr)
		}
	}

	// 'perci' alias (a symlink to the 'prci' binary above, created by
	// install.sh/`make install`) — without this, a broken symlink pointing
	// at the just-removed binary would be left behind. os.Executable()
	// always resolves to the symlink's real target (confirmed — it never
	// returns the symlink itself), so currentExe here is always
	// ".../prci", and the alias lives right next to it under a fixed
	// name. Best-effort, same as the block below: doesn't abort the
	// uninstall if it fails.
	if aliasPath := filepath.Join(filepath.Dir(currentExe), "perci"); aliasPath != currentExe {
		if err := os.Remove(aliasPath); err != nil && !os.IsNotExist(err) {
			if sudoErr := exe.Run(ctx,
				executor.Options{RequiresSudo: true, Stdout: stdout, Stderr: stdout},
				"rm", "-f", "--", aliasPath,
			); sudoErr != nil {
				ui.Warning(stdout, "Falha ao remover o alias 'perci': "+sudoErr.Error())
			}
		}
	}

	// Application menu entry + icon (install.sh/`make install`) — removed
	// alongside the binary, best-effort: without this, a .desktop entry
	// pointing at the binary that just disappeared would be left behind
	// as a "ghost" in the menu. Doesn't abort the uninstall if it fails
	// (same spirit as the removeConfig block below — removing the binary
	// itself already succeeded). Files that need root are removed by a
	// single privileged rm, so pkexec asks once instead of once per icon.
	menuPaths := uninstallMenuPaths()
	var privileged []string
	for _, path := range menuPaths {
		if rmErr := os.Remove(path); rmErr != nil && !os.IsNotExist(rmErr) {
			privileged = append(privileged, path)
		}
	}
	if len(privileged) > 0 {
		steps := []executor.PrivilegedStep{
			{Name: "rm", Args: append([]string{"-f", "--"}, privileged...)},
			iconCacheStep(), // drop the removed icons from the cache in the same prompt
		}
		if sudoErr := exe.RunSudoSequence(ctx, executor.Options{Stdout: stdout, Stderr: stdout}, steps); sudoErr != nil {
			ui.Warning(stdout, "Falha ao remover a entrada do menu de aplicativos: "+sudoErr.Error())
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
