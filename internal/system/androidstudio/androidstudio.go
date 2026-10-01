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

// Package androidstudio manages the Android Studio IDE — a GUI screen
// ("Desenvolvimento :: IDE: Android Studio") using the "single app
// install/uninstall" pattern with a local file picker: the user downloads
// the .tar.gz manually from https://developer.android.com/studio and
// points Perci at it. There is no "Update" button — Android Studio updates
// itself (Configure/three-dot menu → Check for Updates), a deliberate
// choice made with the user.
//
// Installs into /opt/android-studio owned by the current user — the same
// end result as Google's recommended `sudo tar -xvzf <tarball> -C /opt/` +
// `sudo chown -R $USER:$USER /opt/android-studio` (the commands originally
// specified by the user), but root only prepares the empty directory: the
// extraction itself runs unprivileged (see Install).
package androidstudio

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/oito2/perci/internal/executor"
	"github.com/oito2/perci/internal/ui"
)

const installDir = "/opt/android-studio"

// desktopFile is where Android Studio's own "Create Desktop Entry" wizard
// step writes its .desktop file (JetBrains Toolbox convention) — perci
// doesn't create it (the tarball has no bundled .desktop/icon to install
// non-interactively, unlike Antigravity's app.asar), only removes it on
// Uninstall; the user is instructed (Observações da tela) to run that wizard
// step once after installing.
func desktopFile(home string) string {
	return filepath.Join(home, ".local", "share", "applications", "jetbrains-studio.desktop")
}

// Installed reports whether Android Studio is currently installed. Checked
// via a plain os.Stat, not exe.Output(ctx, ..., "test", "-d", installDir)
// like internal/system/linuxtoys/megasync do — deliberately: this is a
// read-only local filesystem check with no subprocess/DryRun concern to
// route through the Executor for.
func Installed() bool {
	info, err := os.Stat(installDir)
	return err == nil && info.IsDir()
}

// Install extracts tarballPath into /opt/android-studio, owned by the
// current user, then launches the first-run setup wizard (studio.sh)
// detached — it's a GUI app in its own right, launched, not awaited, so the
// "Executando" step here finishes right away instead of blocking on the
// wizard window. Safe to re-run with a newer tarball (the user's own path
// to "update" without a dedicated button — see package doc).
//
// Root never touches the tarball's contents: one privileged batch moves any
// previous install aside to installDir+".old", creates an empty installDir
// and hands it to the user; the extraction then runs as the user. The
// tarball is a file the user controls, and extracting it as root would
// trust it past the password dialog (it could be swapped meanwhile), follow
// symlink entries anywhere on the system and keep setuid bits and owners
// from the archive. As the user, the worst a crafted archive can do is what
// the user could already do.
func Install(ctx context.Context, exe *executor.Executor, stdout io.Writer, tarballPath string) error {
	if _, err := os.Stat(tarballPath); err != nil {
		return fmt.Errorf("arquivo não encontrado: %s", tarballPath)
	}

	user := executor.CurrentUser()
	if user == "" {
		return fmt.Errorf("não foi possível determinar o usuário atual")
	}

	// Entries must all live under android-studio/ — that's what
	// --strip-components=1 below strips, so anything else is not an
	// Android Studio tarball.
	if err := validateTarballEntries(tarballPath); err != nil {
		return err
	}

	oldDir := installDir + ".old"
	prepare := `set -e
if [ -e "$1" ]; then rm -rf -- "$2"; mv -- "$1" "$2"; fi
mkdir -m 0755 -- "$1"
chown -- "$3:" "$1"`
	steps := []executor.PrivilegedStep{{
		Announce: "Preparando " + installDir + " para " + user + "...",
		Name:     "sh",
		Args:     []string{"-c", prepare, "sh", installDir, oldDir, user},
	}}
	if err := exe.RunSudoSequence(ctx, executor.Options{Stdout: stdout, Stderr: stdout}, steps); err != nil {
		return fmt.Errorf("preparar %s: %w", installDir, err)
	}

	ui.Info(stdout, "Extraindo "+tarballPath+" em "+installDir+"...")
	if err := exe.Run(ctx, executor.Options{Stdout: stdout, Stderr: stdout},
		"tar", "-xzf", tarballPath, "-C", installDir, "--strip-components=1", "--no-same-owner",
	); err != nil {
		if _, statErr := os.Stat(oldDir); statErr == nil {
			ui.Warning(stdout, "A instalação anterior foi preservada em "+oldDir+".")
		}
		return fmt.Errorf("extrair: %w", err)
	}

	// The previous install was already owned by the user (every install
	// chowns it), so removing it needs no second password prompt. Skipped
	// in DryRun, which must not change the system.
	if exe.DryRun {
		ui.Info(stdout, "[dry-run] remover "+oldDir)
	} else if err := os.RemoveAll(oldDir); err != nil {
		ui.Warning(stdout, "Não foi possível remover a instalação anterior em "+oldDir+": "+err.Error())
	}

	studioSh := filepath.Join(installDir, "bin", "studio.sh")
	ui.Info(stdout, "Abrindo o assistente de configuração do Android Studio (janela própria)...")
	// setsid + & detaches studio.sh from perci's own process (its own
	// session, stdin/stdout/stderr closed) so exe.Run returns as soon as
	// the process launches, without waiting for the wizard window to
	// close — same approach used for every other third-party GUI app
	// launched from here.
	launchScript := "setsid " + executor.ShellQuote(studioSh) + " >/dev/null 2>&1 < /dev/null &"
	if err := exe.Run(ctx, executor.Options{Stdout: stdout, Stderr: stdout}, "bash", "-c", launchScript); err != nil {
		ui.Warning(stdout, "Não foi possível abrir o assistente automaticamente: "+err.Error())
		ui.Warning(stdout, "Abra manualmente: "+studioSh)
	}

	ui.Success(stdout, "Android Studio instalado em "+installDir+".")
	return nil
}

