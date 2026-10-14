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

package appstack

// mkcert's local CA in the browsers installed as Flatpaks. mkcert -install
// registers it in the system store, in ~/.pki/nssdb (Chrome/Chromium) and
// in the profiles under ~/.mozilla/firefox, but a Flatpak browser keeps
// its profile under ~/.var/app/<app id>, and the Firefox family (Firefox,
// Zen, LibreWolf, Floorp…) only trusts its own certificate database.

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/oito2/perci/internal/executor"
	"github.com/oito2/perci/internal/ui"
)

// mkcertCANickname is the name the CA gets in each certificate database.
const mkcertCANickname = "mkcert development CA (perci)"

// flatpakNSSPatterns match, under ~/.var/app, the NSS certificate
// databases of Flatpak browsers: the shared NSS database of the Chromium
// family, the Firefox profiles, and the profiles of Firefox derivatives
// kept in a dot folder of their own (~/.zen/<profile>, ~/.librewolf/…).
var flatpakNSSPatterns = []string{
	"*/.pki/nssdb/cert9.db",
	"*/.mozilla/firefox/*/cert9.db",
	"*/.*/*/cert9.db",
}

// flatpakNSSDirs returns the folders holding an NSS certificate database
// under home/.var/app, sorted and without duplicates.
func flatpakNSSDirs(home string) []string {
	base := filepath.Join(home, ".var", "app")
	var dirs []string
	for _, pattern := range flatpakNSSPatterns {
		matches, _ := filepath.Glob(filepath.Join(base, pattern))
		for _, m := range matches {
			dirs = append(dirs, filepath.Dir(m))
		}
	}
	slices.Sort(dirs)
	return slices.Compact(dirs)
}

// flatpakAppID returns the Flatpak app id of an NSS folder under
// home/.var/app.
func flatpakAppID(home, dir string) string {
	rel, err := filepath.Rel(filepath.Join(home, ".var", "app"), dir)
	if err != nil {
		return dir
	}
	id, _, _ := strings.Cut(filepath.ToSlash(rel), "/")
	return id
}

// trustCAInFlatpakBrowsers adds mkcert's root CA, trusted for websites, to
// every Flatpak browser certificate database that doesn't have it yet. It
// only reports problems as warnings: the stack works without it, and the
// browser can still be fixed by importing the CA by hand.
func trustCAInFlatpakBrowsers(ctx context.Context, exe *executor.Executor, stdout io.Writer) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	dirs := flatpakNSSDirs(home)
	if len(dirs) == 0 {
		return
	}
	if !exe.CommandAvailable(ctx, "certutil") {
		ui.Warning(stdout, "Há navegadores Flatpak, mas o certutil não está instalado: reinstale o Mkcert em Pré-requisitos (ele traz o certutil) e recrie o Contêiner Nginx para que esses navegadores confiem nos sites *.localhost.")
		return
	}
	out, err := exe.Output(ctx, executor.Options{}, "mkcert", "-CAROOT")
	if err != nil {
		ui.Warning(stdout, "Não foi possível localizar a autoridade certificadora do mkcert: "+err.Error())
		return
	}
	rootCA := filepath.Join(strings.TrimSpace(out), "rootCA.pem")
	if _, err := os.Stat(rootCA); err != nil && !exe.DryRun {
		ui.Warning(stdout, "Autoridade certificadora do mkcert não encontrada em "+rootCA)
		return
	}

	var added []string
	for _, dir := range dirs {
		db := "sql:" + dir
		if _, err := exe.Output(ctx, executor.Options{}, "certutil", "-L", "-d", db, "-n", mkcertCANickname); err == nil && !exe.DryRun {
			continue
		}
		if err := exe.Run(ctx, executor.Options{Stdout: stdout, Stderr: stdout}, "certutil", "-A", "-d", db, "-t", "C,,", "-n", mkcertCANickname, "-i", rootCA); err != nil {
			ui.Warning(stdout, "Não foi possível registrar a autoridade certificadora em "+dir+": "+err.Error())
			continue
		}
		added = append(added, flatpakAppID(home, dir))
	}
	if len(added) > 0 {
		slices.Sort(added)
		ui.Info(stdout, "Autoridade certificadora do mkcert registrada nos navegadores Flatpak: "+strings.Join(slices.Compact(added), ", ")+". Reinicie esses navegadores para que confiem nos sites *.localhost.")
	}
}
