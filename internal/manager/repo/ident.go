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
	"fmt"
	"io"
	"strings"

	"github.com/oito2/perci/internal/executor"
)

// gitDirArgs prefixes args with "-C dir" when dir is non-empty — shared by
// GetCurrentLocalIdentity/ApplyLocalIdentityAt below, which used to each
// define their own identical closure for this.
func gitDirArgs(dir string, args ...string) []string {
	if dir == "" {
		return args
	}
	return append([]string{"-C", dir}, args...)
}

// GetCurrentLocalIdentity returns the local git user.name and user.email configured in dir.
func GetCurrentLocalIdentity(ctx context.Context, exe *executor.Executor, dir string) (string, string) {
	name, _ := exe.Output(ctx, executor.Options{}, "git", gitDirArgs(dir, "config", "--local", "user.name")...)
	email, _ := exe.Output(ctx, executor.Options{}, "git", gitDirArgs(dir, "config", "--local", "user.email")...)
	return strings.TrimSpace(name), strings.TrimSpace(email)
}

// ApplyLocalIdentityAt applies local user.name, user.email and credential.helper in dir.
func ApplyLocalIdentityAt(ctx context.Context, exe *executor.Executor, stdout io.Writer, name, email, dir string) error {
	if name == "" || email == "" {
		return fmt.Errorf("nome e e-mail não podem ser vazios")
	}

	cred := resolveCredHelper(ctx, exe)

	opts := executor.Options{Stdout: stdout, Stderr: stdout}
	for _, args := range [][]string{
		// "--" before the key: a value starting with "-" is never read as
		// an option.
		{"config", "--local", "--", "user.name", name},
		{"config", "--local", "--", "user.email", email},
		{"config", "--local", "--", "credential.helper", cred},
	} {
		if err := exe.Run(ctx, opts, "git", gitDirArgs(dir, args...)...); err != nil {
			return fmt.Errorf("git %v: %w", args, err)
		}
	}
	return nil
}
