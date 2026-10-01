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

package postinstall

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/oito2/perci/internal/config"
	"github.com/oito2/perci/internal/executor"
	"github.com/oito2/perci/internal/system/apps"
	"github.com/oito2/perci/internal/ui"
)

// step prints a step panel and runs a sudo command.
func step(ctx context.Context, exe *executor.Executor, stdout io.Writer, msg, name string, args ...string) error {
	ui.Info(stdout, msg)
	return exe.Run(ctx, executor.Options{RequiresSudo: true, Stdout: stdout, Stderr: stdout}, name, args...)
}

// aptInstall installs one or more apt packages with sudo, non-interactively
// (aptBaseOpts, debian.go).
func aptInstall(ctx context.Context, exe *executor.Executor, stdout io.Writer, pkgs ...string) error {
	args := append(append(append([]string{}, aptBaseOpts...), "install", "-y", "--"), pkgs...)
	return exe.Run(ctx, executor.Options{
		RequiresSudo: true,
		Stdout:       stdout,
		Stderr:       stdout,
		Env:          []string{"DEBIAN_FRONTEND=noninteractive"},
	}, "apt-get", args...)
}

// dnfInstall installs one or more dnf packages with sudo.
func dnfInstall(ctx context.Context, exe *executor.Executor, stdout io.Writer, pkgs ...string) error {
	args := append([]string{"install", "-y", "--"}, pkgs...)
	return exe.Run(ctx, executor.Options{RequiresSudo: true, Stdout: stdout, Stderr: stdout}, "dnf", args...)
}

// ensureFlatpakReady installs flatpak if needed and ensures the Flathub
// remote — apps.EnsureFlatpak, shared with Linux :: Apps so both paths
// behave the same (this used to be a second, diverging copy).
func ensureFlatpakReady(ctx context.Context, exe *executor.Executor, stdout io.Writer) error {
	return apps.EnsureFlatpak(ctx, exe, stdout)
}

// flatpakSetupAndInstall makes sure flatpak and Flathub are there and
// installs appIDs — for the system scope as ONE privileged batch (one
// password prompt instead of one for the setup and another for the
// install).
func flatpakSetupAndInstall(ctx context.Context, exe *executor.Executor, stdout io.Writer, appIDs ...string) error {
	if config.FlatpakFlag() != "--system" {
		if err := ensureFlatpakReady(ctx, exe, stdout); err != nil {
			return err
		}
		return flatpakInstall(ctx, exe, stdout, appIDs...)
	}
	steps, _, err := apps.FlatpakSetupSteps(ctx, exe)
	if err != nil {
		return err
	}
	steps = append(steps, executor.PrivilegedStep{
		Announce: "Instalando " + strings.Join(appIDs, ", ") + "...",
		Env:      []string{"TERM=dumb"},
		Name:     "flatpak",
		Args:     append([]string{"install", "--noninteractive", "--system", "-y", "flathub"}, appIDs...),
	})
	return exe.RunSudoSequence(ctx, executor.Options{Stdout: stdout, Stderr: stdout}, steps)
}

// flatpakInstall installs Flatpak apps from Flathub using the configured
// scope — privileged for --system, the same as Linux :: Apps does (it used
// to run unprivileged and depend on flatpak's own polkit prompt).
func flatpakInstall(ctx context.Context, exe *executor.Executor, stdout io.Writer, appIDs ...string) error {
	scope := config.FlatpakFlag()
	args := append([]string{"install", "--noninteractive", scope, "-y", "flathub"}, appIDs...)
	return exe.Run(ctx, executor.Options{RequiresSudo: scope == "--system", Stdout: stdout, Stderr: stdout, Env: []string{"TERM=dumb"}}, "flatpak", args...)
}

const (
	sysctlConfPath       = "/etc/sysctl.d/99-perci.conf"
	legacySysctlConfPath = "/etc/sysctl.d/99-lumina.conf"
)

// configureSysctl sets swappiness, inotify and applies sysctl — both
// commands run as ONE sudo/pkexec authentication (rather than two separate
// RequiresSudo calls, i.e. two password prompts for a single action;
// pkexec has no session cache the way sudo does — same reasoning as
// internal/system/update.Run).
//
// The file used to be 99-lumina.conf (the project's old name): it's
// removed when present, so the two never both apply.
func configureSysctl(ctx context.Context, exe *executor.Executor, stdout io.Writer) error {
	ui.Info(stdout, "Aplicando configurações de kernel (sysctl)...")
	conf := "vm.swappiness=10\nfs.inotify.max_user_watches=524288\n"
	path := sysctlConfPath
	script := fmt.Sprintf(`set -Eeuo pipefail
printf '%%s' %s | tee %s > /dev/null
rm -f -- %s
sysctl -p %s
`, executor.ShellQuote(conf), executor.ShellQuote(path), executor.ShellQuote(legacySysctlConfPath), executor.ShellQuote(path))
	return exe.Run(ctx, executor.Options{RequiresSudo: true, Stdout: stdout, Stderr: stdout}, "bash", "-c", script)
}

