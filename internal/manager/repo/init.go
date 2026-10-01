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

	"github.com/oito2/perci/internal/executor"
)

// Init runs git init and applies local identity in dir, the folder the
// user picked (empty means the process's current directory). `git init [<dir>]` itself creates dir if it
// doesn't exist yet, same as running it after `mkdir -p`.
func Init(ctx context.Context, exe *executor.Executor, stdout io.Writer, dir, name, email string) error {
	if err := checkIdentityPair(name, email); err != nil {
		return err
	}
	opts := executor.Options{Stdout: stdout, Stderr: stdout}
	args := []string{"init", "-b", "main"}
	if dir != "" {
		args = append(args, "--", dir)
	}
	if err := exe.Run(ctx, opts, "git", args...); err != nil {
		return fmt.Errorf("git init: %w", err)
	}

	if name != "" || email != "" {
		if err := ApplyLocalIdentityAt(ctx, exe, stdout, name, email, dir); err != nil {
			return err
		}
	}
	return nil
}
