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

package main

// Bound methods for the "Desenvolvimento" category.

import (
	"context"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/oito2/perci/internal/dev/ide"
	"github.com/oito2/perci/internal/dev/llm"
	"github.com/oito2/perci/internal/dev/prereqs"
	"github.com/oito2/perci/internal/dev/sdks"
	"github.com/oito2/perci/internal/dev/terminal"
	"github.com/oito2/perci/internal/distro"
	"github.com/oito2/perci/internal/system/androidstudio"
	"github.com/oito2/perci/internal/system/antigravityide"
	"github.com/oito2/perci/internal/ui"
)

// DevSetupService binds the "Desenvolvimento" category: Pré-requisitos,
// SDKs, Apps de IA, Terminais, IDEs, Android Studio, Antigravity IDE.
type DevSetupService struct {
	serviceBase
}

// ── Tabelas com ações por item (Pré-requisitos, Linguagens e SDKs) ──────────

// ManagedItemInfo is one row of a table screen whose items each have their
// own Instalar/Atualizar/Desinstalar buttons.
type ManagedItemInfo struct {
	ID          string `json:"id"` // the item's Name
	Label       string `json:"label"`
	Description string `json:"description"`
	Installed   bool   `json:"installed"`
	// Missing lists the requirements not installed yet — while it isn't
	// empty the item can't be installed.
	Missing []string `json:"missing,omitempty"`
	// RemoveWarning is shown in the uninstall confirmation.
	RemoveWarning string `json:"removeWarning,omitempty"`
}

// managedItem is what runManagedItemAction needs from one item.
type managedItem struct {
	name      string
	installed func() bool
	missing   func() []string
	// install, update and uninstall run the operation, writing to stdout.
	install, update, uninstall func(stdout io.Writer) error
}

// runManagedItemAction runs op ("install", "update" or "uninstall") on the
// item lookup finds for name. The state is checked again here: install is
// refused for an installed item or one with missing requirements, update
// and uninstall for an item that isn't installed.
func runManagedItemAction(b *serviceBase, name, op string, lookup func(name string) (managedItem, bool)) error {
	return b.runAction(func(stdout io.Writer) error {
		it, ok := lookup(name)
		if !ok {
			return fmt.Errorf("item desconhecido: %q", name)
		}
		var run func(io.Writer) error
		var label string
		switch op {
		case "install":
			if it.installed() {
				return fmt.Errorf("%s já está instalado", it.name)
			}
			if missing := it.missing(); len(missing) > 0 {
				return fmt.Errorf("%s requer: %s", it.name, strings.Join(missing, ", "))
			}
			run, label = it.install, "Instalando "
		case "update", "uninstall":
			if !it.installed() {
				return fmt.Errorf("%s não está instalado", it.name)
			}
			run, label = it.update, "Atualizando "
			if op == "uninstall" {
				run, label = it.uninstall, "Desinstalando "
			}
		default:
			return fmt.Errorf("operação desconhecida: %q", op)
		}
		ui.Step(stdout, 1, 1, label+it.name)
		return run(stdout)
	})
}

// ── Pré-requisitos ──────────────────────────────────────────────────────────

// GetPrereqsInfo lists the prerequisite catalogue (internal/dev/prereqs),
// in its order, with each item's installed state and missing requirements.
func (d *DevSetupService) GetPrereqsInfo() []ManagedItemInfo {
	ctx := context.Background()
	installed := prereqs.InstalledMap(ctx, d.exe)
	items := make([]ManagedItemInfo, len(prereqs.Catalogue))
	for i, p := range prereqs.Catalogue {
		items[i] = ManagedItemInfo{
			ID: p.Name, Label: p.Name, Description: p.Description,
			Installed:     installed[p.Name],
			Missing:       prereqs.MissingRequirements(ctx, d.exe, p, installed),
			RemoveWarning: p.RemoveWarning,
		}
	}
	return items
}

