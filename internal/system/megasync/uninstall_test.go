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
	"bytes"
	"context"
	"os/exec"
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
	// Repository and key go too, in the same single privileged call.
	for _, path := range []string{aptSourceList, aptKeyring} {
		if !strings.Contains(buf.String(), path) {
			t.Errorf("expected %s to be removed, got: %s", path, buf.String())
		}
	}
	if n := strings.Count(buf.String(), "[dry-run]"); n != 1 {
		t.Errorf("expected one privileged batch, got %d", n)
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
	if !strings.Contains(buf.String(), dnfRepoFile) {
		t.Errorf("expected %s to be removed, got: %s", dnfRepoFile, buf.String())
	}
	// The key imported with rpm --import is removed as well.
	if !strings.Contains(buf.String(), "rpmkeys --delete") || !strings.Contains(buf.String(), "MegaLimited") {
		t.Errorf("expected MEGA's rpm key to be removed, got: %s", buf.String())
	}
	if n := strings.Count(buf.String(), "[dry-run]"); n != 1 {
		t.Errorf("want one privileged batch, got %d", n)
	}
}

func TestRPMKeyRemoveScript_ValidShell(t *testing.T) {
	if out, err := exec.Command("sh", "-n", "-c", rpmKeyRemoveScript).CombinedOutput(); err != nil {
		t.Errorf("sh -n: %v\n%s", err, out)
	}
}

func TestUninstall_UnsupportedFamily(t *testing.T) {
	var buf bytes.Buffer
	exe := &executor.Executor{DryRun: true, Stdout: &buf, Stderr: &buf}

	if err := uninstall(context.Background(), exe, &buf, distro.Unknown); err == nil {
		t.Error("expected an error for an unsupported distro family, got nil")
	}
}
