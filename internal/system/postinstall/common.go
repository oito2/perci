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
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/oito2/perci/internal/config"
	"github.com/oito2/perci/internal/distro"
	"github.com/oito2/perci/internal/executor"
	"github.com/oito2/perci/internal/system/apps"
	"github.com/oito2/perci/internal/ui"
)

// step prints a step panel and runs a sudo command.
func step(ctx context.Context, exe *executor.Executor, stdout io.Writer, msg, name string, args ...string) error {
	ui.Info(stdout, msg)
	return exe.Run(ctx, executor.Options{RequiresSudo: true, Stdout: stdout, Stderr: stdout}, name, args...)
}

// aptInstall installs one or more apt packages with sudo, non-interactively
// (distro.AptNonInteractiveOpts).
func aptInstall(ctx context.Context, exe *executor.Executor, stdout io.Writer, pkgs ...string) error {
	args := distro.AptArgs(append([]string{"install", "-y", "--"}, pkgs...)...)
	return exe.Run(ctx, executor.Options{
		RequiresSudo: true,
		Stdout:       stdout,
		Stderr:       stdout,
		Env:          distro.AptEnv,
	}, "apt-get", args...)
}

// dnfInstall installs one or more dnf packages with sudo.
func dnfInstall(ctx context.Context, exe *executor.Executor, stdout io.Writer, pkgs ...string) error {
	args := append([]string{"install", "-y", "--"}, pkgs...)
	return exe.Run(ctx, executor.Options{RequiresSudo: true, Stdout: stdout, Stderr: stdout}, "dnf", args...)
}

// ensureFlatpakReady installs flatpak if needed and ensures the Flathub
// remote, via apps.EnsureFlatpak.
func ensureFlatpakReady(ctx context.Context, exe *executor.Executor, stdout io.Writer) error {
	return apps.EnsureFlatpak(ctx, exe, stdout)
}

// flatpakSetupAndInstall makes sure flatpak and Flathub are there and
// installs appIDs. For the system scope the setup and the install run as
// ONE privileged batch (a single password prompt).
func flatpakSetupAndInstall(ctx context.Context, exe *executor.Executor, stdout io.Writer, appIDs ...string) error {
	if config.FlatpakFlag() != "--system" {
		if err := ensureFlatpakReady(ctx, exe, stdout); err != nil {
			return err
		}
		return flatpakInstall(ctx, exe, stdout, appIDs...)
	}
	steps, _, err := apps.FlatpakSetupSteps(ctx, exe)
	if err != nil {
		return err
	}
	steps = append(steps, executor.PrivilegedStep{
		Announce: "Instalando " + strings.Join(appIDs, ", ") + "...",
		Env:      []string{"TERM=dumb"},
		Name:     "flatpak",
		Args:     append([]string{"install", "--noninteractive", "--system", "-y", "flathub"}, appIDs...),
	})
	return exe.RunSudoSequence(ctx, executor.Options{Stdout: stdout, Stderr: stdout}, steps)
}

// flatpakInstall installs Flatpak apps from Flathub using the configured
// scope, privileged for --system.
func flatpakInstall(ctx context.Context, exe *executor.Executor, stdout io.Writer, appIDs ...string) error {
	scope := config.FlatpakFlag()
	args := append([]string{"install", "--noninteractive", scope, "-y", "flathub"}, appIDs...)
	return exe.Run(ctx, executor.Options{RequiresSudo: scope == "--system", Stdout: stdout, Stderr: stdout, Env: []string{"TERM=dumb"}}, "flatpak", args...)
}

const (
	sysctlConfPath       = "/etc/sysctl.d/99-perci.conf"
	legacySysctlConfPath = "/etc/sysctl.d/99-lumina.conf"
)

// configureSysctl sets swappiness, inotify and applies sysctl — both
// commands run as ONE sudo/pkexec authentication.
//
// A 99-lumina.conf file is removed when present, so it never applies
// alongside the perci one.
func configureSysctl(ctx context.Context, exe *executor.Executor, stdout io.Writer) error {
	ui.Info(stdout, "Aplicando configurações de kernel (sysctl)...")
	conf := "vm.swappiness=10\nfs.inotify.max_user_watches=524288\n"
	path := sysctlConfPath
	script := fmt.Sprintf(`set -Eeuo pipefail
printf '%%s' %s | tee %s > /dev/null
rm -f -- %s
sysctl -p %s
`, executor.ShellQuote(conf), executor.ShellQuote(path), executor.ShellQuote(legacySysctlConfPath), executor.ShellQuote(path))
	return exe.Run(ctx, executor.Options{RequiresSudo: true, Stdout: stdout, Stderr: stdout}, "bash", "-c", script)
}

