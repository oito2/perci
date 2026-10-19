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
	"os"
	"slices"
	"strings"

	"github.com/oito2/perci/internal/config"
	"github.com/oito2/perci/internal/system/apps"
	"github.com/oito2/perci/internal/system/fonts"
	"github.com/oito2/perci/internal/system/postinstall"
	"github.com/oito2/perci/internal/system/templates"
	"github.com/oito2/perci/internal/system/update"
	"github.com/oito2/perci/internal/ui"
)

// LinuxService binds the "Linux" category: Atualizar Sistema,
// Pós-instalação, Fontes, Templates, Flatpak Apps.
type LinuxService struct {
	serviceBase
}

// RunSystemUpdate runs "Linux :: Atualizar Sistema" through
// internal/system/update.Run. Uses context.Background() instead of a
// request-scoped ctx for the run itself: a package update isn't aborted
// midway just because the window closed, which could corrupt dpkg's
// state. cleanJournal/autoremove are the screen's optional cleanups
// (update.Options), both unchecked by default.
func (l *LinuxService) RunSystemUpdate(cleanJournal, autoremove bool) error {
	return l.runAction(func(stdout io.Writer) error {
		return update.Run(context.Background(), l.exe, stdout, update.Options{CleanJournal: cleanJournal, Autoremove: autoremove})
	})
}

// PostinstallActionInfo is one item of "Linux :: Pós-instalação", without
// the Run and Applied funcs of internal/system/postinstall.Action (func
// values aren't JSON-safe).
type PostinstallActionInfo struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
	DependsOn   string `json:"dependsOn,omitempty"`
}

// PostinstallProfileInfo is what the frontend needs to render "Linux ::
// Pós-instalação" — either Supported (with the item list) or not (shows
// "Seu Sistema Operacional não é suportado pelo Perci").
type PostinstallProfileInfo struct {
	Supported bool                    `json:"supported"`
	Label     string                  `json:"label,omitempty"`
	Actions   []PostinstallActionInfo `json:"actions,omitempty"`
}

// detectPostinstallProfile returns the post-install profile of this
// machine, or nil when it isn't supported. Replaced in tests.
var detectPostinstallProfile = postinstall.DetectProfile

// GetPostinstallProfile detects which of the supported OS/DE/version
// combinations Perci is running on, and returns its items — or
// Supported:false if the machine isn't one of them.
func (l *LinuxService) GetPostinstallProfile() PostinstallProfileInfo {
	profile := detectPostinstallProfile()
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

// postinstallMachineID returns this machine's /etc/machine-id, or "" when it
// can't be read. Replaced in tests.
var postinstallMachineID = func() string {
	data, err := os.ReadFile("/etc/machine-id")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// postinstallStatus returns, for every action of profile, whether it
// counts as applied: detected on the system, or recorded in config.yaml
// for the actions without a detectable state. Records only count when they
// were made on this machine (same /etc/machine-id).
func (l *LinuxService) postinstallStatus(profile *postinstall.Profile) map[string]bool {
	var recorded []string
	if cfg, err := config.Load(); err == nil {
		if id := postinstallMachineID(); id != "" && cfg.PostinstallMachineID == id {
			recorded = cfg.PostinstallApplied[profile.ID]
		}
	}
	return profile.Status(postinstall.NewProbe(context.Background(), l.exe), recorded)
}

// GetPostinstallStatus returns, per action ID of the detected profile,
// whether that action counts as applied — empty when the machine isn't
// supported.
func (l *LinuxService) GetPostinstallStatus() map[string]bool {
	profile := detectPostinstallProfile()
	if profile == nil {
		return map[string]bool{}
	}
	return l.postinstallStatus(profile)
}

// RunPostinstallAction applies the action actionID of the detected
// profile. It refuses an action that already counts as applied or whose
// DependsOn doesn't. After a successful run of an action without a
// detectable state, its ID is recorded in config.yaml. Like
// RunSystemUpdate it executes with context.Background(): a partly-run
// post-install step isn't aborted just because the window closed.
func (l *LinuxService) RunPostinstallAction(actionID string) error {
	return l.runAction(func(stdout io.Writer) error {
		profile := detectPostinstallProfile()
		if profile == nil {
			return fmt.Errorf("sistema operacional não suportado")
		}
		act, ok := profile.Action(actionID)
		if !ok {
			return fmt.Errorf("item de pós-instalação desconhecido: %q", actionID)
		}
		status := l.postinstallStatus(profile)
		if status[act.ID] {
			return fmt.Errorf("%s: já aplicado", act.Label)
		}
		if act.DependsOn != "" && !status[act.DependsOn] {
			dep, _ := profile.Action(act.DependsOn)
			return fmt.Errorf("%s: aplique antes %q", act.Label, dep.Label)
		}
		ui.Step(stdout, 1, 1, act.Label)
		if err := act.Run(context.Background(), l.exe, stdout); err != nil {
			return fmt.Errorf("%s: %w", act.Label, err)
		}
		if act.Applied != nil {
			return nil
		}
		machineID := postinstallMachineID()
		if machineID == "" {
			return nil // without a machine ID the record could never count
		}
		return config.Update(func(cfg *config.Config) error {
			if cfg.PostinstallMachineID != machineID {
				// Records made on another machine ID are replaced.
				cfg.PostinstallApplied = nil
				cfg.PostinstallMachineID = machineID
			}
			if slices.Contains(cfg.PostinstallApplied[profile.ID], act.ID) {
				return nil
			}
			if cfg.PostinstallApplied == nil {
				cfg.PostinstallApplied = make(map[string][]string)
			}
			cfg.PostinstallApplied[profile.ID] = append(cfg.PostinstallApplied[profile.ID], act.ID)
			return nil
		})
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
	// unchecked and applied.
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
// cached.
func (l *LinuxService) RunFonts(selectedNames []string) error {
	return runChecklist(&l.serviceBase, l.fontsChecklist(), selectedNames)
}

// templatesChecklist builds templates.Catalogue's checklistCatalog, once
// templates.Dir() resolves — it returns an error instead of the catalog
// when that fails.
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
// — same reasoning as RunFonts.
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