// validateTarballEntries lists tarballPath's contents and rejects it unless
// every entry is a relative path under android-studio/ with no ".."
// segment. Extraction runs unprivileged, so this isn't the security
// boundary — it just refuses anything that isn't an Android Studio tarball
// before the previous install is moved aside. Read in-process (archive/tar):
// read-only, so it also runs under DryRun.
func validateTarballEntries(tarballPath string) error {
	f, err := os.Open(tarballPath)
	if err != nil {
		return fmt.Errorf("abrir tarball: %w", err)
	}
	defer func() { _ = f.Close() }()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("o arquivo não é um .tar.gz válido: %w", err)
	}
	defer func() { _ = gz.Close() }()
	var names []string
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("listar conteúdo do tarball: %w", err)
		}
		names = append(names, hdr.Name)
	}
	return checkTarballEntries(strings.Join(names, "\n"))
}

// checkTarballEntries is validateTarballEntries' pure part (`tar -t` output in).
func checkTarballEntries(listing string) error {
	found := false
	for _, entry := range strings.Split(listing, "\n") {
		if entry == "" {
			continue
		}
		found = true
		if strings.HasPrefix(entry, "/") || !strings.HasPrefix(entry, "android-studio/") || slices.Contains(strings.Split(entry, "/"), "..") {
			return fmt.Errorf("o arquivo não parece um tarball do Android Studio (entrada %q)", entry)
		}
	}
	if !found {
		return fmt.Errorf("tarball vazio")
	}
	return nil
}

// Uninstall removes the installation directory and the desktop entry (when
// present — created by the user via "Create Desktop Entry", see Install's
// doc comment). Config/cache (~/.config/Google/AndroidStudio*, etc.) and the
// Android SDK/emulators (~/Android/Sdk, ~/.android) are deliberately left in
// place — marked "(Opcional)" by the user, not part of the default flow.
func Uninstall(ctx context.Context, exe *executor.Executor, stdout io.Writer) error {
	sudo := executor.Options{RequiresSudo: true, Stdout: stdout, Stderr: stdout}

	ui.Info(stdout, "Removendo "+installDir+"...")
	if err := exe.Run(ctx, sudo, "rm", "-rf", "--", installDir, installDir+".old"); err != nil {
		return fmt.Errorf("remover %s: %w", installDir, err)
	}

	if home, err := os.UserHomeDir(); err == nil {
		_ = os.Remove(desktopFile(home))
	}

	ui.Success(stdout, "Android Studio desinstalado.")
	return nil
}
