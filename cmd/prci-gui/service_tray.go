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
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"github.com/oito2/perci/internal/config"
)

// TrayService binds the system tray / compact window domain. mainWindow
// is captured on the main window's creation; tray is non-nil only while
// TrayEnabled is on; compactWindow is the single compact window the 3
// tray menu items open/focus — Contêineres/Repositórios/Atualizar
// Sistema become tabs inside it (one reused window, not one per item).
// windowMu protects tray and compactWindow — each bound method the
// frontend calls runs on its own goroutine, so concurrent SetTrayEnabled
// calls (e.g. a quick double-click on the tray toggle) can't leave `tray`
// inconsistent.
type TrayService struct {
	serviceBase

	mainWindow    *application.WebviewWindow
	windowMu      sync.Mutex
	tray          *application.SystemTray
	compactWindow *application.WebviewWindow
}

// compactTabs is the fixed set of tabs the single compact window's tab
// bar offers — each one matches the `?compact=` query value /
// "compact-switch-tab" event payload, which the frontend resolves back to
// a MenuItem by actionId — the 3 keys here are literally the actionId of
// the 3 items ("docker-manage"/"repos"/"system-update").
var compactTabs = map[string]bool{
	"docker-manage": true,
	"repos":         true,
	"system-update": true,
}

// GetTrayEnabled exposes the tray toggle's current state (Home ::
// Configurações), so it starts checked correctly.
func (t *TrayService) GetTrayEnabled() bool {
	cfg, err := config.Load()
	if err != nil {
		return false
	}
	return cfg.TrayEnabled
}

// SetTrayEnabled turns the system tray icon on/off (Home ::
// Configurações) — applies instantly, no restart needed: SystemTray has
// real New()/Destroy() calls at runtime. Always reloads the config
// before writing — don't overwrite fields another process changed while
// the GUI was open.
func (t *TrayService) SetTrayEnabled(enabled bool) error {
	if err := config.Update(func(cfg *config.Config) error {
		cfg.TrayEnabled = enabled
		return nil
	}); err != nil {
		return err
	}

	if enabled {
		t.ensureTray()
		return writeAutostartDesktopFile()
	}

	t.destroyTray()
	return removeAutostartDesktopFile()
}

// ensureTray creates the tray icon if it doesn't exist yet — idempotent
// (called both at boot, when TrayEnabled is already on, and from
// SetTrayEnabled(true)).
//
// No AttachWindow: that method ties a left click to an automatic
// Show()/Hide() of the main window. Linux distros differ on whether a
// physical click becomes the Activate or SecondaryActivate DBus signal,
// so both handlers (OnClick/OnRightClick) open the same menu —
// deterministic on any distro/DE.
func (t *TrayService) ensureTray() {
	t.windowMu.Lock()
	defer t.windowMu.Unlock()

	if t.tray == nil {
		tray := t.wailsApp.SystemTray.New()
		// The click handlers capture this tray itself, never read t.tray:
		// they run outside windowMu, and a click delivered after the toggle
		// was turned off (t.tray already nil) would otherwise race on the
		// field or dereference nil.
		tray.OnClick(tray.OpenMenu)
		tray.OnRightClick(tray.OpenMenu)
		t.tray = tray
	}
	t.tray.SetIcon(t.traySmallIconSource())
	t.tray.SetTooltip("Perci")
	t.tray.SetMenu(t.buildTrayMenu())
}

// destroyTray undoes ensureTray when the toggle is turned off.
func (t *TrayService) destroyTray() {
	t.windowMu.Lock()
	defer t.windowMu.Unlock()

	if t.tray == nil {
		return
	}
	t.tray.Destroy()
	t.tray = nil
}

// buildTrayMenu builds the tray menu — Abrir Aplicativo (shows/focuses the
// main window, same as the compact window's header button) +
// Contêineres/Repositórios/Atualizar Sistema (each opens/focuses the
// single compact window, already on the matching tab — one window with 3
// tabs, not 3 separate windows) + Sair (the only way to quit Perci while
// the tray is enabled, since closing the main window only hides it).
func (t *TrayService) buildTrayMenu() *application.Menu {
	menu := t.wailsApp.NewMenu()
	menu.Add("Abrir Aplicativo").OnClick(func(_ *application.Context) {
		t.FocusMainWindow()
	})
	menu.AddSeparator()
	menu.Add("Contêineres").OnClick(func(_ *application.Context) {
		t.openCompactWindow("docker-manage")
	})
	menu.Add("Repositórios").OnClick(func(_ *application.Context) {
		t.openCompactWindow("repos")
	})
	menu.Add("Atualizar Sistema").OnClick(func(_ *application.Context) {
		t.openCompactWindow("system-update")
	})
	menu.AddSeparator()
	menu.Add("Sair").OnClick(func(_ *application.Context) {
		t.wailsApp.Quit()
	})
	return menu
}

