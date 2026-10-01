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

package sets

// Of converts a slice of strings into a set for fast membership testing.
func Of(items []string) map[string]bool {
	s := make(map[string]bool, len(items))
	for _, item := range items {
		s[item] = true
	}
	return s
}

// Diff compares allKeys (the full catalogue, in display order) against
// installed (currently installed/present, keyed the same way as allKeys)
// and selectedNames (a multi-select's current value) to decide what
// changed — shared by every "toggle install state via multi-select"
// checklist screen (fonts, templates, Flatpak apps, prerequisites, ...).
func Diff(allKeys []string, installed map[string]bool, selectedNames []string) (toInstall, toRemove []string) {
	selectedMap := Of(selectedNames)
	for _, k := range allKeys {
		switch {
		case selectedMap[k] && !installed[k]:
			toInstall = append(toInstall, k)
		case !selectedMap[k] && installed[k]:
			toRemove = append(toRemove, k)
		}
	}
	return toInstall, toRemove
}
