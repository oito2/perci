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

import (
	"context"
	"fmt"
	"io"

	"github.com/oito2/perci/internal/system/apps"
	"github.com/oito2/perci/internal/system/fonts"
	"github.com/oito2/perci/internal/system/linuxtoys"
	"github.com/oito2/perci/internal/system/megasync"
	"github.com/oito2/perci/internal/system/postinstall"
	"github.com/oito2/perci/internal/system/templates"
	"github.com/oito2/perci/internal/system/update"
	"github.com/oito2/perci/internal/ui"
)

// LinuxService binds the "Linux" category (catalog.go): Atualizar Sistema,
// Pós-instalação, Fontes, Templates, Flatpak Apps, Linux Toys, MegaSync.
type LinuxService struct {
	serviceBase
}

// RunSystemUpdate runs "Linux :: Atualizar Sistema" — the same function
// (internal/system/update.Run) domain callers everywhere use, no
// duplicated logic. Uses context.Background() instead of a request-scoped
// ctx for the run itself (only the eventWriter carries the wailsApp
// reference) — deliberate: a package update shouldn't be aborted midway
// just because the window closed, that could corrupt dpkg's state.
// cleanJournal/autoremove are the screen's optional cleanups
// (update.Options), both unchecked by default.
func (l *LinuxService) RunSystemUpdate(cleanJournal, autoremove bool) error {
	return l.runAction(func(stdout io.Writer) error {
		return update.Run(context.Background(), l.exe, stdout, update.Options{CleanJournal: cleanJournal, Autoremove: autoremove})
	})
}

// PostinstallActionInfo is one checklist item, without the unexported
// Run func (internal/system/postinstall.Action isn't JSON-safe as-is —
// Run is a func value).
type PostinstallActionInfo struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
	DependsOn   string `json:"dependsOn,omitempty"`
}

// PostinstallProfileInfo is what the frontend needs to render "Linux ::
// Pós-instalação" — either Supported (with the checklist) or not (shows
// "Seu Sistema Operacional não é suportado pelo Perci").
type PostinstallProfileInfo struct {
	Supported bool                    `json:"supported"`
	Label     string                  `json:"label,omitempty"`
	Actions   []PostinstallActionInfo `json:"actions,omitempty"`
}

// GetPostinstallProfile detects which of the supported OS/DE/version
// combinations Perci is running on, and returns its checklist — or
// Supported:false if the machine isn't one of them.
func (l *LinuxService) GetPostinstallProfile() PostinstallProfileInfo {
	profile := postinstall.DetectProfile()
	if profile == nil {
		return PostinstallProfileInfo{Supported: false}
	}
	actions := make([]PostinstallActionInfo, len(profile.Actions))
	for i, act := range profile.Actions {
		actions[i] = PostinstallActionInfo{
			ID: act.ID, Label: act.Label, Description: act.Description, DependsOn: act.DependsOn,
		}
	}
	return PostinstallProfileInfo{Supported: true, Label: profile.Label, Actions: actions}
}

// RunPostinstall runs the selected actionIDs from the detected Profile, in
// the Profile's own declared order (always dependency-safe — see
// Action.DependsOn's doc comment in internal/system/postinstall), one
// ui.Step per selected action — the step count comes for free from the
// selection itself, no separate declaration needed. Same
// context.Background()-for-execution reasoning as RunSystemUpdate: a
// partly-run post-install (apt mid package-manager-state) shouldn't be
// aborted just because the window closed.
func (l *LinuxService) RunPostinstall(actionIDs []string) error {
	return l.runAction(func(stdout io.Writer) error {
		profile := postinstall.DetectProfile()
		if profile == nil {
			return fmt.Errorf("sistema operacional não suportado")
		}
		selected := make(map[string]bool, len(actionIDs))
		for _, id := range actionIDs {
			selected[id] = true
		}
		var toRun []postinstall.Action
		for _, act := range profile.Actions {
			if selected[act.ID] {
				toRun = append(toRun, act)
			}
		}
		total := len(toRun)
		for i, act := range toRun {
			ui.Step(stdout, i+1, total, act.Label)
			if err := act.Run(context.Background(), l.exe, stdout); err != nil {
				return fmt.Errorf("%s: %w", act.Label, err)
			}
		}
		return nil
	})
}

