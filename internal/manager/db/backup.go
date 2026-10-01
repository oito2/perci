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

// Backup dumps all databases from the MariaDB container to the dest file path.
func Backup(ctx context.Context, exe *executor.Executor, stdout io.Writer, container, user, pass, dest string) error {
	return withDBSession(ctx, exe, container, pass, func(envPath string) error {
		ui.Info(stdout, "Executando dump para: "+dest)

		f, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
		if err != nil {
			return fmt.Errorf("falha ao criar arquivo de backup: %w", err)
		}

		if err := exe.Run(ctx,
			executor.Options{Stdout: f, Stderr: stdout},
			"docker", "exec", "--env-file", envPath, container,
			// A consistent InnoDB snapshot without locking the tables,
			// including stored routines and events (skipped by default).
			"mariadb-dump", "-u", user, "--all-databases",
			"--single-transaction", "--skip-lock-tables", "--routines", "--events",
		); err != nil {
			_ = f.Close()
			_ = os.Remove(dest)
			return fmt.Errorf("falha no dump: %w", err)
		}
		if err := f.Close(); err != nil {
			_ = os.Remove(dest)
			return fmt.Errorf("falha ao fechar arquivo de backup: %w", err)
		}

		ui.Success(stdout, "Backup concluído: "+dest)
		return nil
	})
}
