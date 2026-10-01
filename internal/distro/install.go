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

package distro

import (
	"context"
	"io"

	"github.com/oito2/perci/internal/executor"
)

// InstallPkgs installs pkgs using the distro-appropriate package manager.
// For Debian families it runs apt-get update before installing — both in
// one privileged batch, so the password is asked once.
func InstallPkgs(ctx context.Context, exe *executor.Executor, stdout io.Writer, family string, pkgs ...string) error {
	opts := executor.Options{RequiresSudo: true, Stdout: stdout, Stderr: stdout}
	switch family {
	case Debian:
		return exe.RunSudoSequence(ctx, executor.Options{Stdout: stdout, Stderr: stdout}, []executor.PrivilegedStep{
			{Name: "apt-get", Args: []string{"update", "-q"}},
			{Name: "apt-get", Args: append([]string{"install", "-y", "--"}, pkgs...)},
		})
	case Fedora:
		args := append([]string{"install", "-y", "--"}, pkgs...)
		return exe.Run(ctx, opts, "dnf", args...)
	default:
		return UnsupportedFamilyError(family)
	}
}
