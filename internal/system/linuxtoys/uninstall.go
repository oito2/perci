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

package linuxtoys

import (
	"context"
	"fmt"
	"io"

	"github.com/oito2/perci/internal/distro"
	"github.com/oito2/perci/internal/executor"
	"github.com/oito2/perci/internal/ui"
)

// pkgName is the package name the official installer registers Linux Toys
// under with the native package manager.
const pkgName = "linuxtoys"

// Uninstall removes Linux Toys via the distro's native package manager,
// the same route the official installer (.deb/.rpm) used to install it.
func Uninstall(ctx context.Context, exe *executor.Executor, stdout io.Writer) error {
	return uninstall(ctx, exe, stdout, distro.Detect())
}

// uninstall takes family as a parameter rather than calling
// distro.Detect() itself.
func uninstall(ctx context.Context, exe *executor.Executor, stdout io.Writer, family string) error {
	opts := executor.Options{RequiresSudo: true, Stdout: stdout, Stderr: stdout}
	switch family {
	case distro.Debian:
		if err := exe.Run(ctx, opts, "apt-get", "purge", "-y", "--", pkgName); err != nil {
			return err
		}
	case distro.Fedora:
		if err := exe.Run(ctx, opts, "dnf", "remove", "-y", "--", pkgName); err != nil {
			return err
		}
	default:
		return fmt.Errorf("desinstalação automática do Linux Toys não suportada nesta distribuição")
	}
	ui.Success(stdout, "Linux Toys removido com sucesso.")
	return nil
}
