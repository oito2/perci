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

package flutter

import (
	"context"
	"crypto/sha1"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/oito2/perci/internal/dev/localbin"
	"github.com/oito2/perci/internal/distro"
	"github.com/oito2/perci/internal/executor"
	"github.com/oito2/perci/internal/fsutil"
	"github.com/oito2/perci/internal/shellrc"
	"github.com/oito2/perci/internal/ui"
)

const (
	flutterRepo     = "https://github.com/flutter/flutter.git"
	pathEntry       = `export PATH="$PATH:$HOME/development/flutter/bin"`
	chromeFlatpakID = "com.google.Chrome"
)

// Variables only so tests can point them at a local server.
var (
	// androidCmdlineToolsURL is pinned to a specific build (cmdline-tools 13.0)
	androidCmdlineToolsURL = "https://dl.google.com/android/repository/commandlinetools-linux-11076708_latest.zip"

	// androidRepoManifestURL is Google's own package index — the same one
	// `sdkmanager`/Android Studio consult — used only to fetch the SHA-1 this
	// package independently pins androidCmdlineToolsURL's download against
	// (see androidCmdlineToolsChecksum), so a compromised CDN/MITM serving a
	// tampered zip under that exact URL is still caught before extraction.
	androidRepoManifestURL = "https://dl.google.com/android/repository/repository2-3.xml"
)

// IsInstalled returns true if flutter is installed at the expected path.
func IsInstalled(flutterBin string) bool {
	info, err := os.Stat(flutterBin)
	return err == nil && !info.IsDir()
}

// Uninstall removes the Flutter checkout and its PATH entry in ~/.bashrc.
// Leaves the Android SDK and the Chrome wrapper in place — they're shared
// toolchain pieces, not specific to this Flutter installation.
//
// The checkout is removed through exe (not os.RemoveAll), so DryRun
// really leaves it in place.
func Uninstall(ctx context.Context, exe *executor.Executor, stdout io.Writer, home, flutterDir string) error {
	ui.Info(stdout, "Removendo Flutter...")
	if err := exe.Run(ctx, executor.Options{Stdout: stdout, Stderr: stdout}, "rm", "-rf", "--", flutterDir); err != nil {
		ui.Err(stdout, "Falha ao remover Flutter: "+err.Error())
		return err
	}
	RemovePathFromBashrc(stdout, home, pathEntry, "# Flutter")
	ui.Success(stdout, "Flutter removido com sucesso.")
	return nil
}

// RemovePathFromBashrc undoes an ensurePathInBashrc-style append, removing
// the comment and export lines previously added for entry. Checks both
// ~/.bashrc and the current shell's RC file.
func RemovePathFromBashrc(stdout io.Writer, home, entry, comment string) {
	candidates := shellrc.Dedup([]string{filepath.Join(home, ".bashrc"), shellrc.File(home)})
	for _, res := range shellrc.RemoveEntry(candidates, comment, entry) {
		if res.Err != nil {
			ui.Warning(stdout, "Não foi possível atualizar "+res.Path+": "+res.Err.Error())
			continue
		}
		ui.Info(stdout, "PATH removido de "+res.Path)
	}
}

// InstallPrereqs installs the native libraries Flutter needs to run on Linux
// — one privileged batch (distro.InstallPkgs), so one password prompt.
func InstallPrereqs(ctx context.Context, exe *executor.Executor, stdout io.Writer) error {
	return installPrereqsFor(ctx, exe, stdout, distro.Detect())
}

func installPrereqsFor(ctx context.Context, exe *executor.Executor, stdout io.Writer, family string) error {
	ui.Info(stdout, "Instalando pré-requisitos do Flutter...")
	switch family {
	case distro.Debian:
		return distro.InstallPkgs(ctx, exe, stdout, family,
			"curl", "git", "unzip", "xz-utils", "zip", "libglu1-mesa",
			"clang", "cmake", "ninja-build", "pkg-config", "libgtk-3-dev")
	case distro.Fedora:
		return distro.InstallPkgs(ctx, exe, stdout, family,
			// Fedora names (checked against Fedora 44): xz, not Debian's
			// xz-utils — one unknown name fails the whole transaction — and
			// mesa-libGLU, the counterpart of Debian's libglu1-mesa.
			"curl", "git", "unzip", "xz", "zip", "mesa-libGLU",
			"clang", "cmake", "ninja-build", "pkgconf-pkg-config", "gtk3-devel")
	default:
		ui.Warning(stdout, "Distribuição não suportada para instalação automática de pré-requisitos. Instale manualmente.")
		return nil
	}
}