// acceptEulaAndAptInstall pre-accepts the ttf-mscorefonts-installer EULA and
// installs pkgs as ONE sudo/pkexec authentication. The EULA selection is
// piped to debconf-set-selections from inside a "bash -c" step, because
// PrivilegedStep has no Stdin.
func acceptEulaAndAptInstall(ctx context.Context, exe *executor.Executor, stdout io.Writer, pkgs ...string) error {
	const eulaSelection = "ttf-mscorefonts-installer msttcorefonts/accepted-mscorefonts-eula select true"
	pipeEula := "printf '%s\\n' " + executor.ShellQuote(eulaSelection) + " | debconf-set-selections"
	installArgs := distro.AptArgs(append([]string{"install", "-y", "--"}, pkgs...)...)
	return exe.RunSudoSequence(ctx, executor.Options{Stdout: stdout, Stderr: stdout}, []executor.PrivilegedStep{
		{Announce: "Pré-aceitando EULA das fontes Microsoft...", Name: "bash", Args: []string{"-c", pipeEula}},
		{
			Announce: "Instalando " + strings.Join(pkgs, ", ") + "...",
			Env:      distro.AptEnv,
			Name:     "apt-get", Args: installArgs,
		},
	})
}

// Locations of the apt sources read by aptComponents; variables so tests can
// point them at a temporary tree.
var (
	aptSourcesList = "/etc/apt/sources.list"
	aptSourcesDir  = "/etc/apt/sources.list.d"
)

// aptComponents returns the set of components (e.g. "universe") that at
// least one enabled "deb" source uses. It reads the legacy one-line files
// (*.list) and the DEB822 files (*.sources) once; an unreadable file is
// skipped, so a read error only ever makes a component look disabled.
func aptComponents() map[string]bool {
	components := make(map[string]bool)
	files := []string{aptSourcesList}
	for _, pattern := range []string{"*.list", "*.sources"} {
		if m, err := filepath.Glob(filepath.Join(aptSourcesDir, pattern)); err == nil {
			files = append(files, m...)
		}
	}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		if strings.HasSuffix(f, ".sources") {
			parseDeb822Components(string(data), components)
		} else {
			parseListComponents(string(data), components)
		}
	}
	return components
}

// parseListComponents adds to out the components of every "deb" line of a
// one-line-style sources file ("deb [opts] uri suite comp1 comp2...").
func parseListComponents(text string, out map[string]bool) {
	for _, line := range strings.Split(text, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || fields[0] != "deb" {
			continue
		}
		fields = fields[1:]
		if len(fields) > 0 && strings.HasPrefix(fields[0], "[") {
			for len(fields) > 0 && !strings.HasSuffix(fields[0], "]") {
				fields = fields[1:]
			}
			if len(fields) > 0 {
				fields = fields[1:]
			}
		}
		if len(fields) > 2 { // uri, suite, then the components
			for _, c := range fields[2:] {
				if strings.HasPrefix(c, "#") {
					break
				}
				out[c] = true
			}
		}
	}
}

// parseDeb822Components adds to out the components of every enabled
// stanza of a DEB822 sources file whose Types include "deb".
func parseDeb822Components(text string, out map[string]bool) {
	for _, stanza := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n\n") {
		var types, comps []string
		enabled := true
		for _, line := range strings.Split(stanza, "\n") {
			key, value, ok := strings.Cut(line, ":")
			if !ok || strings.HasPrefix(line, "#") {
				continue
			}
			value = strings.TrimSpace(value)
			switch strings.ToLower(strings.TrimSpace(key)) {
			case "types":
				types = strings.Fields(value)
			case "components":
				comps = strings.Fields(value)
			case "enabled":
				switch strings.ToLower(value) {
				case "no", "false", "0":
					enabled = false
				}
			}
		}
		if enabled && slices.Contains(types, "deb") {
			for _, c := range comps {
				out[c] = true
			}
		}
	}
}

// aptSourceEnabled reports whether an enabled "deb" source (one-line *.list
// or DEB822 *.sources in aptSourcesDir) has a URI containing fragment.
func aptSourceEnabled(fragment string) bool {
	for _, pattern := range []string{"*.list", "*.sources"} {
		files, _ := filepath.Glob(filepath.Join(aptSourcesDir, pattern))
		for _, f := range files {
			data, err := os.ReadFile(f)
			if err != nil {
				continue
			}
			var uris []string
			if strings.HasSuffix(f, ".sources") {
				uris = deb822URIs(string(data))
			} else {
				uris = listURIs(string(data))
			}
			if slices.ContainsFunc(uris, func(u string) bool { return strings.Contains(u, fragment) }) {
				return true
			}
		}
	}
	return false
}

