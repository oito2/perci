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
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/oito2/perci/internal/appstack"
	"github.com/oito2/perci/internal/config"
	"github.com/oito2/perci/internal/selfupdate"
	"github.com/oito2/perci/internal/ui"
	"github.com/oito2/perci/internal/version"
)

// HomeService binds the "Home" category. GetCategories/GetTheme/SetTheme/
// GetGUIThemes are transversal to the whole sidebar rather than specific
// to Home.
type HomeService struct {
	serviceBase
}

// GetCategories returns the sidebar's categories and items.
func (h *HomeService) GetCategories() []MenuCategory {
	return categories
}

// GetTheme returns the active daisyUI theme (~/.perci/config.yaml, field
// gui_theme), falling back to the default when unset or unrecognized.
func (h *HomeService) GetTheme() string {
	cfg, err := config.Load()
	if err != nil {
		return config.DefaultGUITheme
	}
	return cfg.GUIThemeOrDefault()
}

// SetTheme persists the chosen theme to ~/.perci/config.yaml. Always
// reloads the config first, so as not to overwrite other fields (e.g.
// Docker containers) changed elsewhere (another window, the tray) in the
// meantime.
func (h *HomeService) SetTheme(name string) error {
	name = oneOf(name, config.GUIThemes(), config.DefaultGUITheme)

	return config.Update(func(cfg *config.Config) error {
		cfg.GUITheme = name
		return nil
	})
}

// GetGUIThemes returns the fixed list of daisyUI themes the GUI offers, so
// the frontend's theme selector doesn't have to duplicate that list by
// hand.
func (h *HomeService) GetGUIThemes() []string {
	return config.GUIThemes()
}

// GetAppVersion returns the running binary's own version (internal/version)
// with no network access — used by "Home :: Visão Geral" (dashboard), which
// must stay useful offline, unlike GetSelfUpdateInfo below.
func (h *HomeService) GetAppVersion() string {
	return version.Version
}

// SelfUpdateInfo is what "Home :: Atualizar Perci" needs to render: the
// installed version, the latest one found on GitHub Releases, and whether
// they differ — or Error when the check itself failed (e.g. no network).
type SelfUpdateInfo struct {
	CurrentVersion  string `json:"currentVersion"`
	LatestVersion   string `json:"latestVersion,omitempty"`
	UpdateAvailable bool   `json:"updateAvailable"`
	Error           string `json:"error,omitempty"`
}

// GetSelfUpdateInfo checks GitHub Releases (internal/selfupdate.
// LatestRelease) and compares against the running version; RunSelfUpdate
// redoes the lookup before applying, never trusting this earlier check. A
// 10s timeout keeps a dead network from hanging the screen open
// indefinitely (LatestRelease itself has no timeout of its own).
func (h *HomeService) GetSelfUpdateInfo() SelfUpdateInfo {
	info := SelfUpdateInfo{CurrentVersion: version.Version}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rel, err := selfupdate.LatestRelease(ctx)
	if err != nil {
		info.Error = err.Error()
		return info
	}
	info.LatestVersion = rel.Version
	info.UpdateAvailable = selfupdate.IsNewer(rel.Version, version.Version)
	return info
}

// RunSelfUpdate checks for and applies an update — internal/selfupdate.Run.
// The GUI's "Atualizar agora" button click is itself the confirmation.
// Recalculates the latest release itself instead of trusting
// GetSelfUpdateInfo's earlier result.
func (h *HomeService) RunSelfUpdate() error {
	return h.runAction(func(stdout io.Writer) error {
		return selfupdate.Run(context.Background(), h.exe, stdout)
	})
}

// SelfConfigInfo is what "Home :: Configurar" needs to render its form —
// the two fields internal/selfupdate.Configure lets the user edit.
// Distro/DE and Theme are deliberately left out: Distro/DE are
// auto-detected, and the theme has its own setter.
type SelfConfigInfo struct {
	WorkspacePath string `json:"workspacePath"`
	FlatpakScope  string `json:"flatpakScope"`
}

// GetSelfConfigInfo reads the two editable settings from ~/.perci/config.yaml.
func (h *HomeService) GetSelfConfigInfo() SelfConfigInfo {
	cfg, err := config.Load()
	if err != nil {
		return SelfConfigInfo{FlatpakScope: "system"}
	}
	scope := cfg.FlatpakScope
	if scope != "user" {
		scope = "system"
	}
	return SelfConfigInfo{WorkspacePath: cfg.WorkspacePath, FlatpakScope: scope}
}

