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
	"fmt"
	"io"
	"strings"

	"github.com/oito2/perci/internal/config"
	"github.com/oito2/perci/internal/distro"
	"github.com/oito2/perci/internal/executor"
	"github.com/oito2/perci/internal/ui"
)

// installedByScope returns app IDs installed in a specific Flatpak scope.
func installedByScope(ctx context.Context, exe *executor.Executor, scope string) map[string]bool {
	out, err := exe.Output(ctx, executor.Options{}, "flatpak", "list", scope, "--app", "--columns=application")
	if err != nil {
		return map[string]bool{}
	}
	return parseFlatpakIDs(out)
}

// parseFlatpakIDs parses `flatpak list --columns=application`'s output (one
// app ID per line) into a set — split out from installedByScope so this
// parsing logic is testable without a real flatpak/subprocess call, same
// convention as megasync.resolveRepo/update.packageSteps in sibling
// packages.
func parseFlatpakIDs(out string) map[string]bool {
	result := make(map[string]bool)
	for _, line := range strings.Split(out, "\n") {
		if id := strings.TrimSpace(line); id != "" {
			result[id] = true
		}
	}
	return result
}

// listAll returns a map from app ID to its scope flag ("--system" or "--user"),
// querying both scopes in a single pass. System takes precedence over user.
func listAll(ctx context.Context, exe *executor.Executor) map[string]string {
	result := make(map[string]string)
	for id := range installedByScope(ctx, exe, "--user") {
		result[id] = "--user"
	}
	for id := range installedByScope(ctx, exe, "--system") {
		result[id] = "--system"
	}
	return result
}

// InstalledIDs returns all Flatpak app IDs installed in either system or user scope.
func InstalledIDs(ctx context.Context, exe *executor.Executor) map[string]bool {
	all := listAll(ctx, exe)
	out := make(map[string]bool, len(all))
	for id := range all {
		out[id] = true
	}
	return out
}

// InstalledScopeMap returns a map from app ID to its scope flag ("--system" or "--user").
// When an app is installed in both scopes, "--system" takes precedence.
func InstalledScopeMap(ctx context.Context, exe *executor.Executor) map[string]string {
	return listAll(ctx, exe)
}

// flathubRepoURL is Flathub's official .flatpakrepo.
const flathubRepoURL = "https://dl.flathub.org/repo/flathub.flatpakrepo"

// FlatpakSetupSteps returns what makes sure flatpak is installed AND the
// Flathub remote exists in the configured scope (config.FlatpakFlag):
// privileged steps to run first (installing flatpak when missing; adding a
// system-scope remote) and, for the user scope, the unprivileged remote-add
// to run after them. The remote is added every time (idempotent,
// --if-not-exists): many distros ship flatpak without Flathub, or with it
// only in the other scope. Callers put the privileged steps in the same
// batch as their own installs, so the whole action asks for the password
// once.
func FlatpakSetupSteps(ctx context.Context, exe *executor.Executor) (privileged []executor.PrivilegedStep, userRemoteAdd *executor.PrivilegedStep, err error) {
	scope := config.FlatpakFlag()
	remoteAdd := executor.PrivilegedStep{
		Announce: "Configurando repositório Flathub...",
		Env:      []string{"TERM=dumb"},
		Name:     "flatpak",
		Args:     []string{"remote-add", scope, "--if-not-exists", "flathub", flathubRepoURL},
	}

	if !exe.CommandAvailable(ctx, "flatpak") {
		switch distro.Detect() {
		case distro.Debian:
			privileged = append(privileged, executor.PrivilegedStep{
				Announce: "Instalando Flatpak...",
				Env:      []string{"DEBIAN_FRONTEND=noninteractive"},
				Name:     "apt-get",
				Args:     []string{"install", "-y", "-o", "Dpkg::Use-Pty=0", "-o", "Dpkg::Progress-Fancy=0", "-o", "APT::Color=0", "--", "flatpak"},
			})
		case distro.Fedora:
			privileged = append(privileged, executor.PrivilegedStep{Announce: "Instalando Flatpak...", Name: "dnf", Args: []string{"install", "-y", "--", "flatpak"}})
		default:
			return nil, nil, fmt.Errorf("instale o flatpak manualmente nesta distribuição")
		}
	}
	if scope == "--system" {
		return append(privileged, remoteAdd), nil, nil
	}
	return privileged, &remoteAdd, nil
}

// EnsureFlatpak runs FlatpakSetupSteps on its own — one privileged batch,
// then the user-scope remote-add when that's the scope.
func EnsureFlatpak(ctx context.Context, exe *executor.Executor, stdout io.Writer) error {
	privileged, userRemoteAdd, err := FlatpakSetupSteps(ctx, exe)
	if err != nil {
		return err
	}
	return runFlatpakSetup(ctx, exe, stdout, privileged, userRemoteAdd)
}

func runFlatpakSetup(ctx context.Context, exe *executor.Executor, stdout io.Writer, privileged []executor.PrivilegedStep, userRemoteAdd *executor.PrivilegedStep) error {
	if len(privileged) > 0 {
		if err := exe.RunSudoSequence(ctx, executor.Options{Stdout: stdout, Stderr: stdout}, privileged); err != nil {
			return fmt.Errorf("preparar flatpak: %w", err)
		}
	}
	if userRemoteAdd != nil {
		ui.Info(stdout, userRemoteAdd.Announce)
		return exe.Run(ctx, executor.Options{Stdout: stdout, Stderr: stdout, Env: userRemoteAdd.Env}, userRemoteAdd.Name, userRemoteAdd.Args...)
	}
	return nil
}

// installOne installs a single Flatpak app and applies its FlatpakOverride
// (if any) — used by Apply's (apply.go) unbatched (user-scope) install path
// and by its ui.Step loop.
func installOne(ctx context.Context, exe *executor.Executor, stdout io.Writer, appByID map[string]App, scope, id string) error {
	ui.Info(stdout, "Instalando: "+id)
	if err := exe.Run(ctx,
		executor.Options{RequiresSudo: scope == "--system", Stdout: stdout, Stderr: stdout, Env: []string{"TERM=dumb"}},
		"flatpak", "install", "--noninteractive", scope, "-y", "flathub", id,
	); err != nil {
		return err
	}
	if app, ok := appByID[id]; ok && len(app.FlatpakOverride) > 0 {
		overrideArgs := append([]string{"override", scope, id}, app.FlatpakOverride...)
		if err := exe.Run(ctx,
			executor.Options{RequiresSudo: scope == "--system", Stdout: stdout, Stderr: stdout},
			"flatpak", overrideArgs...,
		); err != nil {
			ui.Warning(stdout, fmt.Sprintf("Override do %s falhou: %v", id, err))
		}
	}
	return nil
}
