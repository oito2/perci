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

package selfupdate

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/oito2/perci/internal/executor"
	"github.com/oito2/perci/packaging"
)

// The whole icon set must go through ONE privileged invocation (one pkexec
// prompt) that reads the archive from stdin and refreshes the icon cache.
func TestInstallMenuIcons_SingleBatchFromStdin(t *testing.T) {
	var out strings.Builder
	exe := executor.New(&out, &out)
	exe.DryRun = true

	if err := InstallMenuIcons(context.Background(), exe, &out, "pink"); err != nil {
		t.Fatalf("InstallMenuIcons: %v", err)
	}

	got := out.String()
	if n := strings.Count(got, "[dry-run]"); n != 1 {
		t.Fatalf("expected 1 privileged invocation, got %d:\n%s", n, got)
	}
	for _, want := range []string{"'tar' '-x' '-f' '-' '-C' '" + hicolorDir + "'", "gtk-update-icon-cache"} {
		if !strings.Contains(got, want) {
			t.Errorf("batch missing %q:\n%s", want, got)
		}
	}
}

// Extracting the archive (as root would, but into a temp dir) must produce
// exactly the hicolor paths Uninstall later removes, with the embedded bytes.
func TestMenuIconsTar_ExtractsEveryIcon(t *testing.T) {
	if _, err := exec.LookPath("tar"); err != nil {
		t.Skip("tar not available")
	}
	archive, err := menuIconsTar("blue")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	cmd := exec.Command("tar", "-x", "-f", "-", "-C", dir, "--no-same-owner", "--no-same-permissions", "--no-overwrite-dir")
	cmd.Stdin = archive
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("tar: %v: %s", err, out)
	}
	for _, size := range packaging.IconSizes {
		got, err := os.ReadFile(filepath.Join(dir, packaging.IconRelPath(size)))
		if err != nil {
			t.Fatalf("size %d: %v", size, err)
		}
		want, _ := packaging.Icon("blue", size)
		if !bytes.Equal(got, want) {
			t.Errorf("size %d: extracted icon differs from the embedded one", size)
		}
		if p := filepath.Join(hicolorDir, packaging.IconRelPath(size)); !slices.Contains(menuIconPaths(), p) {
			t.Errorf("%s not covered by menuIconPaths (Uninstall)", p)
		}
	}
}

func TestInstallMenuIcons_UnknownColor(t *testing.T) {
	exe := executor.New(nil, nil)
	exe.DryRun = true
	if err := InstallMenuIcons(context.Background(), exe, nil, "green"); err == nil {
		t.Fatal("expected an error for a color with no icon set")
	}
}
