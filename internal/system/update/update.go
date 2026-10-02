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

package update

import (
	"context"
	"fmt"
	"io"

	"github.com/oito2/perci/internal/config"
	"github.com/oito2/perci/internal/distro"
	"github.com/oito2/perci/internal/executor"
	"github.com/oito2/perci/internal/ui"
)

// Options are the optional cleanups the "Atualizar Sistema" screen offers,
// both off by default: a plain update never deletes logs or removes
// packages on its own.
type Options struct {
	// CleanJournal vacuums journal entries older than 7 days.
	CleanJournal bool
	// Autoremove removes orphaned packages (apt/dnf autoremove) and unused
	// Flatpak runtimes.
	Autoremove bool
}

// Run updates the system: packages, snap and flatpak (when available),
// plus the cleanups opts enables.
//
// Progress reports through ui.Step, one call per top-level phase:
// package/snap/journal update (all privileged) count as a single phase,
// and flatpak (never privileged) is its own. A phase is counted only when
// present.
//
// Every privileged command (package manager + snap + journalctl) runs as
// ONE authentication via executor.RunSudoSequence. Each PrivilegedStep's
// Announce field is echoed from inside the single privileged shell script,
// as plain text.
func Run(ctx context.Context, exe *executor.Executor, stdout io.Writer, opts Options) error {
	ui.PrintHeader(stdout, "Atualizar Sistema", distro.DisplayName())

	family := distro.Detect()
	if family == distro.Unknown {
		ui.Err(stdout, "Distribuição não suportada. Gerenciador de pacotes não reconhecido.")
		return fmt.Errorf("nenhum gerenciador de pacotes suportado encontrado (apt/dnf)")
	}
	ui.Info(stdout, "Distribuição detectada: "+family)

	steps, err := packageSteps(family, opts.Autoremove)
	if err != nil {
		ui.Err(stdout, err.Error())
		return err
	}

	hasSnap := exe.CommandAvailable(ctx, "snap")
	hasFlatpak := exe.CommandAvailable(ctx, "flatpak")
	hasJournal := exe.CommandAvailable(ctx, "journalctl")

	if hasSnap {
		steps = append(steps, executor.PrivilegedStep{
			Announce: "Atualizando Snaps...",
			Soft:     true, WarnMessage: "Falha ao atualizar snaps.",
			Name: "snap", Args: []string{"refresh"},
		})
	} else {
		ui.Info(stdout, "Snap não instalado. Etapa ignorada.")
	}
	if opts.CleanJournal && hasJournal {
		steps = append(steps, executor.PrivilegedStep{
			Announce: "Limpando logs antigos do journal...",
			Soft:     true, WarnMessage: "Falha ao limpar journal.",
			Name: "journalctl", Args: []string{"--vacuum-time=7d"},
		})
	}

	total := 1 // the privileged batch above (packages + snap + journal, whichever apply) always runs
	if hasFlatpak {
		total++
	}
	stepIdx := 0
	nextStep := func(label string) {
		stepIdx++
		ui.Step(stdout, stepIdx, total, label)
	}

	nextStep("Atualizando pacotes do sistema...")
	ui.Info(stdout, "Solicitando autenticação de administrador (uma única vez para as etapas acima)...")
	if err := exe.RunSudoSequence(ctx, executor.Options{Stdout: stdout, Stderr: stdout}, steps); err != nil {
		ui.Err(stdout, "Falha ao atualizar pacotes: "+err.Error())
		return err
	}

	if hasFlatpak {
		nextStep("Atualizando Flatpaks...")
		if err := updateFlatpak(ctx, exe, stdout, opts.Autoremove); err != nil {
			ui.Warning(stdout, "Falha ao atualizar flatpaks: "+err.Error())
		}
	} else {
		ui.Info(stdout, "Flatpak não instalado. Etapa ignorada.")
	}

	ui.Success(stdout, "Atualização finalizada com sucesso.")
	return nil
}

// aptEnv is the extra environment apt-get needs to never block on a TTY
// prompt (ex. a config-file conflict dialog) mid-batch.
var aptEnv = []string{"DEBIAN_FRONTEND=noninteractive"}

// packageSteps returns family's package-manager commands as
// executor.RunSudoSequence steps. Announce carries each step's narration;
// full-upgrade alone covers the upgrade.
func packageSteps(family string, autoremove bool) ([]executor.PrivilegedStep, error) {
	aptOpts := []string{"-o", "APT::Color=0", "-o", "Dpkg::Progress-Fancy=0", "-o", "Dpkg::Use-Pty=0"}
	switch family {
	case distro.Debian:
		steps := []executor.PrivilegedStep{
			{Announce: "Sincronizando lista de pacotes...", Env: aptEnv, Name: "apt-get", Args: append([]string{"update"}, aptOpts...)},
			{Announce: "Instalando atualizações...", Env: aptEnv, Name: "apt-get", Args: append([]string{"full-upgrade", "-y"}, aptOpts...)},
		}
		if autoremove {
			steps = append(steps, executor.PrivilegedStep{Announce: "Removendo pacotes desnecessários...", Env: aptEnv, Name: "apt-get", Args: append([]string{"autoremove", "-y"}, aptOpts...)})
		}
		return append(steps, executor.PrivilegedStep{Announce: "Limpando cache APT...", Env: aptEnv, Name: "apt-get", Args: []string{"autoclean", "-y"}}), nil
	case distro.Fedora:
		steps := []executor.PrivilegedStep{
			{Announce: "Atualizando pacotes DNF...", Name: "dnf", Args: []string{"upgrade", "--refresh", "-y"}},
		}
		if autoremove {
			steps = append(steps, executor.PrivilegedStep{Announce: "Removendo pacotes desnecessários...", Soft: true, WarnMessage: "Falha no dnf autoremove.", Name: "dnf", Args: []string{"autoremove", "-y"}})
		}
		return steps, nil
	default:
		return nil, fmt.Errorf("família de distribuição não suportada: %s", family)
	}
}

// updateFlatpak runs outside the privileged batch (it never needs root),
// so its ui.Step call in Run is this phase's only announcement.
//
// The update has no --user/--system flag, so it covers both installations,
// including apps installed outside the configured scope. Removing unused
// runtimes is the autoremove option's Flatpak half, in the configured
// scope.
func updateFlatpak(ctx context.Context, exe *executor.Executor, stdout io.Writer, autoremove bool) error {
	if !exe.CommandAvailable(ctx, "flatpak") {
		ui.Info(stdout, "Flatpak não instalado. Etapa ignorada.")
		return nil
	}
	opts := executor.Options{Stdout: stdout, Stderr: stdout, Env: []string{"TERM=dumb"}}
	if err := exe.Run(ctx, opts, "flatpak", "update", "--noninteractive", "-y"); err != nil {
		return fmt.Errorf("flatpak update: %w", err)
	}
	if !autoremove {
		return nil
	}
	if err := exe.Run(ctx, opts, "flatpak", "uninstall", "--noninteractive", config.FlatpakFlag(), "--unused", "-y"); err != nil {
		ui.Warning(stdout, "Falha ao remover flatpaks não utilizados: "+err.Error())
	}
	return nil
}
