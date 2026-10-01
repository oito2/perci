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

package executor

import (
	"context"
	"fmt"
	"regexp"
)

var sha256Hex = regexp.MustCompile(`^[0-9a-fA-F]{64}$`)

// verifyStagedScript checks $2 against the SHA-256 in $1 and deletes $2 on
// mismatch. Positional parameters keep both values out of the script text.
const verifyStagedScript = `printf '%s  %s\n' "$1" "$2" | sha256sum -c --status - || { rm -f -- "$2"; echo "checksum não confere: $2" >&2; exit 1; }`

// PrivilegedInstallSteps returns the privileged steps that install src at
// dest owned by root:root with mode, without trusting src after the prompt.
//
// src usually lives somewhere the unprivileged user can write (a download
// in /tmp, a temp file next to the binary), and it was verified before the
// pkexec dialog — which can stay open for as long as the user takes to type
// the password. Moving src by path would let any process of that same user
// swap the file in that window and end up with root-owned code of its
// choosing (and plain `mv` would also keep the user as the file's owner).
// Instead root copies src into a staged file it owns, mode 0600 so a
// swapped-in secret is never readable, re-verifies the SHA-256 on that
// copy, and only then sets the final mode and renames it over dest.
func PrivilegedInstallSteps(src, dest, sha256, mode string) ([]PrivilegedStep, error) {
	if !sha256Hex.MatchString(sha256) {
		return nil, fmt.Errorf("checksum SHA-256 inválido: %q", sha256)
	}
	staged := dest + ".perci-new"
	return []PrivilegedStep{
		{Name: "install", Args: []string{"-T", "-m", "0600", "-o", "root", "-g", "root", "--", src, staged}},
		{Name: "sh", Args: []string{"-c", verifyStagedScript, "sh", sha256, staged}},
		{Name: "chmod", Args: []string{mode, "--", staged}},
		{Name: "mv", Args: []string{"-f", "-T", "--", staged, dest}},
	}, nil
}

// PrivilegedInstall runs PrivilegedInstallSteps as one privileged batch
// (a single password prompt).
func (e *Executor) PrivilegedInstall(ctx context.Context, opts Options, src, dest, sha256, mode string) error {
	steps, err := PrivilegedInstallSteps(src, dest, sha256, mode)
	if err != nil {
		return err
	}
	return e.RunSudoSequence(ctx, opts, steps)
}
