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

package terminal

import (
	"context"
	"fmt"
	"io"

	"github.com/oito2/perci/internal/config"
	"github.com/oito2/perci/internal/dev/prereqs"
	"github.com/oito2/perci/internal/distro"
	"github.com/oito2/perci/internal/executor"
)

// UninstallOne uninstalls a single terminal or shell tool.
func UninstallOne(ctx context.Context, exe *executor.Executor, stdout io.Writer, t Terminal, family string) error {
	opts := executor.Options{Stdout: stdout, Stderr: stdout}
	sudo := executor.Options{RequiresSudo: true, Stdout: stdout, Stderr: stdout}

	switch t.Cmd {
	case "kitty":
		script := `rm -rf "$HOME/.local/kitty.app" "$HOME/.local/bin/kitty" "$HOME/.local/bin/kitten" 2>/dev/null; true`
		return exe.Run(ctx, opts, "bash", "-c", script)
	case "alacritty":
		return removePackage(ctx, exe, sudo, family, "alacritty")
	case "blackbox-terminal":
		flatOpts := executor.Options{Stdout: stdout, Stderr: stdout, Env: []string{"TERM=dumb"}}
		return exe.Run(ctx, flatOpts, "flatpak", "uninstall", "--noninteractive", config.FlatpakFlag(), "-y", t.FlatID)
	case "kgx":
		return removePackage(ctx, exe, sudo, family, "gnome-console")
	}
	return fmt.Errorf("desinstalador desconhecido para %s", t.Name)
}

// removePackage removes pkg only when the removal takes no other package
// with it.
func removePackage(ctx context.Context, exe *executor.Executor, sudo executor.Options, family, pkg string) error {
	if family != distro.Fedora && family != distro.Debian {
		return fmt.Errorf("unsupported distro family for %s: %s", pkg, family)
	}
	return prereqs.RemovePackages(ctx, exe, sudo, family, pkg)
}