// listURIs returns the URI of every "deb" line of a one-line-style sources
// file.
func listURIs(text string) []string {
	var uris []string
	for _, line := range strings.Split(text, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] != "deb" {
			continue
		}
		rest := fields[1:]
		if strings.HasPrefix(rest[0], "[") {
			for len(rest) > 0 && !strings.HasSuffix(rest[0], "]") {
				rest = rest[1:]
			}
			if len(rest) < 2 {
				continue
			}
			rest = rest[1:]
		}
		uris = append(uris, rest[0])
	}
	return uris
}

// deb822URIs returns the URIs of every enabled "deb" stanza of a DEB822
// sources file.
func deb822URIs(text string) []string {
	var uris []string
	for _, stanza := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n\n") {
		var types, stanzaURIs []string
		enabled := true
		for _, line := range strings.Split(stanza, "\n") {
			key, value, ok := strings.Cut(line, ":")
			if !ok || strings.HasPrefix(line, "#") || strings.HasPrefix(line, " ") {
				continue
			}
			value = strings.TrimSpace(value)
			switch strings.ToLower(strings.TrimSpace(key)) {
			case "types":
				types = strings.Fields(value)
			case "uris":
				stanzaURIs = strings.Fields(value)
			case "enabled":
				switch strings.ToLower(value) {
				case "no", "false", "0":
					enabled = false
				}
			}
		}
		if enabled && slices.Contains(types, "deb") {
			uris = append(uris, stanzaURIs...)
		}
	}
	return uris
}

// rpmFusionMissing returns the RPM Fusion release packages (free, nonfree)
// that are not installed yet.
func rpmFusionMissing(ctx context.Context, exe *executor.Executor) []string {
	var missing []string
	for _, flavor := range []string{"free", "nonfree"} {
		if _, err := exe.Output(ctx, executor.Options{}, "rpm", "-q", "--", "rpmfusion-"+flavor+"-release"); err != nil {
			missing = append(missing, flavor)
		}
	}
	return missing
}

func stripNewline(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}

// setupSwapfile creates a 4 GB swapfile at /swapfile and persists it in
// /etc/fstab, unless the system already has any active swap. On btrfs the
// file is made with `btrfs filesystem mkswapfile`. A failure at any point
// removes the file again (and deactivates it), so a later run is not fooled
// into "already exists", and is returned as an error.
func setupSwapfile(ctx context.Context, exe *executor.Executor, stdout io.Writer) error {
	ui.Info(stdout, "Configurando swapfile...")
	return exe.Run(ctx, executor.Options{RequiresSudo: true, Stdout: stdout, Stderr: stdout}, "bash", "-c", swapfileScript)
}

// swapfileScript is setupSwapfile's privileged part — one password prompt.
// The fstab entry is matched only as an uncommented first field; when it
// cannot be written the swap is deactivated and the file removed.
const swapfileScript = `set -Eeuo pipefail
if [ -L /swapfile ]; then
  echo "/swapfile é um link simbólico — abortando." >&2
  exit 1
fi
if [ -n "$(swapon --noheadings --show=NAME 2>/dev/null)" ]; then
  echo "O sistema já tem swap ativo — nada a fazer."
  exit 0
fi
if [ -e /swapfile ]; then
  echo "/swapfile já existe, mas não está ativo — verifique-o manualmente." >&2
  exit 1
fi
if [ "$(findmnt -no FSTYPE / 2>/dev/null || true)" = btrfs ]; then
  btrfs filesystem mkswapfile --size 4g /swapfile || { echo "Falha ao criar swapfile (btrfs)." >&2; rm -f -- /swapfile; exit 1; }
else
  fallocate -l 4G /swapfile || { echo "Falha ao criar swapfile." >&2; rm -f -- /swapfile; exit 1; }
  chmod 600 /swapfile || { echo "Falha ao definir permissões do swapfile." >&2; rm -f -- /swapfile; exit 1; }
  mkswap /swapfile || { echo "Falha ao formatar swapfile." >&2; rm -f -- /swapfile; exit 1; }
fi
swapon /swapfile || { echo "Falha ao ativar swapfile." >&2; rm -f -- /swapfile; exit 1; }
grep -qE '^[[:space:]]*/swapfile[[:space:]]' /etc/fstab || printf '/swapfile none swap sw 0 0\n' >> /etc/fstab \
  || { echo "Falha ao atualizar /etc/fstab." >&2; swapoff /swapfile || true; rm -f -- /swapfile; exit 1; }
echo "Swapfile de 4 GB criado e ativado."
`
