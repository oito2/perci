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

// ubuntuActions is the checklist for both Ubuntu 24.04 and 26.04 — they
// have no differences that change these apt commands (sudo-rs, dracut and
// GNOME 50/Wayland-only are internal OS changes), kept as two Profiles (one
// per version) as requested. Built from the shared blocks in debian.go.
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
