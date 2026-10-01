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

package linuxtoys

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/oito2/perci/internal/executor"
)

// Installed reflects whether linuxtoys resolves on PATH.
func TestInstalled_FollowsPath(t *testing.T) {
	for _, present := range []bool{true, false} {
		exe := &executor.Executor{LookPath: func(name string) (string, error) {
			if present && name == "linuxtoys" {
				return "/usr/bin/linuxtoys", nil
			}
			return "", errors.New("not found")
		}}
		if got := Installed(context.Background(), exe); got != present {
			t.Errorf("present=%v: Installed = %v", present, got)
		}
	}
}

func TestInstall_DryRunFailsOnEmptyDownload(t *testing.T) {
	// Under DryRun, exe.Run("curl", ...) never actually writes to the temp
	// file it's told to create — Install must detect the resulting
	// empty file and fail cleanly instead of proceeding to "bash <empty
	// file>".
	var buf bytes.Buffer
	exe := &executor.Executor{DryRun: true, Stdout: &buf, Stderr: &buf}

	err := Install(context.Background(), exe, &buf)
	if err == nil {
		t.Fatal("expected an error when the downloaded installer is empty, got nil")
	}
	if !strings.Contains(buf.String(), "vazio") {
		t.Errorf("expected an empty-download message in output, got: %s", buf.String())
	}
}