// MultiSelectItemInfo is one checkbox item for the "checklist simples"
// frontend mode (#multiselect-container) — screens that toggle an
// installed state via multi-select with no dependency between items
// (fonts, templates, Flatpak, prerequisites, etc., all built on
// internal/sets.Diff).
type MultiSelectItemInfo struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
	Installed   bool   `json:"installed"`
	// RemoveWarning is confirmed in the GUI before an installed item is
	// unchecked and applied (js/screens/checklists.js).
	RemoveWarning string `json:"removeWarning,omitempty"`
}

// fontsChecklist builds fonts.Catalogue's checklistCatalog — GetFontsInfo/
// RunFonts are thin wrappers below.
func (l *LinuxService) fontsChecklist() checklistCatalog[fonts.Font] {
	return checklistCatalog[fonts.Font]{
		items:   fonts.Catalogue,
		idOf:    func(f fonts.Font) string { return f.Name },
		labelOf: func(f fonts.Font) string { return f.Name },
		installed: func() map[string]bool {
			return fonts.InstalledNames(context.Background(), l.exe)
		},
		apply: func(stdout io.Writer, toInstall, toRemove []string) error {
			return fonts.Apply(context.Background(), l.exe, stdout, toInstall, toRemove)
		},
	}
}

// GetFontsInfo lists the font catalogue (internal/system/fonts) with its
// current installed state, so each checkbox starts checked when the font
// is already installed.
func (l *LinuxService) GetFontsInfo() []MultiSelectItemInfo {
	return getChecklistInfo(l.fontsChecklist())
}

// RunFonts installs newly checked fonts and removes newly unchecked ones —
// same diff (internal/sets.Diff) every checklist screen uses. Recalculates
// the installed state on the spot rather than trusting what the frontend
// cached — same reasoning as DetectProfile in RunPostinstall.
func (l *LinuxService) RunFonts(selectedNames []string) error {
	return runChecklist(&l.serviceBase, l.fontsChecklist(), selectedNames)
}

// templatesChecklist builds templates.Catalogue's checklistCatalog, once
// templates.Dir() resolves — the one pair of the 7 whose install-state/
// apply need an extra dir resolved first, so this returns an error instead
// of the catalog when that fails (GetTemplatesInfo/RunTemplates below
// preserve their existing error behavior around it).
func (l *LinuxService) templatesChecklist() (checklistCatalog[templates.Template], error) {
	dir, err := templates.Dir()
	if err != nil {
		return checklistCatalog[templates.Template]{}, err
	}
	return checklistCatalog[templates.Template]{
		items:   templates.Catalogue,
		idOf:    func(t templates.Template) string { return t.Filename },
		labelOf: func(t templates.Template) string { return t.Label },
		installed: func() map[string]bool {
			return templates.PresentNames(dir)
		},
		apply: func(stdout io.Writer, toCreate, toRemove []string) error {
			return templates.Apply(stdout, dir, toCreate, toRemove)
		},
	}, nil
}

// GetTemplatesInfo lists the file-template catalogue (internal/system/
// templates) with its current presence state in templates.Dir(), so each
// checkbox starts checked when the template already exists there — same
// pattern as GetFontsInfo.
func (l *LinuxService) GetTemplatesInfo() []MultiSelectItemInfo {
	c, err := l.templatesChecklist()
	if err != nil {
		return nil
	}
	return getChecklistInfo(c)
}

// RunTemplates creates newly checked templates and removes newly unchecked
// ones — same diff (internal/sets.Diff) RunFonts uses. Recalculates the
// present state on the spot rather than trusting what the frontend cached
// — same reasoning as RunFonts/RunPostinstall.
func (l *LinuxService) RunTemplates(selectedFilenames []string) error {
	c, err := l.templatesChecklist()
	if err != nil {
		return l.runAction(func(stdout io.Writer) error { return err })
	}
	return runChecklist(&l.serviceBase, c, selectedFilenames)
}