// acceptEulaAndAptInstall pre-accepts the ttf-mscorefonts-installer EULA and
// installs pkgs (every caller needs both together: pkgs is either that
// package itself or a distro metapackage that pulls it in) as ONE sudo/
// pkexec authentication instead of two separate prompts (pkexec has no
// session cache the way sudo does — same reasoning as internal/system/
// update.Run). debconf-set-selections
// reads its selection from stdin/a file, never as a literal argument —
// PrivilegedStep has no Stdin, so the value is piped to it from inside a
// "bash -c" step instead.
func acceptEulaAndAptInstall(ctx context.Context, exe *executor.Executor, stdout io.Writer, pkgs ...string) error {
	const eulaSelection = "ttf-mscorefonts-installer msttcorefonts/accepted-mscorefonts-eula select true"
	pipeEula := "printf '%s\\n' " + executor.ShellQuote(eulaSelection) + " | debconf-set-selections"
	installArgs := append(append(append([]string{}, aptBaseOpts...), "install", "-y", "--"), pkgs...)
	return exe.RunSudoSequence(ctx, executor.Options{Stdout: stdout, Stderr: stdout}, []executor.PrivilegedStep{
		{Announce: "Pré-aceitando EULA das fontes Microsoft...", Name: "bash", Args: []string{"-c", pipeEula}},
		{
			Announce: "Instalando " + strings.Join(pkgs, ", ") + "...",
			Env:      []string{"DEBIAN_FRONTEND=noninteractive"},
			Name:     "apt-get", Args: installArgs,
		},
	})
}

// aptComponentEnabled reports whether the given apt component (e.g. "universe")
// is already present in at least one enabled source. Checks both legacy .list
// and DEB822 .sources formats. Returns false on any read error (safe to retry).
func aptComponentEnabled(ctx context.Context, exe *executor.Executor, component string) bool {
	out, _ := exe.Output(ctx, executor.Options{},
		"bash", "-c",
		"grep -rEh '^deb[^-]|^Components:' /etc/apt/sources.list /etc/apt/sources.list.d/ 2>/dev/null")
	for _, line := range strings.Split(out, "\n") {
		for _, field := range strings.Fields(line) {
			if field == component {
				return true
			}
		}
	}
	return false
}

// rpmFusionEnabled reports whether the rpmfusion-free-release package is installed.
func rpmFusionEnabled(ctx context.Context, exe *executor.Executor) bool {
	_, err := exe.Output(ctx, executor.Options{}, "rpm", "-q", "rpmfusion-free-release")
	return err == nil
}

func stripNewline(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}

// setupSwapfile creates a 4 GB swapfile at /swapfile and persists it in
// /etc/fstab — unless the system already has any active swap (Ubuntu's
// installer creates /swap.img; a second 4 GB swap on top was never the
// intent). On btrfs the file is made with `btrfs filesystem mkswapfile`
// (a fallocate'd file on a copy-on-write filesystem can't be swapped on),
// and a swapon failure removes the file, so a later run isn't fooled into
// "already exists". Non-fatal: shows warnings on failure.
func setupSwapfile(ctx context.Context, exe *executor.Executor, stdout io.Writer) {
	ui.Info(stdout, "Configurando swapfile...")

	if fi, err := os.Lstat("/swapfile"); err == nil && fi.Mode()&os.ModeSymlink != 0 {
		ui.Warning(stdout, "/swapfile é um link simbólico — abortando por segurança.")
		return
	}

	if err := exe.Run(ctx, executor.Options{RequiresSudo: true, Stdout: stdout, Stderr: stdout}, "bash", "-c", swapfileScript); err != nil {
		ui.Warning(stdout, "Falha ao configurar swapfile: "+err.Error())
		return
	}
}

// swapfileScript is setupSwapfile's privileged part — one password prompt.
// The fstab entry is matched only as an uncommented first field.
const swapfileScript = `set -Eeuo pipefail
if [ -n "$(swapon --noheadings --show=NAME 2>/dev/null)" ]; then
  echo "O sistema já tem swap ativo — nada a fazer."
  exit 0
fi
if [ -e /swapfile ]; then
  echo "/swapfile já existe, mas não está ativo — verifique-o manualmente." >&2
  exit 1
fi
if [ "$(findmnt -no FSTYPE / 2>/dev/null || true)" = btrfs ]; then
  btrfs filesystem mkswapfile --size 4g /swapfile || { echo "Falha ao criar swapfile (btrfs)." >&2; rm -f -- /swapfile; exit 1; }
else
  fallocate -l 4G /swapfile || { echo "Falha ao criar swapfile." >&2; rm -f -- /swapfile; exit 1; }
  chmod 600 /swapfile || { echo "Falha ao definir permissões do swapfile." >&2; rm -f -- /swapfile; exit 1; }
  mkswap /swapfile || { echo "Falha ao formatar swapfile." >&2; rm -f -- /swapfile; exit 1; }
fi
swapon /swapfile || { echo "Falha ao ativar swapfile." >&2; rm -f -- /swapfile; exit 1; }
grep -qE '^[[:space:]]*/swapfile[[:space:]]' /etc/fstab || printf '/swapfile none swap sw 0 0\n' >> /etc/fstab || echo "Falha ao atualizar /etc/fstab." >&2
echo "Swapfile de 4 GB criado e ativado."
`
