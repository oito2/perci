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

package apps

import (
	"context"
	"io"

	"github.com/oito2/perci/internal/executor"
	"github.com/oito2/perci/internal/ui"
)

// uninstallOne uninstalls a single Flatpak app from the given scope.
func uninstallOne(ctx context.Context, exe *executor.Executor, stdout io.Writer, id, scope string) error {
	ui.Info(stdout, "Desinstalando: "+id)
	return exe.Run(ctx,
		executor.Options{RequiresSudo: scope == "--system", Stdout: stdout, Stderr: stdout, Env: []string{"TERM=dumb"}},
		"flatpak", "uninstall", "--noninteractive", scope, "-y", id,
	)
}
