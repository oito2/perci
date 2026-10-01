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

package postinstall

import (
	"context"
	"io"

	"github.com/oito2/perci/internal/distro"
	"github.com/oito2/perci/internal/executor"
)

// Action is one independently-selectable post-install step — the
// granular unit behind the checklist UI in "Linux :: Pós-instalação".
// Each per-distro Profile (Mint, Ubuntu, Fedora, ZorinCore, ZorinLite,
// PopOS below) is just a list of these Actions, letting the GUI's
// checklist select and run any subset of them.
type Action struct {
	// ID is stable within a Profile — referenced by DependsOn and by the
	// GUI when the user submits their selection (cmd/prci-gui).
	ID          string
	Label       string
	Description string

	// DependsOn is another Action's ID in the same Profile, or "" for
	// none. The GUI only lets this action be checked once DependsOn is
	// checked, and unchecking DependsOn cascades to uncheck this action
	// too. Profile.Actions is always declared in dependency-safe order,
	// so running a Profile's actions in list order is always safe
	// regardless of which subset was selected.
	DependsOn string

	Run func(ctx context.Context, exe *executor.Executor, stdout io.Writer) error
}

// Profile is one supported OS/DE/version combination — 8 combos in
// total: Linux Mint Cinnamon, Linux Mint XFCE, Ubuntu 24.04, Ubuntu
// 26.04, Fedora 44 (KDE and GNOME share the same actions — no DE check,
// since the actions themselves don't depend on it), ZorinOS 18.1 Core,
// ZorinOS 18.1 Lite, Pop!_OS 24.04 COSMIC.
type Profile struct {
	ID      string // e.g. "mint-cinnamon", "ubuntu-2404" — stable, used by the GUI
	Label   string // e.g. "Linux Mint Cinnamon" — used in "Pós instalação do <Label>"
	Actions []Action
}

// DetectProfile returns the Profile matching the machine perci is
// running on, or nil when it's not one of the officially supported
// combinations — the GUI shows "Seu Sistema Operacional não é suportado
// pelo Perci" in that case.
//
// Fedora and ZorinOS don't check VersionID here — only their DE (for
// ZorinOS) or nothing at all (for Fedora) — because their post-install
// actions don't vary by exact version, only Ubuntu's do (24.04 vs
// 26.04).
func DetectProfile() *Profile {
	id := distro.RawID()
	de := distro.DetectDE()
	ver := distro.VersionID()

	switch id {
	case "linuxmint":
		switch de {
		case "cinnamon":
			return &mintCinnamonProfile
		case "xfce":
			return &mintXFCEProfile
		}
	case "ubuntu":
		switch ver {
		case "24.04":
			return &ubuntu2404Profile
		case "26.04":
			return &ubuntu2604Profile
		}
	case "fedora":
		return &fedoraProfile
	case "zorin":
		switch de {
		case "gnome":
			return &zorinCoreProfile
		case "xfce":
			return &zorinLiteProfile
		}
	case "pop":
		if de == "cosmic" && ver == "24.04" {
			return &poposProfile
		}
	}
	return nil
}
