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

package apps

// App describes a Flatpak application managed by perci.gnl.
type App struct {
	Name            string
	FlatID          string
	FlatpakOverride []string // extra args passed to "flatpak override <FlatID> <args...>" after install
}

// Catalogue lists all Flatpak apps available for installation.
// To add a new app, append a new App entry here.
var Catalogue = []App{
	{Name: "Loupe - Visualizador de Fotos", FlatID: "org.gnome.Loupe"},
	{Name: "Celluloid - Visualizador de Vídeo", FlatID: "io.github.celluloid_player.Celluloid"},
	{Name: "VLC - Visualizador de Vídeo", FlatID: "org.videolan.VLC"},
	{Name: "Vinyl - Player de Música", FlatID: "page.codeberg.M23Snezhok.Vinyl"},
	{Name: "Calculator - Calculadora", FlatID: "org.gnome.Calculator"},
	{Name: "Resources - Monitor do Sistema", FlatID: "net.nokyan.Resources"},
	{Name: "Gradia - Captura de Tela", FlatID: "be.alexandervanhee.gradia"},
	{Name: "Apostrophe - Markdown", FlatID: "org.gnome.gitlab.somas.Apostrophe"},
	{Name: "Eyedropper - Conta Gotas", FlatID: "com.github.finefindus.eyedropper"},
	{Name: "Gear Lever - AppImages", FlatID: "it.mijorus.gearlever"},
	{Name: "AppImage Pool", FlatID: "io.github.prateekmedia.appimagepool"},
	{Name: "Web Apps", FlatID: "net.codelogistics.webapps"},
	{Name: "Flatseal - Gerenciar Flatpak", FlatID: "com.github.tchx84.Flatseal"},
	{Name: "Bazaar", FlatID: "io.github.kolunmi.Bazaar"},
	{Name: "Zen Browser", FlatID: "app.zen_browser.zen"},
	{Name: "Firefox", FlatID: "org.mozilla.firefox"},
	{Name: "Google Chrome", FlatID: "com.google.Chrome"},
	{Name: "Chromium", FlatID: "org.chromium.Chromium"},
	{Name: "Vivaldi", FlatID: "com.vivaldi.Vivaldi"},
	{Name: "FileZilla", FlatID: "org.filezillaproject.Filezilla"},
	{Name: "Inkscape", FlatID: "org.inkscape.Inkscape"},
	{Name: "Krita", FlatID: "org.kde.krita"},
	{Name: "Penpot", FlatID: "com.authormore.penpotdesktop"},
	{Name: "LibreOffice", FlatID: "org.libreoffice.LibreOffice"},
	{Name: "OnlyOffice", FlatID: "org.onlyoffice.desktopeditors"},
	{Name: "AnyDesk", FlatID: "com.anydesk.Anydesk"},
	{Name: "DBeaver Community", FlatID: "io.dbeaver.DBeaverCommunity", FlatpakOverride: []string{"--share=network"}},
	{Name: "Beekeeper Studio", FlatID: "io.beekeeperstudio.Studio"},
	{Name: "Meld - File Compare", FlatID: "org.gnome.meld"},
	{Name: "Minecraft", FlatID: "io.mrarm.mcpelauncher"},
	{Name: "Minecraft Java", FlatID: "org.prismlauncher.PrismLauncher"},
	{Name: "Ente Auth - Segurança", FlatID: "io.ente.auth"},
	{Name: "Font Downloader", FlatID: "org.gustavoperedo.FontDownloader"},
	{Name: "FreeTube - YouTube", FlatID: "io.freetubeapp.FreeTube"},
	{Name: "Tomatillo - Pomodoro", FlatID: "io.github.diegopvlk.Tomatillo"},
	{Name: "Extension Manager (GNOME)", FlatID: "com.mattjakeman.ExtensionManager"},
}
