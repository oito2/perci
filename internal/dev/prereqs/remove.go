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

package prereqs

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/oito2/perci/internal/distro"
	"github.com/oito2/perci/internal/executor"
)

// RemovePackages removes pkgs with the package manager of family, but only
// when the removal takes nothing else with it (see CheckRemoval). The
// removal itself runs as one privileged command.
func RemovePackages(ctx context.Context, exe *executor.Executor, opts executor.Options, family string, pkgs ...string) error {
	if err := CheckRemoval(ctx, exe, family, pkgs...); err != nil {
		return err
	}
	switch family {
	case distro.Debian:
		opts.Env = append(slices.Clone(opts.Env), distro.AptEnv...)
		return exe.Run(ctx, opts, "apt-get", distro.AptArgs(append([]string{"purge", "-y", "--"}, pkgs...)...)...)
	case distro.Fedora:
		return exe.Run(ctx, opts, "dnf", append([]string{"remove", "-y", "--no-autoremove", "--"}, pkgs...)...)
	}
	return fmt.Errorf("unsupported distro family: %s", family)
}

// CheckRemoval simulates removing pkgs, unprivileged, and fails when the
// transaction lists any package besides pkgs.
func CheckRemoval(ctx context.Context, exe *executor.Executor, family string, pkgs ...string) error {
	var extra []string
	switch family {
	case distro.Debian:
		sim, err := exe.Output(ctx, executor.Options{Env: []string{"LC_ALL=C"}}, "apt-get", append([]string{"-s", "purge", "--"}, pkgs...)...)
		if err != nil {
			return fmt.Errorf("simular a remoção de %s: %w", strings.Join(pkgs, ", "), err)
		}
		extra = extraAptRemovals(sim, pkgs)
	case distro.Fedora:
		// dnf exits non-zero when --assumeno aborts the transaction, so
		// the exit status is ignored and only the printed plan is read.
		sim, err := exe.Output(ctx, executor.Options{Env: []string{"LC_ALL=C"}}, "sh", append([]string{"-c",
			`dnf remove --assumeno --no-autoremove -- "$@" 2>&1 || true`, "sh"}, pkgs...)...)
		if err != nil {
			return fmt.Errorf("simular a remoção de %s: %w", strings.Join(pkgs, ", "), err)
		}
		if !exe.DryRun && !dnfPlanPrinted(sim) {
			return fmt.Errorf("não foi possível simular a remoção de %s — nada foi removido:\n%s", strings.Join(pkgs, ", "), strings.TrimSpace(sim))
		}
		extra = extraDnfRemovals(sim, pkgs)
	default:
		return fmt.Errorf("unsupported distro family: %s", family)
	}
	if len(extra) > 0 {
		return fmt.Errorf("remover %s também removeria: %s — nada foi removido; se tiver certeza, remova manualmente pelo terminal",
			strings.Join(pkgs, ", "), strings.Join(extra, ", "))
	}
	return nil
}

// extraAptRemovals lists the packages on the "Remv"/"Purg" lines of
// `apt-get -s` output that are not in pkgs.
func extraAptRemovals(sim string, pkgs []string) []string {
	var extra []string
	for _, line := range strings.Split(sim, "\n") {
		f := strings.Fields(line)
		if len(f) < 2 || (f[0] != "Remv" && f[0] != "Purg") {
			continue
		}
		if name := strings.SplitN(f[1], ":", 2)[0]; !slices.Contains(pkgs, name) && !slices.Contains(extra, name) {
			extra = append(extra, name)
		}
	}
	return extra
}

// dnfPlanPrinted reports whether `dnf remove --assumeno` output contains a
// transaction plan (or says there is nothing to do), as opposed to an error
// that printed no plan at all.
func dnfPlanPrinted(sim string) bool {
	return strings.Contains(sim, "Transaction Summary") || strings.Contains(sim, "Nothing to do")
}

// extraDnfRemovals lists the packages in the "Removing..." tables of
// `dnf remove --assumeno` output that are not in pkgs. A table row starts
// with exactly one space.
func extraDnfRemovals(sim string, pkgs []string) []string {
	var extra []string
	inTable := false
	for _, line := range strings.Split(sim, "\n") {
		switch {
		case strings.HasPrefix(line, "Removing") && strings.HasSuffix(strings.TrimSpace(line), ":"):
			inTable = true
		case strings.HasPrefix(line, "Transaction Summary"):
			inTable = false
		case inTable && len(line) > 1 && line[0] == ' ' && line[1] != ' ':
			if name := strings.Fields(line)[0]; !slices.Contains(pkgs, name) && !slices.Contains(extra, name) {
				extra = append(extra, name)
			}
		}
	}
	return extra
}
