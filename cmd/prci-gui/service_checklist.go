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

package main

// checklistCatalog/getChecklistInfo/runChecklist generalize the "checklist
// simples" Get*Info/Run* pair shared by the GUI checklist screens: list a
// catalogue with its current installed state, then diff a frontend
// selection against it and apply the install/remove batch.

import (
	"io"

	"github.com/oito2/perci/internal/sets"
)

// checklistCatalog declares everything a "checklist simples" screen needs:
// its items, how to read an ID/label/(optional) description from one, its
// current installed state (recalculated fresh on every call — never
// trusting what the frontend cached), and how to apply an install/remove
// batch.
type checklistCatalog[T any] struct {
	items     []T
	idOf      func(T) string
	labelOf   func(T) string
	descOf    func(T) string // optional — nil when the screen doesn't use a description
	warnOf    func(T) string // optional — MultiSelectItemInfo.RemoveWarning
	installed func() map[string]bool
	apply     func(stdout io.Writer, toInstall, toRemove []string) error
}

// getChecklistInfo renders c as the []MultiSelectItemInfo every "checklist
// simples" GetXInfo method returns.
func getChecklistInfo[T any](c checklistCatalog[T]) []MultiSelectItemInfo {
	installed := c.installed()
	items := make([]MultiSelectItemInfo, len(c.items))
	for i, it := range c.items {
		id := c.idOf(it)
		desc := ""
		if c.descOf != nil {
			desc = c.descOf(it)
		}
		items[i] = MultiSelectItemInfo{ID: id, Label: c.labelOf(it), Description: desc, Installed: installed[id]}
		if c.warnOf != nil {
			items[i].RemoveWarning = c.warnOf(it)
		}
	}
	return items
}

// runChecklist diffs selectedIDs against c's current installed state
// (internal/sets.Diff) and, when there's anything to do, applies the batch
// via c.apply inside b's runAction.
func runChecklist[T any](b *serviceBase, c checklistCatalog[T], selectedIDs []string) error {
	return b.runAction(func(stdout io.Writer) error {
		installed := c.installed()
		allIDs := make([]string, len(c.items))
		for i, it := range c.items {
			allIDs[i] = c.idOf(it)
		}
		toInstall, toRemove := sets.Diff(allIDs, installed, selectedIDs)
		if len(toInstall) == 0 && len(toRemove) == 0 {
			return nil
		}
		return c.apply(stdout, toInstall, toRemove)
	})
}
