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

package ide

import (
	"context"
	"fmt"
	"io"

	"github.com/oito2/perci/internal/distro"
	"github.com/oito2/perci/internal/executor"
)

// UninstallOne uninstalls a single IDE.
func UninstallOne(ctx context.Context, exe *executor.Executor, stdout io.Writer, e IDE, family string) error {
	switch e.Cmd {
	case "zed":
		script := `rm -rf "$HOME/.local/zed.app" "$HOME/.local/bin/zed" 2>/dev/null; true`
		return exe.Run(ctx, executor.Options{Stdout: stdout, Stderr: stdout}, "bash", "-c", script)
	case "code":
		opts := executor.Options{RequiresSudo: true, Stdout: stdout, Stderr: stdout}
		return aptDnfRemove(ctx, exe, "code", family, opts)
	case "codium":
		opts := executor.Options{RequiresSudo: true, Stdout: stdout, Stderr: stdout}
		return aptDnfRemove(ctx, exe, "codium", family, opts)
	}
	return fmt.Errorf("desinstalador desconhecido para %s", e.Name)
}

func aptDnfRemove(ctx context.Context, exe *executor.Executor, pkg, family string, opts executor.Options) error {
	switch family {
	case distro.Fedora:
		return exe.Run(ctx, opts, "dnf", "remove", "-y", "--", pkg)
	case distro.Debian:
		return exe.Run(ctx, opts, "apt-get", "purge", "-y", "--", pkg)
	default:
		return fmt.Errorf("unsupported distro family for package removal: %s", family)
	}
}
