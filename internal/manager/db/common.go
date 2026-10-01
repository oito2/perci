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
	"os"
	"strings"

	"github.com/oito2/perci/internal/appstack"
	"github.com/oito2/perci/internal/executor"
)

// RequireContainer verifies that the named container is running in
// Docker (appstack.ContainerStatus — an exact name, not docker ps's
// substring filter).
func RequireContainer(ctx context.Context, exe *executor.Executor, name string) error {
	if appstack.ContainerStatus(ctx, exe, name) != "running" {
		return fmt.Errorf("o contêiner '%s' não está em execução — inicie-o em Docker → Gerenciar Containers (o Docker está rodando?)", name)
	}
	return nil
}

// writeTempSecret writes content to a new temporary file with 0600 permissions.
func writeTempSecret(content, pattern string) (string, func(), error) {
	f, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", nil, err
	}
	if _, err := fmt.Fprint(f, content); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return "", nil, fmt.Errorf("credencial: %w", err)
	}
	_ = f.Close()
	name := f.Name()
	return name, func() { _ = os.Remove(name) }, nil
}

// withDBSession verifies container is running, writes a temporary MYSQL_PWD
// credential file for it, and calls fn with that file's path, removing it
// afterward regardless of outcome. Shared by Backup and Restore — the only
// operations that need a live mysql/mariadb client session.
func withDBSession(ctx context.Context, exe *executor.Executor, container, pass string, fn func(envPath string) error) error {
	if strings.ContainsAny(pass, "\r\n") {
		return fmt.Errorf("senha do banco contém caracteres inválidos")
	}
	if err := RequireContainer(ctx, exe, container); err != nil {
		return err
	}
	envPath, cleanup, err := writeTempSecret("MYSQL_PWD="+pass+"\n", "perci-db-*.env")
	if err != nil {
		return fmt.Errorf("falha ao criar credencial temporária: %w", err)
	}
	defer cleanup()
	return fn(envPath)
}
