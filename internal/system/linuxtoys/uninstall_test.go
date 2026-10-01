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
	"strings"
	"testing"

	"github.com/oito2/perci/internal/distro"
	"github.com/oito2/perci/internal/executor"
)

func TestUninstall_Debian(t *testing.T) {
	var buf bytes.Buffer
	exe := &executor.Executor{DryRun: true, Stdout: &buf, Stderr: &buf}

	if err := uninstall(context.Background(), exe, &buf, distro.Debian); err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	if !strings.Contains(buf.String(), "apt-get") || !strings.Contains(buf.String(), "purge") {
		t.Errorf("expected apt-get purge in dry-run output, got: %s", buf.String())
	}
	if !strings.Contains(buf.String(), pkgName) {
		t.Errorf("expected package name %q in dry-run output, got: %s", pkgName, buf.String())
	}
}

func TestUninstall_Fedora(t *testing.T) {
	var buf bytes.Buffer
	exe := &executor.Executor{DryRun: true, Stdout: &buf, Stderr: &buf}

	if err := uninstall(context.Background(), exe, &buf, distro.Fedora); err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	if !strings.Contains(buf.String(), "dnf") || !strings.Contains(buf.String(), "remove") {
		t.Errorf("expected dnf remove in dry-run output, got: %s", buf.String())
	}
}

func TestUninstall_UnsupportedFamily(t *testing.T) {
	var buf bytes.Buffer
	exe := &executor.Executor{DryRun: true, Stdout: &buf, Stderr: &buf}

	if err := uninstall(context.Background(), exe, &buf, distro.Unknown); err == nil {
		t.Error("expected an error for an unsupported distro family, got nil")
	}
}
