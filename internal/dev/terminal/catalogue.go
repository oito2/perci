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
	"strings"

	"github.com/oito2/perci/internal/config"
	"github.com/oito2/perci/internal/executor"
)

// Terminal describes a terminal emulator managed by perci.gnl.
type Terminal struct {
	Name     string
	Cmd      string
	FlatID   string // non-empty when installed via Flatpak
	DirFlag  string // flag to open in a directory (empty = no context menu entry)
	MenuExec string // exec command for context menus; defaults to Cmd when empty
}

// Catalogue lists all terminal emulators managed by perci.gnl. Starship is
// not listed; it is applied through InstallStarship.
var Catalogue = []Terminal{
	{Name: "Kitty", Cmd: "kitty", DirFlag: "--directory"},
	{Name: "Alacritty", Cmd: "alacritty", DirFlag: "--working-directory"},
	{Name: "Black Box", Cmd: "blackbox-terminal", FlatID: "com.raggesilver.BlackBox", DirFlag: "--working-directory", MenuExec: "flatpak run com.raggesilver.BlackBox"},
	{Name: "GNOME Console", Cmd: "kgx", DirFlag: "--working-directory"},
}

// InstalledMap returns which terminals are currently installed (by Name).
func InstalledMap(ctx context.Context, exe *executor.Executor) map[string]bool {
	result := make(map[string]bool, len(Catalogue))
	flatpakOut, _ := exe.Output(ctx, executor.Options{},
		"flatpak", "list", config.FlatpakFlag(), "--app", "--columns=application")
	for _, t := range Catalogue {
		if t.FlatID != "" && contains(flatpakOut, t.FlatID) {
			result[t.Name] = true
			continue
		}
		if exe.CommandAvailable(ctx, t.Cmd) {
			result[t.Name] = true
		}
	}
	return result
}

func contains(haystack, needle string) bool {
	for _, line := range strings.Split(haystack, "\n") {
		if strings.TrimSpace(line) == needle {
			return true
		}
	}
	return false
}