// RunPrereqAction runs op ("install", "update" or "uninstall") on the
// prerequisite named name.
func (d *DevSetupService) RunPrereqAction(name, op string) error {
	ctx := context.Background()
	return runManagedItemAction(&d.serviceBase, name, op, func(name string) (managedItem, bool) {
		idx := slices.IndexFunc(prereqs.Catalogue, func(p prereqs.Prereq) bool { return p.Name == name })
		if idx < 0 {
			return managedItem{}, false
		}
		p := prereqs.Catalogue[idx]
		return managedItem{
			name:      p.Name,
			installed: func() bool { return prereqs.Installed(ctx, d.exe, p.Name) },
			missing: func() []string {
				return prereqs.MissingRequirements(ctx, d.exe, p, prereqs.InstalledMap(ctx, d.exe))
			},
			install:   func(w io.Writer) error { return prereqs.InstallOne(ctx, d.exe, w, p) },
			update:    func(w io.Writer) error { return prereqs.UpdateOne(ctx, d.exe, w, p) },
			uninstall: func(w io.Writer) error { return prereqs.UninstallOne(ctx, d.exe, w, p) },
		}, true
	})
}

// ── Linguagens e SDKs ───────────────────────────────────────────────────────

// GetSDKsInfo lists the SDK catalogue (internal/dev/sdks: Go + Flutter)
// with each item's installed state and missing requirements.
func (d *DevSetupService) GetSDKsInfo() []ManagedItemInfo {
	ctx := context.Background()
	installed := sdks.InstalledMap(ctx, d.exe)
	items := make([]ManagedItemInfo, len(sdks.Catalogue))
	for i, s := range sdks.Catalogue {
		items[i] = ManagedItemInfo{
			ID: s.Name, Label: s.Name, Description: s.Description,
			Installed:     installed[s.Name],
			Missing:       sdks.MissingRequirements(ctx, d.exe, s),
			RemoveWarning: s.RemoveWarning,
		}
	}
	return items
}

// RunSDKAction runs op ("install", "update" or "uninstall") on the SDK
// named name.
func (d *DevSetupService) RunSDKAction(name, op string) error {
	ctx := context.Background()
	return runManagedItemAction(&d.serviceBase, name, op, func(name string) (managedItem, bool) {
		idx := slices.IndexFunc(sdks.Catalogue, func(s sdks.SDK) bool { return s.Name == name })
		if idx < 0 {
			return managedItem{}, false
		}
		s := sdks.Catalogue[idx]
		return managedItem{
			name:      s.Name,
			installed: func() bool { return sdks.InstalledMap(ctx, d.exe)[s.Name] },
			missing:   func() []string { return sdks.MissingRequirements(ctx, d.exe, s) },
			install:   func(w io.Writer) error { return sdks.InstallOne(ctx, d.exe, w, s) },
			update:    func(w io.Writer) error { return sdks.UpdateOne(ctx, d.exe, w, s) },
			uninstall: func(w io.Writer) error { return sdks.UninstallOne(ctx, d.exe, w, s) },
		}, true
	})
}

// ── Aplicativos: IA (checklist simples) ─────────────────────────────────────

// aiAppsChecklist builds llm.Catalogue's checklistCatalog.
func (d *DevSetupService) aiAppsChecklist() checklistCatalog[llm.LLM] {
	return checklistCatalog[llm.LLM]{
		items:   llm.Catalogue,
		idOf:    func(l llm.LLM) string { return l.Name },
		labelOf: func(l llm.LLM) string { return l.Name },
		installed: func() map[string]bool {
			return llm.InstalledMap(context.Background(), d.exe)
		},
		apply: func(stdout io.Writer, toInstall, toRemove []string) error {
			return llm.Apply(context.Background(), d.exe, stdout, toInstall, toRemove)
		},
	}
}

// GetAIAppsInfo lists the AI CLI catalogue (internal/dev/llm) with its
// current installed state.
func (d *DevSetupService) GetAIAppsInfo() []MultiSelectItemInfo {
	return getChecklistInfo(d.aiAppsChecklist())
}

// RunAIApps installs newly checked AI apps and removes newly unchecked
// ones.
func (d *DevSetupService) RunAIApps(selectedNames []string) error {
	return runChecklist(&d.serviceBase, d.aiAppsChecklist(), selectedNames)
}

// ── Aplicativos: Terminais (checklist simples + botões do Starship) ─────────

