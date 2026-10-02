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

package distro

import (
	"context"
	"fmt"
	"io"
	"net/url"

	"github.com/oito2/perci/internal/executor"
)

// SignedRepo describes a GPG-signed APT/DNF repository to add (skipping
// the add step if the repo file already exists) before installing PkgName
// from it.
type SignedRepo struct {
	// KeyringPath is where the dearmored GPG keyring is written on
	// Debian — ex. "/usr/share/keyrings/vscode.gpg".
	KeyringPath string
	// KeyURL serves an ASCII-armored GPG key, piped through `gpg
	// --dearmor` into KeyringPath.
	KeyURL string

	// AptListPath is the apt sources file to write on Debian — either a
	// legacy one-line "*.list" (a "deb [...] ..." line) or a DEB822
	// "*.sources" stanza; AptListContent's shape decides which. Must
	// include its own trailing newline.
	AptListPath    string
	AptListContent string

	// DnfRepoPath/DnfRepoBody are the ".repo" file written on Fedora
	// (INI-style, ex. "[code]\nname=...\n..."). DnfRepoBody must include
	// its own trailing newline.
	DnfRepoPath string
	DnfRepoBody string

	PkgName string
}

// InstallFromSignedRepo adds r (idempotent — skips re-adding the repo if
// its list/repo file already exists) and installs r.PkgName from it. The
// keyring only counts as present when non-empty, and is downloaded to a
// temp file first, so a failed download never leaves an empty keyring
// behind.
func InstallFromSignedRepo(ctx context.Context, exe *executor.Executor, stdout io.Writer, family string, r SignedRepo) error {
	if err := requireHTTPS(r.KeyURL); err != nil {
		return fmt.Errorf("chave de assinatura do repositório: %w", err)
	}
	sudo := executor.Options{RequiresSudo: true, Stdout: stdout, Stderr: stdout}
	switch family {
	case Debian:
		script := fmt.Sprintf(`set -Eeuo pipefail
if [ ! -s %[1]s ]; then
  tmp=$(mktemp)
  trap 'rm -f -- "$tmp"' EXIT
  wget -qO- %[2]s | gpg --batch --yes --dearmor -o "$tmp"
  install -D -m 0644 -- "$tmp" %[1]s
fi
if [ ! -s %[3]s ]; then
  printf '%%s' %[4]s | tee %[3]s > /dev/null
fi
apt-get update -qq
apt-get install -y %[5]s
`,
			executor.ShellQuote(r.KeyringPath),
			executor.ShellQuote(r.KeyURL),
			executor.ShellQuote(r.AptListPath),
			executor.ShellQuote(r.AptListContent),
			executor.ShellQuote(r.PkgName),
		)
		return exe.Run(ctx, sudo, "bash", "-c", script)
	case Fedora:
		script := fmt.Sprintf(`set -Eeuo pipefail
rpm --import %[1]s
[ -s %[2]s ] || printf '%%s' %[3]s | tee %[2]s > /dev/null
dnf install -y %[4]s
`,
			executor.ShellQuote(r.KeyURL),
			executor.ShellQuote(r.DnfRepoPath),
			executor.ShellQuote(r.DnfRepoBody),
			executor.ShellQuote(r.PkgName),
		)
		return exe.Run(ctx, sudo, "bash", "-c", script)
	default:
		return UnsupportedFamilyError(family)
	}
}

// requireHTTPS rejects a KeyURL that isn't https — the trust of the whole
// "signed repo" model rests on this key being fetched over a channel an
// on-path attacker can't tamper with.
func requireHTTPS(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("URL inválida: %w", err)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("URL da chave de assinatura deve usar https, obtido %q: %s", u.Scheme, rawURL)
	}
	return nil
}
