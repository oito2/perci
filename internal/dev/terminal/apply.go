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

package terminal

import (
	"context"
	"io"

	"github.com/oito2/perci/internal/checklist"
	"github.com/oito2/perci/internal/dev/localbin"
	"github.com/oito2/perci/internal/distro"
	"github.com/oito2/perci/internal/executor"
)

// Apply installs terminal emulators listed in toInstall and removes those in
// toRemove — GUI-only screen ("Desenvolvimento :: Aplicativos - Terminais"),
// same pattern as internal/system/fonts.Apply: one ui.Step per processed
// terminal, with the total coming for free from the selection itself
// (internal/checklist.Apply). Starship doesn't go through here — it's
// applied via its own button ("Aplicar Starship" — InstallStarship), it's
// not part of this checklist (see catalogue.go).
//
// Every run ends by syncing the file-manager context-menu entries with what
// is actually installed (SyncContextMenuEntries) — even when an item failed,
// and also covering terminals installed before this screen existed.
func Apply(ctx context.Context, exe *executor.Executor, stdout io.Writer, toInstall, toRemove []string) error {
	family := distro.Detect()
	err := checklist.Apply(stdout, Catalogue, func(t Terminal) string { return t.Name }, toInstall, toRemove,
		func(t Terminal) error { return InstallOne(ctx, exe, stdout, t, family) },
		func(t Terminal) error { return UninstallOne(ctx, exe, stdout, t, family) },
	)
	if !exe.DryRun {
		// Kitty lands in ~/.local/bin: refresh PATH so InstalledMap sees it.
		localbin.RefreshPath(ctx, exe)
		SyncContextMenuEntries(InstalledMap(ctx, exe), stdout)
	}
	return err
}
