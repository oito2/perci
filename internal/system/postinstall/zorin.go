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

var zorinPackages = []string{
	"libavcodec-extra",
	"ffmpeg",
	"gstreamer1.0-plugins-bad",
	"gstreamer1.0-plugins-ugly",
	"gstreamer1.0-libav",
	"build-essential",
	"git",
	"curl",
	"wget",
	"htop",
	"gdebi",
	"libfuse2t64",
	"unrar",
	"unzip",
	"ntfs-3g",
	"7zip",
	"tree",
	"jq",
	"plocate",
	"net-tools",
	"software-properties-common",
}

// zorinActions is the checklist shared by ZorinOS Core and Lite — the steps
// are desktop-neutral.
func zorinActions() []Action {
	return []Action{
		actEnableUbuntuComponents(),
		actUpdateIndex("enable-repos"),
		actAptUpgrade("update-index"),
		actAptInstall("install-essentials", "Instalar pacotes essenciais",
			"Codecs de mídia, ferramentas de compilação, gerenciador de arquivos comprimidos e utilitários de linha de comando comuns.",
			"update-index", false, zorinPackages...),
		actAptInstall("install-restricted-extras", "Instalar mídias restritas e fontes Microsoft",
			"Instala o pacote zorin-os-restricted-extras (aceita a licença das fontes Microsoft automaticamente).",
			"enable-repos", true, "zorin-os-restricted-extras"),
		actSysctl(),
		actSwap(),
		actTRIM(),
		actAptVAAPIIntel("enable-repos"),
		actAptVAAPIAMD("enable-repos"),
	}
}

var zorinCoreProfile = Profile{ID: "zorin-core", Label: "ZorinOS 18.1", Actions: zorinActions()}
var zorinLiteProfile = Profile{ID: "zorin-lite", Label: "ZorinOS 18.1 Lite", Actions: zorinActions()}
