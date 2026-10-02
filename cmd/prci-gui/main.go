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

// Command prci-gui is Perci's graphical interface, built on Wails v3 (GTK4
// + WebKitGTK 6.0 on Linux), wiring together the domain services
// (HomeService/LinuxService/DevSetupService/DockerService/DevToolsService/
// TrayService) and the main window.
package main

import (
	"context"
	"embed"
	"fmt"
	"os"
	"slices"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"github.com/oito2/perci/internal/config"
	"github.com/oito2/perci/internal/dev/localbin"
	"github.com/oito2/perci/internal/executor"
	"github.com/oito2/perci/internal/ui"
	"github.com/oito2/perci/packaging"
)

//go:embed frontend/dist
var assets embed.FS

// iconBlue/iconPink back "Home :: Configurações :: Ícone do Aplicativo" —
// the square 512 px icon PNGs, not the non-square sidebar mascot PNGs.
// Wails' window icon (application.LinuxWindow.Icon) takes PNG bytes.
//
// Wails has no runtime API to change a window's icon after creation, so
// the choice is read once, here, before the window is created; switching
// it in the GUI only takes effect the next time Perci is started.
var (
	iconBlue = packaging.MustIcon("blue", 512)
	iconPink = packaging.MustIcon("pink", 512)
)

// iconBlueTray/iconPinkTray back the system tray icon — dedicated 64×64
// square assets.
//
//go:embed frontend/dist/assets/perci-blue-systray.png
var iconBlueTray []byte

//go:embed frontend/dist/assets/perci-pink-systray.png
var iconPinkTray []byte

func main() {
	home := &HomeService{}
	linux := &LinuxService{}
	devSetup := &DevSetupService{}
	docker := &DockerService{}
	devTools := &DevToolsService{}
	tray := &TrayService{}

	cfg, cfgErr := config.Load()
	if cfgErr != nil {
		fmt.Fprintln(os.Stderr, "aviso: não foi possível ler as configurações, usando os padrões:", cfgErr)
	}
	// The tray's autostart entry launches `prci --hidden`: on login Perci
	// starts in the tray only. Honored only while the tray is enabled —
	// without it, a hidden window would leave no way to reach the app.
	startHidden := slices.Contains(os.Args[1:], hiddenFlag) && cfgErr == nil && cfg.TrayEnabled
	icon := iconBlue
	if cfgErr == nil && cfg.AppIconOrDefault() == "pink" {
		icon = iconPink
	}

	wailsApp := application.New(application.Options{
		Name: "Perci",
		Services: []application.Service{
			application.NewService(home),
			application.NewService(linux),
			application.NewService(devSetup),
			application.NewService(docker),
			application.NewService(devTools),
			application.NewService(tray),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
	})

	// internal/executor's UsePolicyKit makes RequiresSudo escalate via
	// pkexec instead of sudo — needed because the GUI has no interactive
	// terminal for sudo to prompt on. Each service gets its own Executor
	// instance (Executor holds no state shared across domains, so there's
	// no benefit to sharing one).
	for _, b := range []*serviceBase{
		&home.serviceBase, &linux.serviceBase, &devSetup.serviceBase,
		&docker.serviceBase, &devTools.serviceBase, &tray.serviceBase,
	} {
		b.wailsApp = wailsApp
		b.exe = executor.New(nil, nil)
		b.exe.UsePolicyKit = true
	}

	// A GUI started from the desktop doesn't get the PATH a login shell
	// builds (~/.local/bin, nvm's Node): tools installed there would read
	// as missing. Blocking, but only a shell sourcing nvm.sh — before
	// any binding can check what's installed.
	localbin.RefreshPath(context.Background(), home.exe)

	// ui.Step normally just writes one line; the hook below also emits a
	// "step" event to the frontend (title + progress bar in the Execução
	// tab). Package-level state in internal/ui, configured once here — it
	// doesn't belong to any specific service.
	ui.SetStepHook(func(index, total int, label string) {
		wailsApp.Event.Emit("step", map[string]interface{}{
			"index": index,
			"total": total,
			"label": label,
		})
	})

	mainWindow := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     "Perci",
		Width:     1100,
		Height:    720,
		MinWidth:  800,
		MinHeight: 560,
		Hidden:    startHidden,
		Linux: application.LinuxWindow{
			Icon: icon,
			// Explicit: options.Linux == nil would default to
			// WebviewGpuPolicyNever, but passing Icon makes it non-nil,
			// and the zero value would then be WebviewGpuPolicyOnDemand.
			WebviewGpuPolicy: application.WebviewGpuPolicyNever,
		},
	})
	tray.mainWindow = mainWindow

	// Closing (X) only hides into the tray when it's enabled — read from
	// config on every close, not from an in-memory flag, so toggling it at
	// runtime (SetTrayEnabled) takes effect immediately with no extra
	// synchronization. With the tray disabled (the default), X still quits
	// as usual — otherwise the app would vanish with no way to reopen it,
	// since the tray is opt-in.
	mainWindow.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		if trayCfg, err := config.Load(); err == nil && trayCfg.TrayEnabled {
			e.Cancel()
			mainWindow.Hide()
		}
	})

	if cfgErr == nil && cfg.TrayEnabled {
		tray.ensureTray()
		// Rewritten on every start while the tray is on, so an entry with an
		// outdated format or pointing at a moved binary is brought up to date.
		if err := writeAutostartDesktopFile(); err != nil {
			fmt.Fprintln(os.Stderr, "aviso: não foi possível atualizar o autostart da bandeja:", err)
		}
	}

	if err := wailsApp.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "erro ao iniciar o Perci:", err)
		os.Exit(1)
	}
}
