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

var mintPackages = []string{
	"mint-meta-codecs",
	"ubuntu-drivers-common",
	"libavcodec-extra",
	"ffmpeg",
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
	"timeshift",
}

// mintActions is the checklist shared by Mint Cinnamon and Mint XFCE — the
// steps are desktop-neutral. Each choice that could have been a prompt
// (e.g. Intel/AMD/none for VA-API) is its own checklist item instead.
func mintActions() []Action {
	return []Action{
		actEnableUbuntuComponents(),
		actUpdateIndex("enable-repos"),
		actAptUpgrade("update-index"),
		actAptInstall("install-essentials", "Instalar pacotes essenciais",
			"Codecs de mídia, drivers Ubuntu, ferramentas de compilação, gerenciador de arquivos comprimidos, Timeshift e utilitários de linha de comando comuns.",
			"update-index", false, mintPackages...),
		actAptInstall("install-ms-fonts", "Instalar fontes Microsoft",
			"Instala o pacote ttf-mscorefonts-installer (aceita a licença automaticamente).",
			"enable-repos", true, "ttf-mscorefonts-installer"),
		actFlatpakEssentials("Configurar Flatpak", flatpakSetupDescription),
		actSysctl(),
		actSwap(),
		actTRIM(),
		actAptVAAPIIntel("enable-repos"),
		actAptVAAPIAMD("enable-repos"),
		actUbuntuDrivers("enable-repos"),
	}
}

var mintCinnamonProfile = Profile{ID: "mint-cinnamon", Label: "Linux Mint Cinnamon", Actions: mintActions()}
var mintXFCEProfile = Profile{ID: "mint-xfce", Label: "Linux Mint XFCE", Actions: mintActions()}
