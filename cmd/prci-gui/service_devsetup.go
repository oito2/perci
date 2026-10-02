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
	"strings"

	"github.com/oito2/perci/internal/dev/ide"
	"github.com/oito2/perci/internal/dev/llm"
	"github.com/oito2/perci/internal/dev/prereqs"
	"github.com/oito2/perci/internal/dev/sdks"
	"github.com/oito2/perci/internal/dev/terminal"
	"github.com/oito2/perci/internal/distro"
	"github.com/oito2/perci/internal/system/androidstudio"
	"github.com/oito2/perci/internal/system/antigravityide"
)

// DevSetupService binds the "Desenvolvimento" category: Pré-requisitos,
// SDKs, Apps de IA, Terminais, IDEs, Android Studio, Antigravity IDE.
type DevSetupService struct {
	serviceBase
}

// ── Pré-requisitos (checklist simples) ──────────────────────────────────────

// prereqsChecklist builds prereqs.Catalogue's checklistCatalog, setting
// descOf (p.Description).
func (d *DevSetupService) prereqsChecklist() checklistCatalog[prereqs.Prereq] {
	return checklistCatalog[prereqs.Prereq]{
		items:   prereqs.Catalogue,
		idOf:    func(p prereqs.Prereq) string { return p.Name },
		labelOf: func(p prereqs.Prereq) string { return p.Name },
		descOf:  func(p prereqs.Prereq) string { return p.Description },
		warnOf:  func(p prereqs.Prereq) string { return p.RemoveWarning },
		installed: func() map[string]bool {
			return prereqs.InstalledMap(context.Background(), d.exe)
		},
		apply: func(stdout io.Writer, toInstall, toRemove []string) error {
			return prereqs.Apply(context.Background(), d.exe, stdout, toInstall, toRemove)
		},
	}
}

// GetPrereqsInfo lists the prerequisite catalogue (internal/dev/prereqs)
// with its current installed state.
func (d *DevSetupService) GetPrereqsInfo() []MultiSelectItemInfo {
	return getChecklistInfo(d.prereqsChecklist())
}

// RunPrereqs installs newly checked prerequisites and removes newly
// unchecked ones — same diff (internal/sets.Diff) RunFonts uses.
func (d *DevSetupService) RunPrereqs(selectedNames []string) error {
	return runChecklist(&d.serviceBase, d.prereqsChecklist(), selectedNames)
}

// ── Linguagens e SDKs (checklist simples) ───────────────────────────────────

// sdksChecklist builds sdks.Catalogue's checklistCatalog.
func (d *DevSetupService) sdksChecklist() checklistCatalog[sdks.SDK] {
	return checklistCatalog[sdks.SDK]{
		items:   sdks.Catalogue,
		idOf:    func(s sdks.SDK) string { return s.Name },
		labelOf: func(s sdks.SDK) string { return s.Name },
		installed: func() map[string]bool {
			return sdks.InstalledMap(context.Background(), d.exe)
		},
		apply: func(stdout io.Writer, toInstall, toRemove []string) error {
			return sdks.Apply(context.Background(), d.exe, stdout, toInstall, toRemove)
		},
	}
}

// GetSDKsInfo lists the SDK catalogue (internal/dev/sdks: Go + Flutter)
// with its current installed state.
func (d *DevSetupService) GetSDKsInfo() []MultiSelectItemInfo {
	return getChecklistInfo(d.sdksChecklist())
}

// RunSDKs installs newly checked SDKs and removes newly unchecked ones.
func (d *DevSetupService) RunSDKs(selectedNames []string) error {
	return runChecklist(&d.serviceBase, d.sdksChecklist(), selectedNames)
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
