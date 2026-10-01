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

package prereqs

import (
	"context"
	"io"

	"github.com/oito2/perci/internal/checklist"
	"github.com/oito2/perci/internal/executor"
)

// Apply installs prerequisites listed in toInstall and removes those in
// toRemove — GUI-only screen ("Desenvolvimento :: Pré-requisitos"), same
// pattern as internal/system/fonts.Apply: one ui.Step per processed item,
// with the total coming for free from the selection itself
// (internal/checklist.Apply).
func Apply(ctx context.Context, exe *executor.Executor, stdout io.Writer, toInstall, toRemove []string) error {
	return checklist.Apply(stdout, Catalogue, func(p Prereq) string { return p.Name }, toInstall, toRemove,
		func(p Prereq) error { return InstallOne(ctx, exe, stdout, p) },
		func(p Prereq) error { return UninstallOne(ctx, exe, stdout, p) },
	)
}