// terminalsChecklist builds terminal.Catalogue's checklistCatalog.
func (d *DevSetupService) terminalsChecklist() checklistCatalog[terminal.Terminal] {
	return checklistCatalog[terminal.Terminal]{
		items:   terminal.Catalogue,
		idOf:    func(t terminal.Terminal) string { return t.Name },
		labelOf: func(t terminal.Terminal) string { return t.Name },
		installed: func() map[string]bool {
			return terminal.InstalledMap(context.Background(), d.exe)
		},
		apply: func(stdout io.Writer, toInstall, toRemove []string) error {
			return terminal.Apply(context.Background(), d.exe, stdout, toInstall, toRemove)
		},
	}
}

// GetTerminalsInfo lists the terminal catalogue (internal/dev/terminal,
// excluding Starship) with its current installed state.
func (d *DevSetupService) GetTerminalsInfo() []MultiSelectItemInfo {
	return getChecklistInfo(d.terminalsChecklist())
}

// RunTerminals installs newly checked terminals and removes newly
// unchecked ones.
func (d *DevSetupService) RunTerminals(selectedNames []string) error {
	return runChecklist(&d.serviceBase, d.terminalsChecklist(), selectedNames)
}

// ApplyStarship downloads/installs Starship and applies a default preset
// when there's no ~/.config/starship.toml yet — its own button, outside
// the checklist (internal/dev/terminal.InstallStarship is idempotent:
// reapplying it doesn't lose the existing config).
func (d *DevSetupService) ApplyStarship() error {
	return d.runAction(func(stdout io.Writer) error {
		return terminal.InstallStarship(context.Background(), d.exe, stdout, "")
	})
}

// RemoveStarship removes the Starship binary and its bash/zsh/fish init
// hooks — the "Remover Starship" button next to "Aplicar Starship".
// ~/.config/starship.toml is kept as starship.toml.perci-bak.
func (d *DevSetupService) RemoveStarship() error {
	return d.runAction(terminal.UninstallStarship)
}

// ── IDE: Zed Editor / VS Code / VSCodium (install/atualizar/desinstalar) ────

func findIDE(cmd string) (ide.IDE, bool) {
	for _, e := range ide.Catalogue {
		if e.Cmd == cmd {
			return e, true
		}
	}
	return ide.IDE{}, false
}

// GetIDEInfo reports whether the IDE identified by cmd ("zed"/"code"/
// "codium" — internal/dev/ide.Catalogue) is currently installed. cmd is
// derived by the frontend from the item's actionId ("ide-zed" → "zed",
// etc.).
func (d *DevSetupService) GetIDEInfo(cmd string) SingleAppInfo {
	e, ok := findIDE(cmd)
	if !ok {
		return SingleAppInfo{}
	}
	installed := ide.InstalledMap(context.Background(), d.exe)
	return SingleAppInfo{Installed: installed[e.Name]}
}

// InstallIDE installs the IDE identified by cmd.
func (d *DevSetupService) InstallIDE(cmd string) error {
	return d.runAction(func(stdout io.Writer) error {
		e, ok := findIDE(cmd)
		if !ok {
			return fmt.Errorf("IDE desconhecida: %s", cmd)
		}
		return ide.InstallOne(context.Background(), d.exe, stdout, e, distro.Detect())
	})
}

// UpdateIDE updates the IDE identified by cmd — reinstalls over it (ide.
// UpdateOne), the same operation Install performs for all three IDEs in
// this catalogue.
func (d *DevSetupService) UpdateIDE(cmd string) error {
	return d.runAction(func(stdout io.Writer) error {
		e, ok := findIDE(cmd)
		if !ok {
			return fmt.Errorf("IDE desconhecida: %s", cmd)
		}
		return ide.UpdateOne(context.Background(), d.exe, stdout, e, distro.Detect())
	})
}

// UninstallIDE uninstalls the IDE identified by cmd.
func (d *DevSetupService) UninstallIDE(cmd string) error {
	return d.runAction(func(stdout io.Writer) error {
		e, ok := findIDE(cmd)
		if !ok {
			return fmt.Errorf("IDE desconhecida: %s", cmd)
		}
		return ide.UninstallOne(context.Background(), d.exe, stdout, e, distro.Detect())
	})
}

