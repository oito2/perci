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

package repo

import (
	"context"
	"os"

	"github.com/oito2/perci/internal/executor"
)

// libsecret binary locations across distros.
var libsecretPaths = []string{
	"/usr/share/doc/git/contrib/credential/libsecret/git-credential-libsecret",
	"/usr/lib/git-core/git-credential-libsecret",
	"/usr/libexec/git-core/git-credential-libsecret",
	"/usr/lib/git/git-credential-libsecret",
}

func resolveCredHelper(ctx context.Context, exe *executor.Executor) string {
	for _, p := range libsecretPaths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	if exe.CommandAvailable(ctx, "git-credential-libsecret") {
		return "libsecret"
	}
	return "cache"
}
