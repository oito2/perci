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

// Package sdks installs, updates and removes the SDKs managed by
// perci.gnl — Go, Rust and Flutter — one at a time, wrapping the golang,
// rust and flutter packages.
package sdks

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/oito2/perci/internal/dev/flutter"
	"github.com/oito2/perci/internal/dev/golang"
	"github.com/oito2/perci/internal/dev/nettimeout"
	"github.com/oito2/perci/internal/dev/prereqs"
	"github.com/oito2/perci/internal/dev/rust"
	"github.com/oito2/perci/internal/executor"
	"github.com/oito2/perci/internal/ui"
)

// SDK describes a language/SDK toolchain managed by perci.gnl.
type SDK struct {
	Name        string
	ID          string // "go", "rust" or "flutter"
	Description string
	// Requires lists the Names of the prereqs.Catalogue items its installer
	// needs.
	Requires []string
	// RemoveWarning is shown in the uninstall confirmation.
	RemoveWarning string
}

// Catalogue lists all SDKs managed by perci.gnl.
var Catalogue = []SDK{
	{Name: "Go SDK", ID: "go",
		Description:   "Linguagem Go, na versão estável mais recente, instalada em /usr/local/go",
		Requires:      []string{"Curl"},
		RemoveWarning: "Remove /usr/local/go e a linha do Go no PATH do arquivo de inicialização do shell."},
	{Name: "Rust (rustup)", ID: "rust",
		Description:   "Linguagem Rust (canal stable) pelo rustup, só para o seu usuário em ~/.cargo e ~/.rustup: rustc, cargo, clippy, rustfmt e rust-analyzer",
		Requires:      []string{"Curl", "Build Essential"},
		RemoveWarning: "Remove ~/.cargo (inclusive os programas instalados com cargo install), ~/.rustup com todos os toolchains, e a linha do Rust no PATH do arquivo de inicialização do shell."},
	{Name: "Flutter + Dart SDK", ID: "flutter",
		Description:   "Flutter (canal stable) com o Dart, em ~/development/flutter, mais o Android cmdline-tools e o wrapper do Chrome. A instalação aceita as licenças do Android SDK em seu nome; os termos estão em " + flutter.AndroidTermsURL + ".",
		Requires:      []string{"Git"},
		RemoveWarning: "Remove ~/development/flutter e a linha do Flutter no PATH do arquivo de inicialização do shell. O Android SDK (~/Android/Sdk) e o wrapper do Chrome continuam instalados."},
}

// paths resolves the fixed locations used by InstalledMap and the
// install, update and remove operations.
func paths() (home, flutterDir, flutterBin string, err error) {
	home, err = os.UserHomeDir()
	if err != nil {
		return "", "", "", fmt.Errorf("obter diretório home: %w", err)
	}
	flutterDir = filepath.Join(home, "development", "flutter")
	flutterBin = filepath.Join(flutterDir, "bin", "flutter")
	return home, flutterDir, flutterBin, nil
}

// InstalledMap returns which SDKs are currently installed (by Name).
func InstalledMap(ctx context.Context, exe *executor.Executor) map[string]bool {
	result := make(map[string]bool, len(Catalogue))

	goInstalled, _ := golang.InstalledVersion(ctx, exe)
	result["Go SDK"] = goInstalled
	result["Rust (rustup)"] = rust.IsInstalled()

	if _, _, flutterBin, err := paths(); err == nil {
		result["Flutter + Dart SDK"] = flutter.IsInstalled(flutterBin)
	}
	return result
}

// MissingRequirements returns the labels of s's requirements that aren't
// installed.
func MissingRequirements(ctx context.Context, exe *executor.Executor, s SDK) []string {
	var missing []string
	for _, req := range s.Requires {
		if !prereqs.Installed(ctx, exe, req) {
			missing = append(missing, req+" (Pré-requisitos)")
		}
	}
	return missing
}

// InstallOne installs s.
func InstallOne(ctx context.Context, exe *executor.Executor, stdout io.Writer, s SDK) error {
	home, flutterDir, flutterBin, err := paths()
	if err != nil {
		return err
	}
	return install(ctx, exe, stdout, s.ID, home, flutterDir, flutterBin)
}

