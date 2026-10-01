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

// ConfigureGlobal sets global git user.name, user.email and
// credential.helper — same checks as ApplyLocalIdentityAt: both values
// required, and "--" before each key so a value starting with "-" is never
// read as an option. git's own output goes to stdout (the GUI's terminal).
func ConfigureGlobal(ctx context.Context, exe *executor.Executor, stdout io.Writer, name, email string) error {
	if name == "" || email == "" {
		return fmt.Errorf("nome e e-mail não podem ser vazios")
	}
	cred := resolveCredHelper(ctx, exe)

	opts := executor.Options{Stdout: stdout, Stderr: stdout}
	for _, args := range [][]string{
		{"config", "--global", "--", "user.name", name},
		{"config", "--global", "--", "user.email", email},
		{"config", "--global", "--", "credential.helper", cred},
	} {
		if err := exe.Run(ctx, opts, "git", args...); err != nil {
			return fmt.Errorf("git %v: %w", args, err)
		}
	}
	return nil
}

// GetGlobalIdentity returns the current configured global user.name and user.email.
func GetGlobalIdentity(ctx context.Context, exe *executor.Executor) (string, string) {
	name, _ := exe.Output(ctx, executor.Options{}, "git", "config", "--global", "user.name")
	email, _ := exe.Output(ctx, executor.Options{}, "git", "config", "--global", "user.email")
	return strings.TrimSpace(name), strings.TrimSpace(email)
}
