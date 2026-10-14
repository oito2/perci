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

package prereqs

// Flatpak Builder Tools: the offline source generators of
// flatpak/flatpak-builder-tools, cloned at a pinned commit into
// ~/tools/flatpak-builder-tools. flatpak-node-generator (npm
// dependencies) is installed with pipx into ~/.local/bin;
// flatpak-cargo-generator.py (Rust dependencies) runs from the clone with
// the system Python, which needs the aiohttp and tomlkit packages.

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/oito2/perci/internal/dev/localbin"
	"github.com/oito2/perci/internal/dev/nettimeout"
	"github.com/oito2/perci/internal/distro"
	"github.com/oito2/perci/internal/executor"
	"github.com/oito2/perci/internal/ui"
)

const (
	// flatpakToolsRepo is the repository cloned, and flatpakToolsCommit
	// the commit checked out.
	flatpakToolsRepo   = "https://github.com/flatpak/flatpak-builder-tools.git"
	flatpakToolsCommit = "74697c75b630d7330e77250fc13cb5ea688d9479"
	// flatpakNodePackage is the pipx package of flatpak-node-generator.
	flatpakNodePackage = "flatpak_node_generator"
)

// flatpakToolsPkgs are the distro packages the generators need: pipx for
// flatpak-node-generator, aiohttp and tomlkit for flatpak-cargo-generator.
var flatpakToolsPkgs = []string{"pipx", "python3-aiohttp", "python3-tomlkit"}

// flatpakToolsDir returns where the repository is cloned.
func flatpakToolsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("obter diretório home: %w", err)
	}
	return filepath.Join(home, "tools", "flatpak-builder-tools"), nil
}

// flatpakToolsInstalled reports whether the clone has the cargo generator
// and flatpak-node-generator is in ~/.local/bin.
func flatpakToolsInstalled() bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	dir, err := flatpakToolsDir()
	if err != nil {
		return false
	}
	if _, err := os.Stat(filepath.Join(dir, "cargo", "flatpak-cargo-generator.py")); err != nil {
		return false
	}
	_, err = os.Stat(filepath.Join(home, ".local", "bin", "flatpak-node-generator"))
	return err == nil
}

// isFlatpakToolsClone reports whether dir is a git clone of
// flatpakToolsRepo.
func isFlatpakToolsClone(ctx context.Context, exe *executor.Executor, dir string) bool {
	out, err := exe.Output(ctx, executor.Options{}, "git", "-C", dir, "remote", "get-url", "origin")
	return err == nil && strings.TrimSpace(string(out)) == flatpakToolsRepo
}

// installFlatpakTools installs the distro packages, clones (or updates)
// the repository at flatpakToolsCommit and installs flatpak-node-generator
// with pipx. Running it again moves an existing install to the pinned
// commit. A folder at the clone path that isn't a clone of the repository
// is refused.
func installFlatpakTools(ctx context.Context, exe *executor.Executor, stdout io.Writer, family string) error {
	dir, err := flatpakToolsDir()
	if err != nil {
		return err
	}
	if err := distro.InstallPkgs(ctx, exe, stdout, family, flatpakToolsPkgs...); err != nil {
		return err
	}

	netCtx, cancel := nettimeout.Context(ctx)
	defer cancel()
	opts := executor.Options{Stdout: stdout, Stderr: stdout}
	if _, err := os.Stat(dir); err == nil {
		if !exe.DryRun && !isFlatpakToolsClone(netCtx, exe, dir) {
			return fmt.Errorf("%s já existe e não é um clone de %s — mova ou apague essa pasta", dir, flatpakToolsRepo)
		}
		ui.Info(stdout, "Atualizando "+dir+"...")
		if err := exe.Run(netCtx, opts, "git", "-C", dir, "fetch", "--quiet", "origin"); err != nil {
			return fmt.Errorf("atualizar flatpak-builder-tools: %w", err)
		}
	} else {
		if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
			return err
		}
		ui.Info(stdout, "Clonando flatpak-builder-tools em "+dir+"...")
		if err := exe.Run(netCtx, opts, "git", "clone", "--quiet", "--", flatpakToolsRepo, dir); err != nil {
			return fmt.Errorf("clonar flatpak-builder-tools: %w", err)
		}
	}
	if err := exe.Run(netCtx, opts, "git", "-C", dir, "checkout", "--quiet", "--detach", flatpakToolsCommit); err != nil {
		return fmt.Errorf("fixar flatpak-builder-tools em %s: %w", flatpakToolsCommit[:12], err)
	}

	ui.Info(stdout, "Instalando flatpak-node-generator (pipx)...")
	if err := exe.Run(netCtx, opts, "pipx", "install", "--force", filepath.Join(dir, "node")); err != nil {
		return fmt.Errorf("instalar flatpak-node-generator: %w", err)
	}
	localbin.EnsureInPath(stdout)
	ui.Success(stdout, "Geradores prontos: flatpak-node-generator e python3 "+filepath.Join(dir, "cargo", "flatpak-cargo-generator.py"))
	return nil
}

// uninstallFlatpakTools removes flatpak-node-generator from pipx and the
// clone (only when it is a clone of the repository). The distro packages
// stay.
func uninstallFlatpakTools(ctx context.Context, exe *executor.Executor, stdout io.Writer) error {
	dir, err := flatpakToolsDir()
	if err != nil {
		return err
	}
	opts := executor.Options{Stdout: stdout, Stderr: stdout}
	if exe.CommandAvailable(ctx, "pipx") {
		if err := exe.Run(ctx, opts, "pipx", "uninstall", flatpakNodePackage); err != nil {
			ui.Warning(stdout, "flatpak-node-generator não foi removido do pipx: "+err.Error())
		}
	}
	if _, err := os.Stat(dir); err != nil {
		return nil
	}
	if !exe.DryRun && !isFlatpakToolsClone(ctx, exe, dir) {
		return fmt.Errorf("%s não é um clone de %s; nada foi apagado", dir, flatpakToolsRepo)
	}
	ui.Info(stdout, "Removendo "+dir+"...")
	if exe.DryRun {
		return nil
	}
	return os.RemoveAll(dir)
}
