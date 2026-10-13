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

// Action is one post-install step, applied on its own from the
// "Linux :: Pós-instalação" screen. Each per-distro Profile (Mint, Ubuntu,
// Fedora, ZorinCore, ZorinLite, PopOS below) is just a list of these
// Actions.
type Action struct {
	// ID is stable within a Profile — referenced by DependsOn and by the
	// GUI when the user submits their selection (cmd/prci-gui).
	ID          string
	Label       string
	Description string

	// DependsOn is another Action's ID in the same Profile, or "" for
	// none. This action can only be applied once DependsOn counts as
	// applied (Profile.Status). Profile.Actions is declared in
	// dependency order.
	DependsOn string

	// Applied detects on the system whether this action's result is
	// already in place. nil means the action leaves no state to detect
	// (e.g. refreshing the package index): its applied state is the
	// record saved after a successful run.
	Applied func(p *Probe) bool

	Run func(ctx context.Context, exe *executor.Executor, stdout io.Writer) error
}

// Profile is one supported OS/DE/version combination — 11 combos in
// total: Linux Mint Cinnamon, Linux Mint XFCE, Ubuntu 24.04, Ubuntu 26.04,
// Ubuntu Budgie 24.04, Ubuntu Budgie 26.04, Fedora 44 (KDE and GNOME share
// the same actions), Fedora 44 Budgie, ZorinOS 18.1 Core, ZorinOS 18.1
// Lite, Pop!_OS 24.04 COSMIC.
type Profile struct {
	ID      string // e.g. "mint-cinnamon", "ubuntu-2404" — stable, used by the GUI
	Label   string // e.g. "Linux Mint Cinnamon" — used in "Pós instalação do <Label>"
	Actions []Action
}

// DetectProfile returns the Profile matching the machine perci is
// running on, or nil when it's not one of the officially supported
// combinations — the GUI shows "Seu Sistema Operacional não é suportado
// pelo Perci" in that case.
func DetectProfile() *Profile {
	return detectProfile(distro.RawID(), distro.DetectDE(), distro.VersionID())
}

// detectProfile picks the Profile for os-release ID id, desktop token de
// (distro.DetectDE) and os-release VERSION_ID ver.
//
// Fedora and ZorinOS don't check the version — only their DE (Budgie on
// Fedora, Core/Lite on ZorinOS) — because their post-install actions don't
// vary by exact version; Ubuntu's do (24.04 vs 26.04). On Ubuntu, Budgie
// gets its own profiles and every other desktop the GNOME ones.
func detectProfile(id, de, ver string) *Profile {
	switch id {
	case "linuxmint":
		switch de {
		case "cinnamon":
			return &mintCinnamonProfile
		case "xfce":
			return &mintXFCEProfile
		}
	case "ubuntu":
		switch {
		case de == "budgie" && ver == "24.04":
			return &ubuntuBudgie2404Profile
		case de == "budgie" && ver == "26.04":
			return &ubuntuBudgie2604Profile
		case ver == "24.04":
			return &ubuntu2404Profile
		case ver == "26.04":
			return &ubuntu2604Profile
		}
	case "fedora":
		if de == "budgie" {
			return &fedoraBudgieProfile
		}
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
