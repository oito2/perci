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
// MEGA's official signed repository, without a confirmation prompt. Only
// amd64/x86_64 is supported; other architectures return an error.
func Install(ctx context.Context, exe *executor.Executor, stdout io.Writer) error {
	if runtime.GOARCH != "amd64" {
		return fmt.Errorf("arquitetura não suportada: %s — instale manualmente em https://mega.nz/sync", runtime.GOARCH)
	}
	return installCore(ctx, exe, stdout, distro.RawID(), distro.VersionID())
}

// installCore resolves MEGA's repository for the distro id/version and
// installs MegaSync from it, letting the system package manager verify the
// GPG signature. id and ver are parameters rather than read from the
// machine.
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
// "xUbuntu_24.04") and installs megasync. The repository key is dearmored
// into a keyring referenced via signed-by.
//
// Safe to re-run: the key is dearmored with --batch --yes into a temp file
// and installed over the keyring, so an existing keyring never triggers an
// overwrite prompt and a failed download never leaves a truncated keyring.
func installFromAPT(ctx context.Context, exe *executor.Executor, sudo executor.Options, path string) error {
	return exe.Run(ctx, sudo, "bash", "-c", aptInstallScript(path))
}

// megaKeyFingerprint is the fingerprint of the primary key MEGA signs its
// repositories with; any other key downloaded from the repository is
// refused.
const megaKeyFingerprint = "B01C811880480C854C73EC7E1A664B787094A482"

// rpmKeyFile is where installFromDNF keeps the verified MEGA key, which the
// repository file references instead of a network URL.
const rpmKeyFile = "/etc/pki/rpm-gpg/RPM-GPG-KEY-megasync"

// keyCheckScript defines verify_mega_key, which fails unless the first key
// of the given file has megaKeyFingerprint. gpg runs with a throwaway home so
// no keyring of the invoking user is touched.
const keyCheckScript = `GNUPGHOME=$(mktemp -d)
export GNUPGHOME
verify_mega_key() {
  local got
  got=$(gpg --batch --show-keys --with-colons -- "$1" | awk -F: '$1=="fpr"{print $10; exit}') || got=""
  if [ "$got" != "` + megaKeyFingerprint + `" ]; then
    echo "chave do repositório MEGA inesperada: ${got:-nenhuma}" >&2
    exit 1
  fi
}
`

func aptInstallScript(path string) string {
	return fmt.Sprintf(`
set -Eeuo pipefail
%[5]s
tmp=$(mktemp)
trap 'rm -rf -- "$tmp" "$tmp.gpg" "$GNUPGHOME"' EXIT
install -d -m 0755 %[2]s
curl -fsSL --connect-timeout 10 --max-time 600 -o "$tmp" "https://mega.nz/linux/repo/%[1]s/Release.key"
verify_mega_key "$tmp"
gpg --batch --yes --dearmor -o "$tmp.gpg" "$tmp"
install -m 0644 -- "$tmp.gpg" %[3]s
echo "deb [arch=amd64 signed-by=%[3]s] https://mega.nz/linux/repo/%[1]s/ ./" > %[4]s
export DEBIAN_FRONTEND=noninteractive
apt-get %[6]s update -qq
apt-get %[6]s install -y megasync
`, path, aptKeyringDir, aptKeyring, aptSourceList, keyCheckScript, strings.Join(distro.AptNonInteractiveOpts, " "))
}

// installFromDNF adds MEGA's signed YUM/DNF repository for the given path
// (e.g. "Fedora_44") and installs megasync.
func installFromDNF(ctx context.Context, exe *executor.Executor, sudo executor.Options, path string) error {
	return exe.Run(ctx, sudo, "bash", "-c", dnfInstallScript(path))
}

func dnfInstallScript(path string) string {
	return fmt.Sprintf(`
set -Eeuo pipefail
%[4]s
tmp=$(mktemp)
trap 'rm -rf -- "$tmp" "$GNUPGHOME"' EXIT
curl -fsSL --connect-timeout 10 --max-time 600 -o "$tmp" "https://mega.nz/linux/repo/%[1]s/repodata/repomd.xml.key"
verify_mega_key "$tmp"
install -m 0644 -- "$tmp" %[3]s
rpm --import %[3]s
printf '[MEGAsync]\nname=MEGAsync\nbaseurl=https://mega.nz/linux/repo/%[1]s/\ngpgkey=file://%[3]s\ngpgcheck=1\nenabled=1\n' > %[2]s
dnf install -y megasync
`, path, dnfRepoFile, rpmKeyFile, keyCheckScript)
}

// resolveRepo maps the given distro id and version to MEGA's repository
// path segment.
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