// UpdateOne updates an installed s: Go to the latest stable release (no-op
// when it already is), Rust through "rustup update", Flutter through
// "flutter upgrade".
func UpdateOne(ctx context.Context, exe *executor.Executor, stdout io.Writer, s SDK) error {
	_, _, flutterBin, err := paths()
	if err != nil {
		return err
	}
	return update(ctx, exe, stdout, s.ID, flutterBin)
}

// UninstallOne removes s.
func UninstallOne(ctx context.Context, exe *executor.Executor, stdout io.Writer, s SDK) error {
	home, flutterDir, _, err := paths()
	if err != nil {
		return err
	}
	return remove(ctx, exe, stdout, s.ID, home, flutterDir)
}

// install installs an SDK that isn't installed yet.
func install(ctx context.Context, exe *executor.Executor, stdout io.Writer, id, home, flutterDir, flutterBin string) error {
	switch id {
	case "go":
		// Needs go.dev reachable for the download and its checksum.
		latest, checksum, err := golang.LatestRelease(ctx)
		if err != nil {
			return fmt.Errorf("obter a versão mais recente do Go: %w", err)
		}
		if err := golang.Install(ctx, exe, stdout, latest, checksum); err != nil {
			return err
		}
		// Puts Go's bin directory on PATH in the shell rc file.
		golang.EnsurePathInBashrc(stdout)
		return nil
	case "rust":
		return rust.Install(ctx, exe, stdout)
	case "flutter":
		if err := flutter.CloneFlutter(ctx, exe, stdout, home, flutterDir); err != nil {
			return err
		}
		flutter.EnsurePathInBashrc(stdout, home)
		if err := flutter.InstallPrereqs(ctx, exe, stdout); err != nil {
			ui.Warning(stdout, "Falha ao instalar pré-requisitos do Flutter: "+err.Error())
		}
		if err := flutter.EnsureAndroidCmdlineTools(ctx, exe, stdout, home, flutterBin); err != nil {
			ui.Warning(stdout, "Falha ao instalar o Android cmdline-tools: "+err.Error())
		}
		if err := flutter.EnsureChromeWrapper(ctx, exe, stdout, home); err != nil {
			ui.Warning(stdout, "Falha ao criar o wrapper do Chrome: "+err.Error())
		}
		ui.Info(stdout, "Executando diagnóstico (flutter doctor)...")
		_ = exe.Run(ctx, executor.Options{Stdout: stdout, Stderr: stdout}, flutterBin, "doctor")
		return nil
	}
	return fmt.Errorf("instalador desconhecido para %s", id)
}

// latestGoRelease and installedGoVersion look up the Go versions update
// compares. Replaced in tests.
var (
	latestGoRelease    = golang.LatestRelease
	installedGoVersion = golang.InstalledVersion
)

// update updates an installed SDK.
func update(ctx context.Context, exe *executor.Executor, stdout io.Writer, id, flutterBin string) error {
	switch id {
	case "go":
		latest, checksum, err := latestGoRelease(ctx)
		if err != nil {
			return fmt.Errorf("obter a versão mais recente do Go: %w", err)
		}
		if _, current := installedGoVersion(ctx, exe); current == latest {
			ui.Info(stdout, "Go já está na versão mais recente ("+current+").")
			return nil
		}
		return golang.Install(ctx, exe, stdout, latest, checksum)
	case "rust":
		return rust.Update(ctx, exe, stdout)
	case "flutter":
		ui.Info(stdout, "Atualizando Flutter (flutter upgrade)...")
		ctx, cancel := nettimeout.Context(ctx)
		defer cancel()
		return exe.Run(ctx, executor.Options{Stdout: stdout, Stderr: stdout}, flutterBin, "upgrade")
	}
	return fmt.Errorf("atualizador desconhecido para %s", id)
}

func remove(ctx context.Context, exe *executor.Executor, stdout io.Writer, id, home, flutterDir string) error {
	switch id {
	case "go":
		return golang.Uninstall(ctx, exe, stdout)
	case "rust":
		return rust.Uninstall(ctx, exe, stdout)
	case "flutter":
		return flutter.Uninstall(ctx, exe, stdout, home, flutterDir)
	}
	return fmt.Errorf("desinstalador desconhecido para %s", id)
}
