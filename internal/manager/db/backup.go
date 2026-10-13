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
	"path/filepath"

	"github.com/oito2/perci/internal/executor"
	"github.com/oito2/perci/internal/ui"
)

// Backup dumps all databases from the MariaDB container to the dest file path.
// The dump goes to a temporary file next to dest and replaces it only when
// it completes, so a failed dump leaves an existing dest untouched.
func Backup(ctx context.Context, exe *executor.Executor, stdout io.Writer, container, user, pass, dest string) error {
	return withDBSession(ctx, exe, container, pass, func(envPath string) error {
		ui.Info(stdout, "Executando dump para: "+dest)

		tmp, err := os.CreateTemp(filepath.Dir(dest), ".perci-backup-*.sql")
		if err != nil {
			return fmt.Errorf("falha ao criar arquivo de backup: %w", err)
		}
		defer func() { _ = os.Remove(tmp.Name()) }() // no-op once renamed
		if err := tmp.Chmod(0o600); err != nil {
			_ = tmp.Close()
			return fmt.Errorf("falha ao proteger o arquivo de backup: %w", err)
		}

		if err := exe.Run(ctx,
			executor.Options{Stdout: tmp, Stderr: stdout},
			"docker", "exec", "--env-file", envPath, container,
			// A consistent InnoDB snapshot without locking the tables,
			// including stored routines and events (skipped by default).
			"mariadb-dump", "-u", user, "--all-databases",
			"--single-transaction", "--skip-lock-tables", "--routines", "--events",
		); err != nil {
			_ = tmp.Close()
			return fmt.Errorf("falha no dump: %w", err)
		}
		if err := tmp.Sync(); err != nil {
			_ = tmp.Close()
			return fmt.Errorf("falha ao gravar o arquivo de backup: %w", err)
		}
		if err := tmp.Close(); err != nil {
			return fmt.Errorf("falha ao fechar arquivo de backup: %w", err)
		}
		if err := os.Rename(tmp.Name(), dest); err != nil {
			return fmt.Errorf("falha ao salvar o backup em %s: %w", dest, err)
		}

		ui.Success(stdout, "Backup concluído: "+dest)
		return nil
	})
}
