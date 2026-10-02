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

package megasync

import (
	"context"
	"fmt"
	"io"

	"github.com/oito2/perci/internal/distro"
	"github.com/oito2/perci/internal/executor"
	"github.com/oito2/perci/internal/ui"
)

// pkgName is the package name megasync is installed under with the native
// package manager.
const pkgName = "megasync"

// rpmKeyRemoveScript deletes the key installFromDNF imported with `rpm
// --import`, found by its name rather than a fingerprint. It lists and
// deletes keys with rpmkeys; finding none is not an error.
const rpmKeyRemoveScript = `rpmkeys --list | awk '/MegaLimited/ {print $1}' | while read -r fp; do rpmkeys --delete "$fp"; done`

// Uninstall removes MegaSync via the distro's native package manager, then
// MEGA's repository and key, all in the same privileged batch (one password
// prompt).
func Uninstall(ctx context.Context, exe *executor.Executor, stdout io.Writer) error {
	return uninstall(ctx, exe, stdout, distro.Detect())
}

// uninstall takes family as a parameter rather than calling
// distro.Detect() itself.
func uninstall(ctx context.Context, exe *executor.Executor, stdout io.Writer, family string) error {
	var steps []executor.PrivilegedStep
	switch family {
	case distro.Debian:
		steps = []executor.PrivilegedStep{
			{Name: "apt-get", Args: []string{"purge", "-y", "--", pkgName}},
			{Announce: "Removendo o repositório da MEGA...", Name: "rm", Args: []string{"-f", "--", aptSourceList, aptKeyring}},
		}
	case distro.Fedora:
		steps = []executor.PrivilegedStep{
			{Name: "dnf", Args: []string{"remove", "-y", "--", pkgName}},
			{Announce: "Removendo o repositório da MEGA...", Name: "rm", Args: []string{"-f", "--", dnfRepoFile}},
			{
				Announce: "Removendo a chave de assinatura da MEGA...",
				Name:     "sh", Args: []string{"-c", rpmKeyRemoveScript},
				Soft: true, WarnMessage: "Não foi possível remover a chave da MEGA (rpmkeys --list mostra as chaves instaladas).",
			},
		}
	default:
		return fmt.Errorf("desinstalação automática do MegaSync não suportada nesta distribuição")
	}
	if err := exe.RunSudoSequence(ctx, executor.Options{Stdout: stdout, Stderr: stdout}, steps); err != nil {
		return err
	}
	ui.Success(stdout, "MegaSync removido com sucesso.")
	return nil
}