// EnsureAndroidCmdlineTools installs the Android SDK command-line tools.
func EnsureAndroidCmdlineTools(ctx context.Context, exe *executor.Executor, stdout io.Writer, home, flutterBin string) error {
	androidHome := filepath.Join(home, "Android", "Sdk")
	cmdlineToolsDir := filepath.Join(androidHome, "cmdline-tools")
	latestDir := filepath.Join(cmdlineToolsDir, "latest")
	sdkmanagerBin := filepath.Join(latestDir, "bin", "sdkmanager")

	if IsInstalled(sdkmanagerBin) {
		ui.Info(stdout, "Android cmdline-tools já instalado em: "+androidHome)
		return exe.Run(ctx, executor.Options{Stdout: stdout, Stderr: stdout}, flutterBin, "config", "--android-sdk", androidHome)
	}

	if err := ensureJava(ctx, exe, stdout, distro.Detect()); err != nil {
		return fmt.Errorf("instalar Java: %w", err)
	}

	// tmp is created inside cmdlineToolsDir (not the system temp dir) so it
	// shares a filesystem with latestDir below — os.Rename between them
	// would otherwise fail with EXDEV whenever /tmp is a separate mount
	// (e.g. tmpfs), which is the common case.
	if err := os.MkdirAll(cmdlineToolsDir, 0o755); err != nil {
		return fmt.Errorf("criar diretório do Android SDK: %w", err)
	}
	tmp, err := os.MkdirTemp(cmdlineToolsDir, "staging-*")
	if err != nil {
		return fmt.Errorf("criar diretório temporário: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmp) }()

	zipPath := filepath.Join(tmp, "cmdline-tools.zip")
	ui.Info(stdout, "Baixando Android cmdline-tools...")
	if err := exe.Run(ctx,
		executor.Options{Stdout: stdout, Stderr: stdout},
		"curl", "-fSL", "--connect-timeout", "10", "--max-time", "600", "--progress-bar", "-o", zipPath, androidCmdlineToolsURL,
	); err != nil {
		return fmt.Errorf("baixar cmdline-tools: %w", err)
	}

	ui.Info(stdout, "Verificando integridade do cmdline-tools...")
	checksum, err := androidCmdlineToolsChecksum(ctx)
	if err != nil {
		return fmt.Errorf("obter checksum do Android SDK: %w", err)
	}
	if err := fsutil.VerifyFile(zipPath, sha1.New(), checksum); err != nil {
		return fmt.Errorf("verificação de integridade do cmdline-tools falhou: %w", err)
	}

	ui.Info(stdout, "Extraindo cmdline-tools...")
	if err := exe.Run(ctx, executor.Options{Stdout: stdout, Stderr: stdout}, "unzip", "-q", "-o", zipPath, "-d", tmp); err != nil {
		return fmt.Errorf("extrair cmdline-tools: %w", err)
	}

	_ = os.RemoveAll(latestDir)
	if err := os.Rename(filepath.Join(tmp, "cmdline-tools"), latestDir); err != nil {
		return fmt.Errorf("mover cmdline-tools: %w", err)
	}

	EnsureAndroidEnvInBashrc(stdout, home)

	ui.Info(stdout, "Aceitando licenças do Android SDK...")
	licenseScript := "yes | " + executor.ShellQuote(sdkmanagerBin) + " --licenses >/dev/null"
	if err := exe.Run(ctx, executor.Options{Stdout: stdout, Stderr: stdout}, "bash", "-c", licenseScript); err != nil {
		return fmt.Errorf("aceitar licenças: %w", err)
	}

	ui.Info(stdout, "Instalando platform-tools...")
	if err := exe.Run(ctx, executor.Options{Stdout: stdout, Stderr: stdout}, sdkmanagerBin, "platform-tools"); err != nil {
		return fmt.Errorf("instalar platform-tools: %w", err)
	}

	if err := exe.Run(ctx, executor.Options{Stdout: stdout, Stderr: stdout}, flutterBin, "config", "--android-sdk", androidHome); err != nil {
		return fmt.Errorf("configurar android-sdk no flutter: %w", err)
	}

	ui.Success(stdout, "Android cmdline-tools instalado em: "+androidHome)
	return nil
}

// androidCmdlineToolsChecksum fetches Google's package repository manifest
// (the same one sdkmanager/Android Studio consult) and returns the SHA-1
// Google publishes for the linux archive matching androidCmdlineToolsURL —
// an independent source from the download itself, so a tampered zip served
// under that URL by a compromised CDN/MITM doesn't also need to have
// tampered this separate manifest to go undetected.
func androidCmdlineToolsChecksum(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, androidRepoManifestURL, nil)
	if err != nil {
		return "", fmt.Errorf("criar requisição: %w", err)
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("consultar manifesto do Android SDK: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("manifesto do Android SDK retornou status %d", resp.StatusCode)
	}

	var manifest struct {
		Packages []struct {
			Archives struct {
				Archive []struct {
					Complete struct {
						Checksum struct {
							Type  string `xml:"type,attr"`
							Value string `xml:",chardata"`
						} `xml:"checksum"`
						URL string `xml:"url"`
					} `xml:"complete"`
					HostOS string `xml:"host-os"`
				} `xml:"archive"`
			} `xml:"archives"`
		} `xml:"remotePackage"`
	}
	if err := xml.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&manifest); err != nil {
		return "", fmt.Errorf("decodificar manifesto: %w", err)
	}

	wantURL := filepath.Base(androidCmdlineToolsURL)
	for _, pkg := range manifest.Packages {
		for _, a := range pkg.Archives.Archive {
			if a.HostOS != "linux" || a.Complete.URL != wantURL {
				continue
			}
			if a.Complete.Checksum.Type != "sha1" {
				return "", fmt.Errorf("tipo de checksum inesperado no manifesto: %q", a.Complete.Checksum.Type)
			}
			return a.Complete.Checksum.Value, nil
		}
	}
	return "", fmt.Errorf("checksum não encontrado no manifesto para %s", wantURL)
}

// temurinRepo is Adoptium's RPM repository, as documented by Adoptium
// (adoptium.net/installation/linux), for the Eclipse Temurin 21 JDK.
var temurinRepo = distro.SignedRepo{
	KeyURL:      "https://packages.adoptium.net/artifactory/api/gpg/key/public",
	DnfRepoPath: "/etc/yum.repos.d/adoptium.repo",
	DnfRepoBody: "[Adoptium]\nname=Adoptium\nbaseurl=https://packages.adoptium.net/artifactory/rpm/fedora/$releasever/$basearch\nenabled=1\ngpgcheck=1\ngpgkey=https://packages.adoptium.net/artifactory/api/gpg/key/public\n",
	PkgName:     "temurin-21-jdk",
}

func ensureJava(ctx context.Context, exe *executor.Executor, stdout io.Writer, family string) error {
	if _, err := exe.Output(ctx, executor.Options{}, "java", "-version"); err == nil {
		return nil
	}
	return ensureJavaInstall(ctx, exe, stdout, family)
}

// ensureJavaInstall installs JDK 21 for family.
func ensureJavaInstall(ctx context.Context, exe *executor.Executor, stdout io.Writer, family string) error {
	opts := executor.Options{RequiresSudo: true, Stdout: stdout, Stderr: stdout}

	ui.Info(stdout, "Instalando Java 21 (necessário para o Android SDK)...")
	switch family {
	case distro.Debian:
		// openjdk-21-jdk, not default-jdk: on Ubuntu 26.04 default-jdk is
		// already JDK 25, which needs Gradle 9.1+ — current Flutter projects
		// still build Android with Gradle 8.x (checked 2026-09-29).
		return exe.Run(ctx, opts, "apt-get", "install", "-y", "--", "openjdk-21-jdk")
	case distro.Fedora:
		// Fedora 44 ships only JDK 25 (java-17/21-openjdk are gone), so JDK
		// 21 comes from Adoptium's official signed repository (decided with
		// the user on 2026-09-29; checked against Fedora 44).
		return distro.InstallFromSignedRepo(ctx, exe, stdout, family, temurinRepo)
	default:
		return fmt.Errorf("distribuição não suportada para instalação automática do Java — instale um JDK manualmente")
	}
}

// androidEnvLines are appended one by one: shellrc.AppendIfMissing only
// takes single-line entries (a guard against injecting extra commands into
// the rc file), and the three exports used to go in as one multi-line block
// that it always rejected — so ANDROID_HOME/ANDROID_SDK_ROOT and the SDK's
// PATH were never actually written.
var androidEnvLines = []string{
	`export ANDROID_HOME="$HOME/Android/Sdk"`,
	`export ANDROID_SDK_ROOT="$HOME/Android/Sdk"`,
	`export PATH="$PATH:$ANDROID_HOME/cmdline-tools/latest/bin:$ANDROID_HOME/platform-tools"`,
}

// EnsureAndroidEnvInBashrc adds ANDROID_HOME/ANDROID_SDK_ROOT and the SDK's PATH.
func EnsureAndroidEnvInBashrc(stdout io.Writer, home string) {
	rc := shellrc.File(home)
	added := false
	for i, line := range androidEnvLines {
		comment := ""
		if i == 0 {
			comment = "# Android SDK"
		}
		appended, err := shellrc.AppendIfMissing(rc, comment, line)
		if err != nil {
			ui.Warning(stdout, "Não foi possível atualizar "+rc+": "+err.Error())
			return
		}
		added = added || appended
	}
	if added {
		ui.Info(stdout, "Variáveis do Android SDK adicionadas a "+rc)
	}
}

// EnsureChromeWrapper makes "flutter doctor" find Chrome when it's only installed via Flatpak.
func EnsureChromeWrapper(ctx context.Context, exe *executor.Executor, stdout io.Writer, home string) error {
	wrapperPath := filepath.Join(home, ".local", "bin", "google-chrome")

	if _, err := os.Stat(wrapperPath); err == nil {
		return nil
	}

	for _, cmd := range []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser"} {
		if exe.CommandAvailable(ctx, cmd) {
			return nil
		}
	}

	if _, err := exe.Output(ctx, executor.Options{}, "flatpak", "info", chromeFlatpakID); err != nil {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(wrapperPath), 0o755); err != nil {
		return fmt.Errorf("criar ~/.local/bin: %w", err)
	}

	script := "#!/usr/bin/env bash\nexec flatpak run " + chromeFlatpakID + " \"$@\"\n"
	if err := os.WriteFile(wrapperPath, []byte(script), 0o755); err != nil {
		return fmt.Errorf("criar wrapper do Chrome: %w", err)
	}

	localbin.EnsureInPath(stdout)
	ensureChromeEnvInBashrc(stdout, home, wrapperPath)

	ui.Success(stdout, "Wrapper do Chrome (Flatpak) criado em: "+wrapperPath)
	return nil
}

