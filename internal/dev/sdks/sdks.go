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

// Package sdks provides the GUI-only "checklist simples" wrapper
// ("Desenvolvimento :: Linguagens e SDKs" screen) around internal/dev/golang
// and internal/dev/flutter — a diff-driven install/remove of exactly the
// same two SDKs, without an extra "already installed, want to update?"
// confirmation step: same pattern as internal/system/fonts.Apply, clicking
// "Executar" is itself the confirmation (an item that's checked and already
// installed is a no-op, same as every multi-select screen in the GUI).
package sdks

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/oito2/perci/internal/checklist"
	"github.com/oito2/perci/internal/dev/flutter"
	"github.com/oito2/perci/internal/dev/golang"
	"github.com/oito2/perci/internal/executor"
	"github.com/oito2/perci/internal/ui"
)

// SDK describes a language/SDK toolchain managed by perci.gnl.
type SDK struct {
	Name string
	ID   string // "go" or "flutter"
}

// Catalogue lists all SDKs managed by perci.gnl.
var Catalogue = []SDK{
	{Name: "Go SDK", ID: "go"},
	{Name: "Flutter + Dart SDK", ID: "flutter"},
}

// paths resolves the fixed locations used by both InstalledMap and Apply.
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

	if _, _, flutterBin, err := paths(); err == nil {
		result["Flutter + Dart SDK"] = flutter.IsInstalled(flutterBin)
	}
	return result
}

// Apply installs SDKs listed in toInstall and removes those in toRemove —
// same pattern as internal/system/fonts.Apply: one ui.Step per processed
// SDK, with the total coming for free from the selection itself
// (internal/checklist.Apply).
func Apply(ctx context.Context, exe *executor.Executor, stdout io.Writer, toInstall, toRemove []string) error {
	home, flutterDir, flutterBin, err := paths()
	if err != nil {
		return err
	}

	return checklist.Apply(stdout, Catalogue, func(s SDK) string { return s.Name }, toInstall, toRemove,
		func(s SDK) error { return install(ctx, exe, stdout, s.ID, home, flutterDir, flutterBin) },
		func(s SDK) error { return remove(ctx, exe, stdout, s.ID, home, flutterDir) },
	)
}

// install installs an SDK that isn't installed yet.
func install(ctx context.Context, exe *executor.Executor, stdout io.Writer, id, home, flutterDir, flutterBin string) error {
	switch id {
	case "go":
		// No offline fallback: installing needs go.dev reachable anyway,
		// for the download and its checksum.
		latest, checksum, err := golang.LatestRelease(ctx)
		if err != nil {
			return fmt.Errorf("obter a versão mais recente do Go: %w", err)
		}
		if err := golang.Install(ctx, exe, stdout, latest, checksum); err != nil {
			return err
		}
		// Uninstall removes this same line (golang.Uninstall).
		golang.EnsurePathInBashrc(stdout)
		return nil
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

func remove(ctx context.Context, exe *executor.Executor, stdout io.Writer, id, home, flutterDir string) error {
	switch id {
	case "go":
		return golang.Uninstall(ctx, exe, stdout)
	case "flutter":
		return flutter.Uninstall(ctx, exe, stdout, home, flutterDir)
	}
	return fmt.Errorf("desinstalador desconhecido para %s", id)
}
