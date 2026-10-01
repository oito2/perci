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
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/oito2/perci/internal/config"
	"github.com/oito2/perci/internal/executor"
	"github.com/oito2/perci/internal/sets"
	"github.com/oito2/perci/internal/ui"
)

// unprivilegedAppJob is one user-scope Flatpak app install/uninstall queued
// for parallel execution — system-scope apps stay out of this: they're
// already batched into one sudo/pkexec authentication via RunSudoSequence
// below, which runs as a single script and gains nothing from
// parallelizing.
type unprivilegedAppJob struct {
	app    App
	remove bool // false = install
	scope  string
}

// Apply installs apps listed in toInstall and uninstalls those in
// toUninstall — combined operation for the "checklist simples" GUI screen
// (Linux :: Aplicativos - Flatpak), one ui.Step per app processed
// (installed or uninstalled), same pattern as
// fonts.Apply/templates.Apply. Driven by sets.Diff, so both toInstall
// and toUninstall are Catalogue members — iterates Catalogue in declared
// order, like fonts.Apply.
func Apply(ctx context.Context, exe *executor.Executor, stdout io.Writer, toInstall, toUninstall []string) error {
	total := len(toInstall) + len(toUninstall)
	if total == 0 {
		return nil
	}

	scope := config.FlatpakFlag()
	// Setting flatpak up (install it, add Flathub) joins the install batch
	// below for the system scope — one password prompt for the whole click.
	// For the user scope it runs first: the user-scope installs need it.
	var setupSteps []executor.PrivilegedStep
	if len(toInstall) > 0 {
		privileged, userRemoteAdd, err := FlatpakSetupSteps(ctx, exe)
		if err != nil {
			return err
		}
		if scope == "--system" {
			setupSteps = privileged
		} else if err := runFlatpakSetup(ctx, exe, stdout, privileged, userRemoteAdd); err != nil {
			return err
		}
	}

	appByID := make(map[string]App, len(Catalogue))
	for _, a := range Catalogue {
		appByID[a.FlatID] = a
	}
	scopeMap := InstalledScopeMap(ctx, exe)

	installSet := sets.Of(toInstall)
	uninstallSet := sets.Of(toUninstall)
	step := 0
	var failed []string
	var failedMu sync.Mutex

	// Every step that needs root — every install (always at the config's
	// global scope) and any uninstall whose app happens to be installed at
	// system scope — is collected here instead of run immediately, so they
	// all go through ONE sudo/pkexec authentication via RunSudoSequence
	// below instead of one prompt per app: pkexec (the GUI's escalation
	// path) has no session cache the way sudo does, so this used to mean
	// one graphical password dialog per app for a single "Aplicar" click
	// (same reasoning as internal/system/update.Run, decided with the
	// user on 2026-09-14). Their failures are warned inline (Soft steps,
	// same messages as before) but can't be added to the `failed` list
	// below — RunSudoSequence doesn't report which individual step failed,
	// only the whole batch's exit status.
	privSteps := setupSteps
	// User-scope installs/uninstalls need no root at all — queued into
	// jobs and run concurrently below instead of one at a time.
	var jobs []unprivilegedAppJob

	for _, app := range Catalogue {
		switch {
		case installSet[app.FlatID]:
			step++
			ui.Step(stdout, step, total, "Instalando "+app.Name+"...")
			if scope == "--system" {
				privSteps = append(privSteps, installStep(app.FlatID, scope))
				if len(app.FlatpakOverride) > 0 {
					privSteps = append(privSteps, overrideStep(app.FlatID, scope, app.FlatpakOverride))
				}
				continue
			}
			jobs = append(jobs, unprivilegedAppJob{app: app, scope: scope})
		case uninstallSet[app.FlatID]:
			step++
			ui.Step(stdout, step, total, "Desinstalando "+app.Name+"...")
			appScope, ok := scopeMap[app.FlatID]
			if !ok {
				ui.Warning(stdout, app.Name+" não está instalado, ignorando.")
				continue
			}
			if appScope == "--system" {
				privSteps = append(privSteps, uninstallStep(app.FlatID, appScope))
				continue
			}
			jobs = append(jobs, unprivilegedAppJob{app: app, remove: true, scope: appScope})
		}
	}

	if len(jobs) > 0 {
		syncOut := executor.NewSyncWriter(stdout)
		runErr := executor.RunConcurrent(ctx, jobs, 4, func(j unprivilegedAppJob) {
			var err error
			verb := "instalar"
			if j.remove {
				verb = "desinstalar"
				err = uninstallOne(ctx, exe, syncOut, j.app.FlatID, j.scope)
			} else {
				err = installOne(ctx, exe, syncOut, appByID, j.scope, j.app.FlatID)
			}
			if err != nil {
				ui.Warning(syncOut, fmt.Sprintf("Falha ao %s %s: %v", verb, j.app.Name, err))
				failedMu.Lock()
				failed = append(failed, j.app.Name)
				failedMu.Unlock()
			}
		})
		if runErr != nil {
			ui.Warning(stdout, "Execução paralela interrompida: "+runErr.Error())
			failed = append(failed, "(interrompido)")
		}
	}

	var batchErr error
	if len(privSteps) > 0 {
		batchErr = exe.RunSudoSequence(ctx, executor.Options{Stdout: stdout, Stderr: stdout}, privSteps)
	}

	if len(failed) > 0 {
		return fmt.Errorf("%d aplicativo(s) com falha: %s", len(failed), strings.Join(failed, ", "))
	}
	if errors.Is(batchErr, executor.ErrCompletedWithWarnings) {
		return batchErr // some system-scope app failed; each warned inline
	}
	if batchErr != nil {
		return fmt.Errorf("lote de operações privilegiadas falhou: %w", batchErr)
	}
	return nil
}

// installStep/overrideStep/uninstallStep build the RunSudoSequence steps for
// Apply's batched (system-scope) path — same flatpak invocations installOne/
// uninstallOne run individually for the unbatched (user-scope) path, just
// not executed immediately.
func installStep(id, scope string) executor.PrivilegedStep {
	return executor.PrivilegedStep{
		Announce: "Instalando: " + id,
		Soft:     true, SoftReport: true, WarnMessage: "Falha ao instalar " + id + ".",
		Name: "flatpak", Args: []string{"install", "--noninteractive", scope, "-y", "flathub", id},
		Env: []string{"TERM=dumb"},
	}
}

func overrideStep(id, scope string, overrideArgs []string) executor.PrivilegedStep {
	args := append([]string{"override", scope, id}, overrideArgs...)
	return executor.PrivilegedStep{
		Announce: "Aplicando override do " + id + "...",
		Soft:     true, SoftReport: true, WarnMessage: "Override do " + id + " falhou.",
		Name: "flatpak", Args: args,
	}
}

func uninstallStep(id, scope string) executor.PrivilegedStep {
	return executor.PrivilegedStep{
		Announce: "Desinstalando: " + id,
		Soft:     true, SoftReport: true, WarnMessage: "Falha ao desinstalar " + id + ".",
		Name: "flatpak", Args: []string{"uninstall", "--noninteractive", scope, "-y", id},
		Env: []string{"TERM=dumb"},
	}
}
