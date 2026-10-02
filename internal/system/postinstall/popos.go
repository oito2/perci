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

// poposPackages holds the same general-purpose utilities as
// ubuntuPackages/zorinPackages, minus the GNOME-specific entries
// (gnome-tweaks, gnome-software-plugin-flatpak) and minus
// ubuntu-drivers-common (Pop!_OS manages drivers through system76-driver).
//
// Pop!_OS 24.04 is based on Ubuntu 24.04 (apt/deb) and ships with
// main/restricted/universe/multiverse already enabled, so it has no
// "enable universe/multiverse" step.
var poposPackages = []string{
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
	"timeshift",
}

// poposProfile deliberately does NOT include:
//   - "enable repositories": universe/multiverse are already enabled.
//   - "configure swapfile": Pop!_OS already uses zram by default.
//   - "detect additional drivers" (ubuntu-drivers autoinstall): drivers are
//     managed by system76-driver/system76-driver-nvidia.
var poposProfile = Profile{
	ID:    "popos-cosmic",
	Label: "Pop!_OS 24.04 COSMIC",
	Actions: []Action{
		actAptUpgrade(""),
		// Same ubuntu-restricted-extras as Ubuntu/Zorin, so the same EULA.
		actAptInstall("install-codecs", "Instalar codecs multimídia",
			"Instala ubuntu-restricted-extras e gstreamer1.0-plugins-bad (recomendação oficial da System76 para Pop!_OS).",
			"", true, "ubuntu-restricted-extras", "gstreamer1.0-plugins-bad"),
		actAptInstall("install-essentials", "Instalar pacotes essenciais",
			"Ferramentas de compilação, Timeshift e utilitários de linha de comando comuns.",
			"", false, poposPackages...),
		actSysctl(),
		actTRIM(),
		actAptVAAPIIntel(""),
		actAptVAAPIAMD(""),
		actFlatpakEssentials("Instalar Flatpaks essenciais",
			"Instala VLC e um app de atalhos web via Flathub (já vem pré-configurado no Pop!_OS)."),
	},
}