// flatpakAppsChecklist builds apps.Catalogue's checklistCatalog.
func (l *LinuxService) flatpakAppsChecklist() checklistCatalog[apps.App] {
	return checklistCatalog[apps.App]{
		items:   apps.Catalogue,
		idOf:    func(a apps.App) string { return a.FlatID },
		labelOf: func(a apps.App) string { return a.Name },
		installed: func() map[string]bool {
			return apps.InstalledIDs(context.Background(), l.exe)
		},
		apply: func(stdout io.Writer, toInstall, toRemove []string) error {
			return apps.Apply(context.Background(), l.exe, stdout, toInstall, toRemove)
		},
	}
}

// GetFlatpakAppsInfo lists the Flatpak app catalogue (internal/system/apps)
// with its current installed state — same pattern as GetFontsInfo/
// GetTemplatesInfo.
func (l *LinuxService) GetFlatpakAppsInfo() []MultiSelectItemInfo {
	return getChecklistInfo(l.flatpakAppsChecklist())
}

// RunFlatpakApps installs newly checked apps and uninstalls newly unchecked
// ones — same diff (internal/sets.Diff) RunFonts/RunTemplates use.
// Recalculates the installed state on the spot, same reasoning as every
// other multi-select screen.
func (l *LinuxService) RunFlatpakApps(selectedFlatIDs []string) error {
	return runChecklist(&l.serviceBase, l.flatpakAppsChecklist(), selectedFlatIDs)
}

// SingleAppInfo is what the frontend needs for the "app único
// install/desinstalar" screen pattern (e.g. Linux :: Aplicativos - Linux
// Toys/MegaSync) — two buttons instead of a checklist: "Instalar" (always
// enabled) and "Desinstalar" (enabled only when Installed is true).
type SingleAppInfo struct {
	Installed bool `json:"installed"`
}

// GetLinuxToysInfo reports whether Linux Toys is currently installed.
func (l *LinuxService) GetLinuxToysInfo() SingleAppInfo {
	return SingleAppInfo{Installed: linuxtoys.Installed(context.Background(), l.exe)}
}

// InstallLinuxToys runs the official installer (internal/system/
// linuxtoys.Install) — the GUI's "Instalar" button click is itself the
// confirmation, no extra prompt.
func (l *LinuxService) InstallLinuxToys() error {
	return l.runAction(func(stdout io.Writer) error {
		return linuxtoys.Install(context.Background(), l.exe, stdout)
	})
}

// UninstallLinuxToys removes Linux Toys via the distro's native package
// manager (internal/system/linuxtoys.Uninstall).
func (l *LinuxService) UninstallLinuxToys() error {
	return l.runAction(func(stdout io.Writer) error {
		return linuxtoys.Uninstall(context.Background(), l.exe, stdout)
	})
}

// GetMegaSyncInfo reports whether MegaSync is currently installed — same
// "app único install/desinstalar" screen shape as Linux Toys.
func (l *LinuxService) GetMegaSyncInfo() SingleAppInfo {
	return SingleAppInfo{Installed: megasync.Installed(context.Background(), l.exe)}
}

// InstallMegaSync runs the official install flow (internal/system/
// megasync.Install) — same reasoning as InstallLinuxToys: the GUI's
// "Instalar" button click is itself the confirmation.
func (l *LinuxService) InstallMegaSync() error {
	return l.runAction(func(stdout io.Writer) error {
		return megasync.Install(context.Background(), l.exe, stdout)
	})
}

// UninstallMegaSync removes MegaSync via the distro's native package manager
// (internal/system/megasync.Uninstall).
func (l *LinuxService) UninstallMegaSync() error {
	return l.runAction(func(stdout io.Writer) error {
		return megasync.Uninstall(context.Background(), l.exe, stdout)
	})
}