// validateTarballPath checks that tarballPath is a regular .tar.gz file
// before it's handed to a package that extracts it with elevated privilege —
// the native file dialog only filters by extension in the UI, which is a
// convention, not a security control, so this is enforced again here.
func validateTarballPath(tarballPath string) error {
	if err := requireApprovedPath(tarballPath); err != nil {
		return err
	}
	info, err := os.Stat(tarballPath)
	if err != nil {
		return fmt.Errorf("arquivo inválido: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("selecione um arquivo .tar.gz válido")
	}
	if !strings.HasSuffix(tarballPath, ".tar.gz") {
		return fmt.Errorf("selecione um arquivo .tar.gz válido")
	}
	return nil
}

// ── IDE: Android Studio (local file, install/uninstall) ─────────────────────

// PickAndroidStudioTarball opens the native file picker and returns the
// chosen path, or "" if the user cancelled.
func (d *DevSetupService) PickAndroidStudioTarball() (string, error) {
	return approvePath(d.wailsApp.Dialog.OpenFile().
		SetTitle("Selecione o arquivo do Android Studio (.tar.gz)").
		AddFilter("Android Studio (*.tar.gz)", "*.tar.gz").
		PromptForSingleSelection())
}

// GetAndroidStudioInfo reports whether Android Studio is currently installed.
func (d *DevSetupService) GetAndroidStudioInfo() SingleAppInfo {
	return SingleAppInfo{Installed: androidstudio.Installed()}
}

// InstallAndroidStudio extracts tarballPath (chosen via
// PickAndroidStudioTarball) into /opt/android-studio and launches the setup
// wizard. Also used to "update" by re-running with a newer tarball.
func (d *DevSetupService) InstallAndroidStudio(tarballPath string) error {
	return d.runAction(func(stdout io.Writer) error {
		if err := validateTarballPath(tarballPath); err != nil {
			return err
		}
		return androidstudio.Install(context.Background(), d.exe, stdout, tarballPath)
	})
}

// UninstallAndroidStudio removes /opt/android-studio and its desktop entry.
func (d *DevSetupService) UninstallAndroidStudio() error {
	return d.runAction(func(stdout io.Writer) error {
		return androidstudio.Uninstall(context.Background(), d.exe, stdout)
	})
}

// ── IDE: Antigravity IDE (local file, install/update/uninstall) ────────────

// PickAntigravityIDETarball opens the native file picker and returns the
// chosen path, or "" if the user cancelled.
func (d *DevSetupService) PickAntigravityIDETarball() (string, error) {
	return approvePath(d.wailsApp.Dialog.OpenFile().
		SetTitle("Selecione o arquivo do Antigravity IDE (.tar.gz)").
		AddFilter("Antigravity IDE (*.tar.gz)", "*.tar.gz").
		PromptForSingleSelection())
}

// GetAntigravityIDEInfo reports whether Antigravity IDE is currently installed.
func (d *DevSetupService) GetAntigravityIDEInfo() SingleAppInfo {
	return SingleAppInfo{Installed: antigravityide.Installed()}
}

// InstallAntigravityIDE performs a first install from tarballPath.
func (d *DevSetupService) InstallAntigravityIDE(tarballPath string) error {
	return d.runAction(func(stdout io.Writer) error {
		if err := validateTarballPath(tarballPath); err != nil {
			return err
		}
		return antigravityide.Install(context.Background(), d.exe, stdout, tarballPath)
	})
}

// UpdateAntigravityIDE replaces the current installation with tarballPath.
func (d *DevSetupService) UpdateAntigravityIDE(tarballPath string) error {
	return d.runAction(func(stdout io.Writer) error {
		if err := validateTarballPath(tarballPath); err != nil {
			return err
		}
		return antigravityide.Update(context.Background(), d.exe, stdout, tarballPath)
	})
}

// UninstallAntigravityIDE removes the installation, desktop entry and icon.
func (d *DevSetupService) UninstallAntigravityIDE() error {
	return d.runAction(func(stdout io.Writer) error {
		return antigravityide.Uninstall(context.Background(), d.exe, stdout)
	})
}