func ensureChromeEnvInBashrc(stdout io.Writer, home, wrapperPath string) {
	rc := shellrc.File(home)
	// ShellQuote, not %q: a Go-quoted string still expands $(...) and
	// backticks once the shell reads the rc file.
	entry := "export CHROME_EXECUTABLE=" + executor.ShellQuote(wrapperPath)
	appended, err := shellrc.AppendIfMissing(rc, "# Chrome (Flatpak)", entry)
	if err != nil {
		ui.Warning(stdout, "Não foi possível atualizar "+rc+": "+err.Error())
		return
	}
	if appended {
		ui.Info(stdout, "CHROME_EXECUTABLE adicionado a "+rc)
	}
}

// CloneFlutter clones the stable channel into flutterDir.
func CloneFlutter(ctx context.Context, exe *executor.Executor, stdout io.Writer, home, flutterDir string) error {
	if err := os.MkdirAll(filepath.Join(home, "development"), 0o755); err != nil {
		return fmt.Errorf("criar diretório development: %w", err)
	}

	ui.Info(stdout, "Clonando "+flutterRepo+" (branch stable)...")
	if err := exe.Run(ctx,
		executor.Options{Stdout: stdout, Stderr: stdout},
		"git", "clone", "--branch", "stable", flutterRepo, flutterDir,
	); err != nil {
		return fmt.Errorf("git clone: %w", err)
	}
	return nil
}

// EnsurePathInBashrc adds flutter/bin to PATH.
func EnsurePathInBashrc(stdout io.Writer, home string) {
	rc := shellrc.File(home)
	appended, err := shellrc.AppendIfMissing(rc, "# Flutter", pathEntry)
	if err != nil {
		ui.Warning(stdout, "Não foi possível atualizar "+rc+": "+err.Error())
		return
	}
	if appended {
		ui.Info(stdout, "PATH atualizado em "+rc)
	}
}
