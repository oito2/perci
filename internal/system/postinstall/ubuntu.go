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

import "slices"

var ubuntuPackages = []string{
	"ubuntu-restricted-extras",
	"ffmpeg",
	"gnome-tweaks",
	"build-essential",
	"gparted",
	"gdebi",
	"libfuse2t64",
	"unrar",
	"unzip",
	"ntfs-3g",
	"7zip",
	"curl",
	"wget",
	"git",
	"htop",
	"make",
	"tree",
	"jq",
	"plocate",
	"net-tools",
	"python3-pip",
	"software-properties-common",
	"ubuntu-drivers-common",
	"timeshift",
	"gnome-software-plugin-flatpak",
}

// ubuntuActions is the checklist for both Ubuntu 24.04 and 26.04, which
// share the same apt commands; each version has its own Profile. Built
// from the shared Debian-family blocks.
func ubuntuActions() []Action {
	return []Action{
		actEnableUbuntuComponents(),
		actUpdateIndex("enable-repos"),
		actAptUpgrade("update-index"),
		// ubuntuPackages includes ubuntu-restricted-extras (codecs), which
		// pulls in the Microsoft fonts — hence the EULA.
		actAptInstall("install-essentials", "Instalar pacotes essenciais e codecs",
			"Codecs restritos (ubuntu-restricted-extras), GNOME Tweaks, ferramentas de compilação, Timeshift e utilitários de linha de comando comuns.",
			"update-index", true, ubuntuPackages...),
		actSysctl(),
		actSwap(),
		actTRIM(),
		actAptVAAPIIntel("enable-repos"),
		actAptVAAPIAMD("enable-repos"),
		actFlatpakEssentials("Configurar Flatpak", flatpakSetupDescription),
		actUbuntuDrivers("enable-repos"),
	}
}

var ubuntu2404Profile = Profile{ID: "ubuntu-2404", Label: "Ubuntu 24.04", Actions: ubuntuActions()}
var ubuntu2604Profile = Profile{ID: "ubuntu-2604", Label: "Ubuntu 26.04", Actions: ubuntuActions()}

// ubuntuBudgiePackages is ubuntuPackages without the GNOME-only entries:
// gnome-tweaks, and gnome-software-plugin-flatpak, which would install
// GNOME Software alongside Ubuntu Budgie's own software store.
var ubuntuBudgiePackages = slices.DeleteFunc(slices.Clone(ubuntuPackages), func(pkg string) bool {
	return pkg == "gnome-tweaks" || pkg == "gnome-software-plugin-flatpak"
})

// ubuntuBudgieActions is the checklist for Ubuntu Budgie: ubuntuActions
// with ubuntuBudgiePackages, and flatpakApps as the Flathub apps of the
// Flatpak step.
func ubuntuBudgieActions(flatpakApps []string, flatpakDescription string) []Action {
	return []Action{
		actEnableUbuntuComponents(),
		actUpdateIndex("enable-repos"),
		actAptUpgrade("update-index"),
		actAptInstall("install-essentials", "Instalar pacotes essenciais e codecs",
			"Codecs restritos (ubuntu-restricted-extras), ferramentas de compilação, Timeshift e utilitários de linha de comando comuns.",
			"update-index", true, ubuntuBudgiePackages...),
		actSysctl(),
		actSwap(),
		actTRIM(),
		actAptVAAPIIntel("enable-repos"),
		actAptVAAPIAMD("enable-repos"),
		actFlatpakApps("Configurar Flatpak", flatpakDescription, flatpakApps...),
		actUbuntuDrivers("enable-repos"),
	}
}

var ubuntuBudgie2404Profile = Profile{ID: "ubuntu-budgie-2404", Label: "Ubuntu Budgie 24.04",
	Actions: ubuntuBudgieActions(flatpakEssentialApps, flatpakSetupDescription)}

// Ubuntu Budgie 26.04 ships VLC as a distro package, so the Flatpak step
// installs only the web-app manager.
var ubuntuBudgie2604Profile = Profile{ID: "ubuntu-budgie-2604", Label: "Ubuntu Budgie 26.04",
	Actions: ubuntuBudgieActions(flatpakWebAppsOnly, flatpakWebAppsDescription)}
