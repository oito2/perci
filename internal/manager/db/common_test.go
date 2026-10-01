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

package db

import (
	"context"
	"testing"

	"github.com/oito2/perci/internal/executor"
)

func TestWithDBSession_RejectsPasswordWithNewline(t *testing.T) {
	exe := executor.New(nil, nil)
	called := false
	// A password containing a newline must be rejected before RequireContainer
	// even runs (no Docker needed for this test) — otherwise it would inject
	// extra lines into the MYSQL_PWD env file passed to `docker exec`.
	err := withDBSession(context.Background(), exe, "mariadb", "x\nMYSQL_PWD=hacked", func(envPath string) error {
		called = true
		return nil
	})
	if err == nil {
		t.Fatal("expected error for password containing a newline, got nil")
	}
	if called {
		t.Error("fn should not have been called when the password is rejected")
	}
}
