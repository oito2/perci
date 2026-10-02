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
	"github.com/oito2/perci/internal/distro"
	"github.com/oito2/perci/internal/executor"
)

// InstallOne installs a single terminal or shell tool.
func InstallOne(ctx context.Context, exe *executor.Executor, stdout io.Writer, t Terminal, family string) error {
	opts := executor.Options{Stdout: stdout, Stderr: stdout}

	switch t.Cmd {
	case "kitty":
		// pipefail makes a failed download fail the whole command.
		script := `set -Eeuo pipefail; curl -fsSL https://sw.kovidgoyal.net/kitty/installer.sh | sh /dev/stdin`
		if err := exe.Run(ctx, opts, "bash", "-c", script); err != nil {
			return err
		}
		linkScript := `
mkdir -p "$HOME/.local/bin"
ln -sf "$HOME/.local/kitty.app/bin/kitty"  "$HOME/.local/bin/kitty"
ln -sf "$HOME/.local/kitty.app/bin/kitten" "$HOME/.local/bin/kitten"
`
		return exe.Run(ctx, opts, "bash", "-c", linkScript)

	case "alacritty":
		// One privileged batch, so the password is asked once.
		return distro.InstallPkgs(ctx, exe, stdout, family, "alacritty")

	case "blackbox-terminal":
		flatOpts := executor.Options{Stdout: stdout, Stderr: stdout, Env: []string{"TERM=dumb"}}
		return exe.Run(ctx, flatOpts, "flatpak", "install", "--noninteractive", config.FlatpakFlag(), "-y", "flathub", t.FlatID)
	case "kgx":
		return distro.InstallPkgs(ctx, exe, stdout, family, "gnome-console")
	}
	return fmt.Errorf("instalador desconhecido para %s", t.Name)
}
