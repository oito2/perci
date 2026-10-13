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
	"os"
	"slices"
	"strings"

	"github.com/oito2/perci/internal/executor"
)

// Probe answers the read-only system queries behind Action.Applied. It
// runs without privileges and caches the dpkg package list, so a whole
// profile is checked with a single dpkg-query call.
type Probe struct {
	ctx context.Context
	exe *executor.Executor

	debLoaded bool
	deb       map[string]bool

	componentsLoaded bool
	components       map[string]bool

	flatpakLoaded bool
	flatpakApps   map[string]bool
}

// NewProbe returns a Probe that runs its queries through exe.
func NewProbe(ctx context.Context, exe *executor.Executor) *Probe {
	return &Probe{ctx: ctx, exe: exe}
}

// Status reports, for every action of p, whether it counts as applied:
// detected on the system through its Applied func, or — for an action
// without one — listed in recorded (the IDs saved after a successful run).
func (p *Profile) Status(probe *Probe, recorded []string) map[string]bool {
	status := make(map[string]bool, len(p.Actions))
	for _, act := range p.Actions {
		if act.Applied != nil {
			status[act.ID] = act.Applied(probe)
		} else {
			status[act.ID] = slices.Contains(recorded, act.ID)
		}
	}
	return status
}

// Action returns the action of p with the given ID.
func (p *Profile) Action(id string) (Action, bool) {
	for _, act := range p.Actions {
		if act.ID == id {
			return act, true
		}
	}
	return Action{}, false
}

// debInstalled reports whether every name in pkgs is an installed dpkg
// package or is provided by one (virtual packages included).
func (p *Probe) debInstalled(pkgs ...string) bool {
	if !p.debLoaded {
		p.debLoaded = true
		out, err := p.exe.Output(p.ctx, executor.Options{}, "dpkg-query", "-W", "-f=${db:Status-Abbrev}|${Package}|${Provides}\n")
		if err == nil {
			p.deb = parseDpkgInstalled(out)
		}
	}
	for _, pkg := range pkgs {
		if !p.deb[pkg] {
			return false
		}
	}
	return true
}

// parseDpkgInstalled reads dpkg-query lines in the form
// "<status abbrev>|<package>|<provides>" and returns the names of the
// installed packages ("ii" status, or "hi" for a package on hold) plus
// every name they provide.
func parseDpkgInstalled(out string) map[string]bool {
	names := make(map[string]bool)
	for _, line := range strings.Split(out, "\n") {
		fields := strings.SplitN(line, "|", 3)
		if len(fields) != 3 || (!strings.HasPrefix(fields[0], "ii") && !strings.HasPrefix(fields[0], "hi")) {
			continue
		}
		names[fields[1]] = true
		for _, provided := range strings.Split(fields[2], ",") {
			// Each entry is "name" or "name (= version)".
			if name, _, _ := strings.Cut(strings.TrimSpace(provided), " "); name != "" {
				names[name] = true
			}
		}
	}
	return names
}

// rpmInstalled reports whether every package in pkgs is installed.
func (p *Probe) rpmInstalled(pkgs ...string) bool {
	_, err := p.exe.Output(p.ctx, executor.Options{}, "rpm", append([]string{"-q", "--"}, pkgs...)...)
	return err == nil
}

// ubuntuComponentsEnabled reports whether both universe and multiverse are
// present in the apt sources.
func (p *Probe) ubuntuComponentsEnabled() bool {
	if !p.componentsLoaded {
		p.componentsLoaded = true
		p.components = aptComponents()
	}
	return p.components["universe"] && p.components["multiverse"]
}

// sysctlConfigured reports whether perci's sysctl file exists.
func (p *Probe) sysctlConfigured() bool {
	_, err := os.Stat(sysctlConfPath)
	return err == nil
}

// swapActive reports whether the system has any active swap.
func (p *Probe) swapActive() bool {
	out, err := p.exe.Output(p.ctx, executor.Options{}, "swapon", "--noheadings", "--show=NAME")
	return err == nil && strings.TrimSpace(out) != ""
}

// trimEnabled reports whether fstrim.timer is enabled.
func (p *Probe) trimEnabled() bool {
	out, err := p.exe.Output(p.ctx, executor.Options{}, "systemctl", "is-enabled", "fstrim.timer")
	return err == nil && strings.TrimSpace(out) == "enabled"
}

// flatpakAppsInstalled reports whether the Flathub remote exists and every
// app in appIDs is installed, in either the user or the system installation.
func (p *Probe) flatpakAppsInstalled(appIDs ...string) bool {
	if !p.exe.CommandAvailable(p.ctx, "flatpak") {
		return false
	}
	out, err := p.exe.Output(p.ctx, executor.Options{}, "flatpak", "remotes", "--columns=name")
	if err != nil || !slices.Contains(strings.Fields(out), "flathub") {
		return false
	}
	if !p.flatpakLoaded {
		p.flatpakLoaded = true
		if list, err := p.exe.Output(p.ctx, executor.Options{}, "flatpak", "list", "--app", "--columns=application"); err == nil {
			p.flatpakApps = make(map[string]bool)
			for _, id := range strings.Fields(list) {
				p.flatpakApps[id] = true
			}
		}
	}
	for _, id := range appIDs {
		if !p.flatpakApps[id] {
			return false
		}
	}
	return true
}
