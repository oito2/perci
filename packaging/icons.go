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

// Package packaging embeds the application icon sets under icons/ — one
// freedesktop hicolor tree per mascot color ("blue"/"pink",
// internal/config.AppIcons), square, 5% transparent margin. The same files
// are installed by `make install`/install.sh (straight from this folder)
// and by "Home :: Configurações :: Ícone do Aplicativo" (from this embed,
// internal/selfupdate.InstallMenuIcons), and the 512 px one doubles as the
// window icon (cmd/prci-gui/main.go).
package packaging

import (
	"embed"
	"fmt"
)

//go:embed icons
var icons embed.FS

// IconSizes lists every hicolor size shipped for each color, largest first.
var IconSizes = []int{512, 256, 128, 64, 48, 32}

// IconName is the icon's freedesktop name — packaging/perci.desktop's
// Icon= key and the installed file's basename.
const IconName = "perci"

// IconRelPath returns the icon's path relative to a hicolor theme root
// (e.g. "512x512/apps/perci.png").
func IconRelPath(size int) string {
	return fmt.Sprintf("%dx%d/apps/%s.png", size, size, IconName)
}

// Icon returns the PNG bytes for color at size.
func Icon(color string, size int) ([]byte, error) {
	return icons.ReadFile("icons/" + color + "/hicolor/" + IconRelPath(size))
}

// MustIcon is Icon for package-level initialization — every color/size
// pair is embedded at build time, so a failure here means a broken build.
func MustIcon(color string, size int) []byte {
	data, err := Icon(color, size)
	if err != nil {
		panic(err)
	}
	return data
}
