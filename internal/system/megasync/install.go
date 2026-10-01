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

package megasync

import (
	"context"
	"fmt"
	"io"
	"runtime"
	"strings"

	"github.com/oito2/perci/internal/distro"
	"github.com/oito2/perci/internal/executor"
	"github.com/oito2/perci/internal/ui"
)

// repoInfo describes MEGA's repository path segment for a distro/version
// (e.g. "xUbuntu_24.04", "Fedora_44") and whether it uses the APT (deb) or
// DNF (rpm) package format.
type repoInfo struct {
	path string
	rpm  bool
}

// Installed reports whether the megasync binary is on PATH.
func Installed(ctx context.Context, exe *executor.Executor) bool {
	return exe.CommandAvailable(ctx, "megasync")
}

// Install detects the running distribution and installs MegaSync from
// MEGA's official signed repository (mega.nz/linux/repo) — the GUI's
// "Instalar" button click is itself the confirmation, no prompt. Enforces
// an amd64-only check: automatic install is only available for amd64/x86_64
// packages.
func Install(ctx context.Context, exe *executor.Executor, stdout io.Writer) error {
	if runtime.GOARCH != "amd64" {
		return fmt.Errorf("arquitetura não suportada: %s — instale manualmente em https://mega.nz/sync", runtime.GOARCH)
	}
	return installCore(ctx, exe, stdout, distro.RawID(), distro.VersionID())
}

// installCore resolves MEGA's repository for the distro id/version and
// installs MegaSync from it, letting the system package manager verify the
// GPG signature (no bare .deb/.rpm download with no integrity check
// involved). id/ver are parameters so it's testable independent of the
// machine running the tests, like uninstall's family.
func installCore(ctx context.Context, exe *executor.Executor, stdout io.Writer, id, ver string) error {
	repo, err := resolveRepo(id, ver)
	if err != nil {
		ui.Err(stdout, err.Error())
		return err
	}

	ui.Info(stdout, "Adicionando repositório oficial do MegaSync e instalando...")
	sudo := executor.Options{RequiresSudo: true, Stdout: stdout, Stderr: stdout}
	var installErr error
	if repo.rpm {
		installErr = installFromDNF(ctx, exe, sudo, repo.path)
	} else {
		installErr = installFromAPT(ctx, exe, sudo, repo.path)
	}

	if installErr != nil {
		ui.Err(stdout, "Falha ao instalar MegaSync: "+installErr.Error())
		return fmt.Errorf("instalar megasync: %w", installErr)
	}

	ui.Success(stdout, "MegaSync instalado com sucesso.")
	return nil
}

// Repository files Install writes (and Uninstall removes).
const (
	aptKeyringDir = "/etc/apt/keyrings"
	aptKeyring    = "/etc/apt/keyrings/mega.nz.gpg"
	aptSourceList = "/etc/apt/sources.list.d/mega.nz.list"
	dnfRepoFile   = "/etc/yum.repos.d/megasync.repo"
)

// installFromAPT adds MEGA's signed APT repository for the given path (e.g.
// "xUbuntu_24.04") and installs megasync, following MEGA's documented setup
// (https://mega.nz/linux/repo/<path>/Release.key, dearmored into a keyring
// referenced via signed-by — the modern replacement for apt-key).
//
// Safe to re-run (reinstall, retry, install after uninstall): the key is
// dearmored with --batch --yes into a temp file and installed over the
// keyring — a plain `gpg --dearmor -o` over an existing file asks
// "Overwrite?" on /dev/tty, which fails without a terminal and aborted the
// whole script — and a failed download never leaves a truncated keyring.
func installFromAPT(ctx context.Context, exe *executor.Executor, sudo executor.Options, path string) error {
	return exe.Run(ctx, sudo, "bash", "-c", aptInstallScript(path))
}

func aptInstallScript(path string) string {
	return fmt.Sprintf(`
set -Eeuo pipefail
install -d -m 0755 %[2]s
tmp=$(mktemp)
trap 'rm -f -- "$tmp"' EXIT
curl -fsSL --connect-timeout 10 --max-time 600 "https://mega.nz/linux/repo/%[1]s/Release.key" | gpg --batch --yes --dearmor -o "$tmp"
install -m 0644 -- "$tmp" %[3]s
echo "deb [arch=amd64 signed-by=%[3]s] https://mega.nz/linux/repo/%[1]s/ ./" > %[4]s
apt-get update -qq
apt-get install -y megasync
`, path, aptKeyringDir, aptKeyring, aptSourceList)
}

// installFromDNF adds MEGA's signed YUM/DNF repository for the given path
// (e.g. "Fedora_44") and installs megasync.
func installFromDNF(ctx context.Context, exe *executor.Executor, sudo executor.Options, path string) error {
	return exe.Run(ctx, sudo, "bash", "-c", dnfInstallScript(path))
}

func dnfInstallScript(path string) string {
	return fmt.Sprintf(`
set -Eeuo pipefail
rpm --import "https://mega.nz/linux/repo/%s/repodata/repomd.xml.key"
printf '[MEGAsync]\nname=MEGAsync\nbaseurl=https://mega.nz/linux/repo/%s/\ngpgkey=https://mega.nz/linux/repo/%s/repodata/repomd.xml.key\ngpgcheck=1\nenabled=1\n' > %s
dnf install -y megasync
`, path, path, path, dnfRepoFile)
}

// resolveRepo maps the given distro id and version to MEGA's repository
// path segment. Accepting id and ver as parameters makes the logic testable
// without file I/O.
func resolveRepo(id, ver string) (repoInfo, error) {
	if id == "" {
		return repoInfo{}, fmt.Errorf(
			"não foi possível identificar a distribuição — verifique /etc/os-release",
		)
	}

	switch {
	case id == "linuxmint" && strings.HasPrefix(ver, "22"):
		// Mint 22.x, base Ubuntu 24.04
		return repoInfo{path: "xUbuntu_24.04"}, nil

	case (id == "ubuntu" && ver == "24.04") ||
		(id == "zorin" && strings.HasPrefix(ver, "18")):
		return repoInfo{path: "xUbuntu_24.04"}, nil

	case id == "ubuntu" && ver == "26.04":
		return repoInfo{path: "xUbuntu_26.04"}, nil

	case id == "fedora" && ver == "44":
		return repoInfo{path: "Fedora_44", rpm: true}, nil

	default:
		return repoInfo{}, fmt.Errorf(
			"distribuição não suportada para MegaSync: %s %s\nInstale manualmente em https://mega.nz/sync",
			id, ver,
		)
	}
}
