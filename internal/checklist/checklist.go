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

// Package checklist holds the shared "checklist simples" loop shape used
// by every GUI screen that installs/removes a batch of catalog items and
// reports one ui.Step per item — internal/dev/{prereqs,llm,terminal},
// internal/dev/sdks and internal/system/fonts/templates/apps all used to
// duplicate this same loop.
package checklist

import (
	"errors"
	"fmt"
	"io"

	"github.com/oito2/perci/internal/sets"
	"github.com/oito2/perci/internal/ui"
)

// Apply drives the install/remove pass over catalogue: every item named in
// toInstall gets install(item) called (removed items get remove(item)),
// with one ui.Step per item processed — total comes from the selection
// itself, same convention every caller already followed. A failure is
// warned inline and does not stop the rest of the batch; Apply then returns
// every failure joined, so the caller (and the GUI) don't report success.
func Apply[T any](
	stdout io.Writer,
	catalogue []T,
	nameOf func(T) string,
	toInstall, toRemove []string,
	install, remove func(T) error,
) error {
	installSet := sets.Of(toInstall)
	removeSet := sets.Of(toRemove)

	total := len(toInstall) + len(toRemove)
	step := 0
	var errs []error

	for _, item := range catalogue {
		name := nameOf(item)
		switch {
		case installSet[name]:
			step++
			ui.Step(stdout, step, total, "Instalando "+name+"...")
			if err := install(item); err != nil {
				ui.Warning(stdout, fmt.Sprintf("Falha ao instalar %s: %v", name, err))
				errs = append(errs, fmt.Errorf("instalar %s: %w", name, err))
			}
		case removeSet[name]:
			step++
			ui.Step(stdout, step, total, "Removendo "+name+"...")
			if err := remove(item); err != nil {
				ui.Warning(stdout, fmt.Sprintf("Falha ao remover %s: %v", name, err))
				errs = append(errs, fmt.Errorf("remover %s: %w", name, err))
			}
		}
	}
	return errors.Join(errs...)
}
