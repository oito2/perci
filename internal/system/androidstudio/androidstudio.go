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

// Package androidstudio manages the Android Studio IDE. Install takes a
// local .tar.gz path chosen by the user; there is no update action, and
// reinstalling with a newer tarball replaces the previous install.
//
// Installs into /opt/android-studio owned by the current user: root only
// prepares the empty directory, and the extraction itself runs
// unprivileged (see Install).
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

// desktopFile is the .desktop file Android Studio's own "Create Desktop
// Entry" step writes. It is not created here; Uninstall removes it.
func desktopFile(home string) string {
	return filepath.Join(home, ".local", "share", "applications", "jetbrains-studio.desktop")
}

// Installed reports whether Android Studio is currently installed, using a
// plain os.Stat on the install directory.
func Installed() bool {
	info, err := os.Stat(installDir)
	return err == nil && info.IsDir()
}

// Install extracts tarballPath into /opt/android-studio, owned by the
// current user, then launches the first-run setup wizard (studio.sh)
// detached: it is not awaited, so the step finishes right away. Safe to
// re-run with a newer tarball.
//
// Root never touches the tarball's contents: one privileged batch moves any
// previous install aside to installDir+".old", creates an empty installDir
// and hands it to the user; the extraction then runs as the user. The
// archive's symlinks, setuid bits and owners therefore take effect only
// with the user's own privileges.
func Install(ctx context.Context, exe *executor.Executor, stdout io.Writer, tarballPath string) error {
	if _, err := os.Stat(tarballPath); err != nil {
		return fmt.Errorf("arquivo não encontrado: %s", tarballPath)
	}

	user := executor.CurrentUser()
	if user == "" {
		return fmt.Errorf("não foi possível determinar o usuário atual")
	}

	// Entries must all live under android-studio/, which
	// --strip-components=1 below strips; any other tarball is rejected.
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

	// The previous install is owned by the user, so removing it needs no
	// second password prompt. Skipped in DryRun.
	if exe.DryRun {
		ui.Info(stdout, "[dry-run] remover "+oldDir)
	} else if err := os.RemoveAll(oldDir); err != nil {
		ui.Warning(stdout, "Não foi possível remover a instalação anterior em "+oldDir+": "+err.Error())
	}

	studioSh := filepath.Join(installDir, "bin", "studio.sh")
	ui.Info(stdout, "Abrindo o assistente de configuração do Android Studio (janela própria)...")
	// setsid + & detaches studio.sh from perci's own process (its own
	// session, stdin/stdout/stderr closed) so exe.Run returns as soon as
	// the process launches.
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
// segment. The archive is read in-process (archive/tar), read-only, so it
// also runs under DryRun.
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
// present). User config/cache (~/.config/Google/AndroidStudio*, etc.) and
// the Android SDK/emulators (~/Android/Sdk, ~/.android) are left in
// place.
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