// FocusMainWindow brings the main window to the front — used by the tray
// menu's "Abrir Aplicativo" item and the button of the same name in the
// compact window's header (#compact-header), to scale up from the tray to
// the full app (also when it starts hidden).
func (t *TrayService) FocusMainWindow() {
	if t.mainWindow == nil {
		return
	}
	t.mainWindow.Show().Focus()
}

// openCompactWindow opens the tray menu's single compact window (if it
// doesn't exist yet) already on the requested tab, or — if it's already
// open — just focuses it and tells the frontend to switch tabs via an
// event (one window with 3 tabs — Contêineres/Repositórios/Atualizar
// Sistema — sized 735×825). The header (image+"Perci"+"Abrir
// Aplicativo") and the tabs themselves live entirely in the frontend
// (?compact=<tab> skips the sidebar).
func (t *TrayService) openCompactWindow(tab string) {
	if !compactTabs[tab] {
		return
	}

	t.windowMu.Lock()
	defer t.windowMu.Unlock()

	if t.compactWindow != nil {
		t.compactWindow.Show().Focus()
		t.wailsApp.Event.Emit("compact-switch-tab", tab)
		return
	}

	win := t.wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     "Perci",
		Width:     735,
		Height:    825,
		MinWidth:  480,
		MinHeight: 560,
		URL:       "/?compact=" + tab,
		Linux: application.LinuxWindow{
			Icon:             t.trayIconSource(),
			WebviewGpuPolicy: application.WebviewGpuPolicyNever,
		},
	})
	t.compactWindow = win

	win.RegisterHook(events.Common.WindowClosing, func(_ *application.WindowEvent) {
		t.windowMu.Lock()
		t.compactWindow = nil
		t.windowMu.Unlock()
	})
}

// trayIconSource returns which 512 px square icon (iconBlue/iconPink) to
// use as the window icon of the main and compact windows — follows the
// AppIconOrDefault choice.
func (t *TrayService) trayIconSource() []byte {
	if cfg, err := config.Load(); err == nil && cfg.AppIconOrDefault() == "pink" {
		return iconPink
	}
	return iconBlue
}

// traySmallIconSource returns the tray's dedicated square icon
// (iconBlueTray/iconPinkTray) — not the window's large PNG:
// StatusNotifierItem expects a small icon.
func (t *TrayService) traySmallIconSource() []byte {
	if cfg, err := config.Load(); err == nil && cfg.AppIconOrDefault() == "pink" {
		return iconPinkTray
	}
	return iconBlueTray
}

// autostartDesktopPath is where writeAutostartDesktopFile/
// removeAutostartDesktopFile read/write — XDG autostart, read by the
// graphical session at login (not by the application menu:
// NoDisplay=true below).
func autostartDesktopPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "autostart", "perci.desktop"), nil
}

// writeAutostartDesktopFile creates/updates the hidden tray autostart
// entry — called from inside SetTrayEnabled(true). Exec points at the
// real binary resolved via os.Executable() (the same function
// internal/selfupdate uses to find the binary itself), not the `perci`
// symlink — which may not exist, depending on how it was installed.
func writeAutostartDesktopFile() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	path, err := autostartDesktopPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	execLine, err := desktopExecQuote(exe)
	if err != nil {
		return err
	}
	content := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=Perci
Comment=Perci iniciado oculto na bandeja do sistema
Exec=%s %s
X-GNOME-Autostart-enabled=true
NoDisplay=true
Terminal=false
`, execLine, hiddenFlag)
	return os.WriteFile(path, []byte(content), 0o644)
}

// hiddenFlag starts Perci with its main window hidden — passed by the
// tray's autostart entry, so logging in doesn't pop the full window up.
const hiddenFlag = "--hidden"

// desktopExecQuote renders one argument for a Desktop Entry's Exec key: in
// double quotes, with double quote, backtick, dollar sign and backslash
// backslash-escaped (the spec's quoting rule), every backslash then escaped
// again (its string escaping rule, applied first when reading), and `%`
// doubled (field codes).
func desktopExecQuote(arg string) (string, error) {
	if strings.ContainsAny(arg, "\n\r") {
		return "", fmt.Errorf("caminho inválido para um arquivo .desktop: %q", arg)
	}
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range arg {
		switch r {
		case '"', '`', '$':
			b.WriteString(`\\`)
			b.WriteRune(r)
		case '\\':
			b.WriteString(`\\\\`)
		case '%':
			b.WriteString("%%")
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String(), nil
}

// removeAutostartDesktopFile undoes writeAutostartDesktopFile when the
// toggle is turned off — a missing file (never enabled) is not an error.
func removeAutostartDesktopFile() error {
	path, err := autostartDesktopPath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