// PickWorkspaceFolder opens the native folder picker for the workspace
// path, with CanCreateDirectories enabled.
func (h *HomeService) PickWorkspaceFolder() (string, error) {
	return h.pickFolder("Selecionar pasta do workspace")
}

// SetWorkspacePath persists the workspace folder path chosen via
// PickWorkspaceFolder (expanding a leading ~). Synchronous, no Execução
// tab involved. Always reloads the config first: don't clobber other
// fields (e.g. Docker containers) another process may have changed while
// the GUI was open.
//
// Only a path picked in that dialog this session is accepted
// (requireApprovedPath): Docker creates folders under the workspace and
// mounts it into containers, so the webview must not be able to point it
// anywhere it likes.
func (h *HomeService) SetWorkspacePath(path string) error {
	if expanded, expandErr := config.ExpandPath(path); expandErr == nil {
		path = expanded
	}
	if err := requireApprovedPath(path); err != nil {
		return err
	}
	return config.Update(func(cfg *config.Config) error {
		cfg.WorkspacePath = path
		return nil
	})
}

// SetFlatpakScope persists the Flatpak install scope — applies instantly
// on select, synchronously like SetWorkspacePath.
func (h *HomeService) SetFlatpakScope(scope string) error {
	if scope != "user" {
		scope = "system"
	}
	return config.Update(func(cfg *config.Config) error {
		cfg.FlatpakScope = scope
		return nil
	})
}

// GetSidebarLogo returns the active sidebar mascot variant ("blue"/"pink"),
// with the same load/fallback shape as GetTheme.
func (h *HomeService) GetSidebarLogo() string {
	cfg, err := config.Load()
	if err != nil {
		return config.DefaultSidebarLogo
	}
	return cfg.SidebarLogoOrDefault()
}

// SetSidebarLogo persists the chosen sidebar mascot variant — applies
// immediately in the frontend (just swaps an <img> src, no restart needed),
// same "click applies and persists right away" convention as SetTheme.
func (h *HomeService) SetSidebarLogo(name string) error {
	name = oneOf(name, config.SidebarLogos(), config.DefaultSidebarLogo)

	return config.Update(func(cfg *config.Config) error {
		cfg.SidebarLogo = name
		return nil
	})
}

// GetSidebarLogos returns the fixed list of sidebar mascot variants offered.
func (h *HomeService) GetSidebarLogos() []string {
	return config.SidebarLogos()
}

// GetAppIcon returns the persisted window/taskbar icon variant
// ("blue"/"pink") — the one read at startup, not necessarily what's
// showing right now if the user just changed it (see SetAppIcon).
func (h *HomeService) GetAppIcon() string {
	cfg, err := config.Load()
	if err != nil {
		return config.DefaultAppIcon
	}
	return cfg.AppIconOrDefault()
}

// SetAppIcon persists the chosen window/taskbar icon variant. Unlike
// SetSidebarLogo, this has no live effect on the running window — the
// icon is only applied when the window is created, so the new window
// icon only shows up the next time Perci is started.
//
// When the application menu icon set is installed system-wide, it's
// replaced with the chosen color first — one pkexec prompt.
// The choice is only persisted if that succeeds, so a cancelled
// prompt leaves config and menu icon consistent.
func (h *HomeService) SetAppIcon(name string) error {
	name = oneOf(name, config.AppIcons(), config.DefaultAppIcon)

	if selfupdate.MenuIconsInstalled() {
		if err := selfupdate.InstallMenuIcons(context.Background(), h.exe, io.Discard, name); err != nil {
			return fmt.Errorf("instalar ícone do menu de aplicativos: %w", err)
		}
	}

	return config.Update(func(cfg *config.Config) error {
		cfg.AppIcon = name
		return nil
	})
}

// GetAppIcons returns the fixed list of window/taskbar icon variants offered.
func (h *HomeService) GetAppIcons() []string {
	return config.AppIcons()
}

// PickUninstallBackupPath opens the native "Salvar Como" dialog for the
// uninstall's "Fazer backup das configurações" checkbox — it differs from
// PickExportConfigPath only in its title/default filename; both save
// through the same appstack.ExportConfig underneath.
func (h *HomeService) PickUninstallBackupPath() (string, error) {
	return approvePath(h.wailsApp.Dialog.SaveFileWithOptions(&application.SaveFileDialogOptions{
		Title:    "Salvar backup das configurações",
		Filename: "perci-backup-" + time.Now().Format("2006-01-02-150405") + ".yaml",
		Filters:  []application.FileFilter{{DisplayName: "YAML (*.yaml)", Pattern: "*.yaml;*.yml"}},
	}).PromptForSingleSelection())
}

