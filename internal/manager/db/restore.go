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

package db

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/oito2/perci/internal/executor"
	"github.com/oito2/perci/internal/ui"
)

// Restore imports a SQL backup file into the MariaDB container.
func Restore(ctx context.Context, exe *executor.Executor, stdout io.Writer, container, user, pass, srcFile string) error {
	return withDBSession(ctx, exe, container, pass, func(envPath string) error {
		ui.Info(stdout, "Restaurando... Isso pode levar alguns minutos.")

		f, err := os.Open(srcFile)
		if err != nil {
			return fmt.Errorf("falha ao abrir arquivo de backup: %w", err)
		}
		defer func() { _ = f.Close() }()

		if err := exe.Run(ctx,
			executor.Options{Stdin: f, Stdout: stdout, Stderr: stdout},
			"docker", "exec", "-i", "--env-file", envPath, container,
			"mariadb", "-u", user,
		); err != nil {
			return fmt.Errorf("falha no restore: %w", err)
		}

		ui.Success(stdout, "Restore concluído com sucesso.")
		return nil
	})
}
