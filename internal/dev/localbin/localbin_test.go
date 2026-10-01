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

package localbin

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oito2/perci/internal/executor"
)

// RefreshPath puts ~/.local/bin on the process PATH exactly once, keeping
// what was already there.
func TestRefreshPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", "/usr/bin:/bin")

	exe := executor.New(nil, nil)
	RefreshPath(context.Background(), exe)
	RefreshPath(context.Background(), exe)

	want := filepath.Join(home, ".local", "bin") + ":/usr/bin:/bin"
	if got := os.Getenv("PATH"); got != want {
		t.Errorf("PATH = %q, want %q", got, want)
	}
}

func TestRefreshPath_DryRunKeepsPath(t *testing.T) {
	t.Setenv("PATH", "/usr/bin:/bin")
	exe := executor.New(nil, nil)
	exe.DryRun = true
	RefreshPath(context.Background(), exe)
	if got := os.Getenv("PATH"); got != "/usr/bin:/bin" {
		t.Errorf("dry-run changed PATH to %q", got)
	}
}

func TestEnsureInPath_Fish(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SHELL", "/usr/bin/fish")
	saved := inheritedPath
	inheritedPath = "/usr/bin"
	t.Cleanup(func() { inheritedPath = saved })
	if err := os.MkdirAll(filepath.Join(home, ".config", "fish"), 0o755); err != nil {
		t.Fatal(err)
	}

	EnsureInPath(nil)

	b, err := os.ReadFile(filepath.Join(home, ".config", "fish", "config.fish"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), fishExportLine) || strings.Contains(string(b), "export PATH") {
		t.Errorf("config.fish = %q, want the fish syntax only", b)
	}
}