// RunSelfUninstall removes the Perci binary — internal/selfupdate.Uninstall.
// Two steps run first, both optional:
//
//   - backupPath != "": exports cfg.Docker to that path first (appstack.
//     ExportConfig) — reimportable later via "Importar configurações". A
//     failure here (e.g. nothing registered yet) is logged as a warning
//     but doesn't abort the uninstall itself.
//   - removeDocker: removes every registered container (apps, then
//     MariaDB, then Nginx — apps first so nothing routes through infra
//     that's about to disappear) via appstack.DeleteApp/DeleteMariaDB/
//     DeleteNginx — disk data (workspace/localhost/...) is left
//     untouched.
//
// Doesn't go through runAction: a successful uninstall must also close the
// GUI window, and runAction's generic "ok/error" event has no notion of
// that, so the goroutine/event/quit sequence is inlined here — with the
// same runMu serialization, panic recovery and emitDone.
// ErrUninstalled is success, not a failure, so it's translated to a nil
// error (ok:true) before the "action-done" event is sent; the window
// closes a short moment later (time enough to read the success line in
// the terminal panel).
func (h *HomeService) RunSelfUninstall(removeConfig, removeDocker bool, backupPath string) error {
	// A backup path not chosen in PickUninstallBackupPath refuses the whole
	// uninstall — checked before anything destructive starts.
	if backupPath != "" {
		if err := requireApprovedPath(backupPath); err != nil {
			return err
		}
	}
	if !runMu.TryLock() {
		return errActionRunning
	}
	go func() {
		defer runMu.Unlock()
		w := h.newEventWriter()
		quit := false
		err := runRecovered(func() error {
			err := h.selfUninstall(w, removeConfig, removeDocker, backupPath)
			if errors.Is(err, selfupdate.ErrUninstalled) {
				quit = true
				return nil
			}
			return err
		})
		w.Flush()
		h.emitDone(err)
		if quit {
			time.Sleep(uninstallQuitDelay)
			h.wailsApp.Quit()
		}
	}()
	return nil
}

// uninstallQuitDelay leaves the success line readable in the terminal
// panel before the window closes.
const uninstallQuitDelay = 1500 * time.Millisecond

// selfUninstall is RunSelfUninstall's body: optional backup, optional
// Docker cleanup, then selfupdate.Uninstall (whose ErrUninstalled means
// success). The tray's autostart entry is removed only once the uninstall
// itself succeeded.
func (h *HomeService) selfUninstall(w io.Writer, removeConfig, removeDocker bool, backupPath string) error {
	ctx := context.Background()

	if backupPath != "" {
		if err := appstack.ExportConfig(backupPath); err != nil {
			ui.Warning(w, "Falha ao fazer backup das configurações: "+err.Error())
		} else {
			ui.Success(w, "Backup salvo em "+backupPath+".")
		}
	}

	if removeDocker {
		if cfg, err := config.Load(); err == nil {
			for _, app := range cfg.Docker.Apps {
				if delErr := appstack.DeleteApp(ctx, h.exe, w, app.Folder); delErr != nil {
					ui.Warning(w, "Falha ao remover contêiner "+app.Folder+": "+delErr.Error())
				}
			}
			if delErr := appstack.DeleteMariaDB(ctx, h.exe, w); delErr != nil {
				ui.Warning(w, "Falha ao remover contêiner MariaDB: "+delErr.Error())
			}
			if delErr := appstack.DeleteNginx(ctx, h.exe, w); delErr != nil {
				ui.Warning(w, "Falha ao remover contêiner Nginx: "+delErr.Error())
			}
		} else {
			ui.Warning(w, "Falha ao carregar configuração para remover contêineres Docker: "+err.Error())
		}
	}

	err := selfupdate.Uninstall(ctx, h.exe, w, removeConfig)
	if errors.Is(err, selfupdate.ErrUninstalled) {
		// Tray autostart (~/.config/autostart/perci.desktop) — removed regardless
		// of TrayEnabled: with the binary gone, it would be a "ghost" autostart
		// entry (same class of problem as the app-menu .desktop entry, handled
		// inside selfupdate.Uninstall).
		if rmErr := removeAutostartDesktopFile(); rmErr != nil {
			ui.Warning(w, "Falha ao remover autostart da bandeja: "+rmErr.Error())
		}
	}
	return err
}
