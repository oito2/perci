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

package prereqs

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/oito2/perci/internal/distro"
	"github.com/oito2/perci/internal/executor"
)

// Install clones at the pinned commit and installs the node generator with
// pipx, without privileges beyond the one package batch.
func TestInstallFlatpakTools(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(flatpakToolsCommit) {
		t.Fatalf("commit not pinned: %q", flatpakToolsCommit)
	}
	var buf bytes.Buffer
	if err := installFlatpakTools(context.Background(), dryRun(&buf), &buf, distro.Debian); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	dir := filepath.Join(home, "tools", "flatpak-builder-tools")
	for _, want := range []string{
		"'pipx' 'python3-aiohttp' 'python3-tomlkit'",
		"git [clone --quiet -- " + flatpakToolsRepo + " " + dir + "]",
		"git [-C " + dir + " checkout --quiet --detach " + flatpakToolsCommit + "]",
		"pipx [install --force " + filepath.Join(dir, "node") + "]",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	if n := strings.Count(out, "pkexec"); n != 1 {
		t.Errorf("%d password prompts, want 1:\n%s", n, out)
	}
}

// A folder at the clone path that isn't a clone of the repository is never
// deleted.
func TestUninstallFlatpakTools_KeepsAForeignFolder(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, "tools", "flatpak-builder-tools")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	fakeBin(t, map[string]string{"git": "echo https://example.com/other.git", "pipx": ""})
	err := uninstallFlatpakTools(context.Background(), executor.New(nil, nil), &bytes.Buffer{})
	if err == nil {
		t.Fatal("a foreign folder was accepted")
	}
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("foreign folder deleted: %v", err)
	}

	fakeBin(t, map[string]string{"git": "echo " + flatpakToolsRepo, "pipx": ""})
	if err := uninstallFlatpakTools(context.Background(), executor.New(nil, nil), &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("clone kept: %v", err)
	}
}

func TestFlatpakToolsInstalled(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if flatpakToolsInstalled() {
		t.Fatal("installed with nothing in place")
	}
	gen := filepath.Join(home, "tools", "flatpak-builder-tools", "cargo", "flatpak-cargo-generator.py")
	node := filepath.Join(home, ".local", "bin", "flatpak-node-generator")
	for _, f := range []string{gen, node} {
		if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f, nil, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if !flatpakToolsInstalled() {
		t.Error("not detected")
	}
}
