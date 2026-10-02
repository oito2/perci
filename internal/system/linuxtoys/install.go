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
	"bytes"
	"context"
	"fmt"
	"io"
	"os"

	"github.com/oito2/perci/internal/executor"
	"github.com/oito2/perci/internal/ui"
)

// Installed reports whether the linuxtoys binary is on PATH.
func Installed(ctx context.Context, exe *executor.Executor) bool {
	return exe.CommandAvailable(ctx, "linuxtoys")
}

// Install runs the official linux.toys installer script as root (one
// pkexec prompt): the script escalates with its own `sudo apt/dnf/
// rpm-ostree ...`, and as root those inner sudo calls need no password.
//
// The script reaches root on stdin (`bash -s`), never as a file path,
// because the download sits in a user-writable temp file that any process
// of the same user could swap while the password dialog is open. A non-tty
// stdin also makes the script take its non-interactive branch. Re-running
// the same script updates an existing installation.
//
// The download is NOT verified against a pinned checksum: the script
// always fetches whatever package is current for the distro. It is
// downloaded over HTTPS from the project's own domain.
func Install(ctx context.Context, exe *executor.Executor, stdout io.Writer) error {
	ui.Info(stdout, "Baixando instalador do Linux Toys...")
	tmpFile, err := os.CreateTemp("", "linuxtoys-install-*.sh")
	if err != nil {
		return fmt.Errorf("criar arquivo temporário: %w", err)
	}
	tmpPath := tmpFile.Name()
	_ = tmpFile.Close()
	defer func() { _ = os.Remove(tmpPath) }()

	if err := exe.Run(ctx, executor.Options{Stdout: stdout, Stderr: stdout},
		"curl", "-fsSL", "--proto", "=https", "--connect-timeout", "10", "--max-time", "600", "-o", tmpPath, "https://linux.toys/install.sh",
	); err != nil {
		ui.Err(stdout, "Falha ao baixar o instalador do Linux Toys: "+err.Error())
		return err
	}
	script, err := os.ReadFile(tmpPath)
	if err != nil || len(script) == 0 {
		ui.Err(stdout, "Download do instalador do Linux Toys retornou um arquivo vazio.")
		return fmt.Errorf("instalador do linux toys baixado está vazio")
	}

	ui.Info(stdout, "Executando instalador do Linux Toys como administrador...")
	if err := exe.Run(ctx,
		executor.Options{RequiresSudo: true, Stdin: bytes.NewReader(script), Stdout: stdout, Stderr: stdout},
		"bash", "-s",
	); err != nil {
		ui.Err(stdout, "Falha ao instalar Linux Toys: "+err.Error())
		return err
	}
	ui.Success(stdout, "Linux Toys instalado com sucesso.")
	return nil
}
