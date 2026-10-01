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

package terminal

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oito2/perci/internal/executor"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// TestUninstallStarship checks that the binary is removed, the config is
// kept as .perci-bak, only the init lines leave the rc files, and that a
// second run (nothing installed) still succeeds.
func TestUninstallStarship(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	bin := filepath.Join(home, ".local", "bin", "starship")
	cfg := filepath.Join(home, ".config", "starship.toml")
	bashrc := filepath.Join(home, ".bashrc")
	zshrc := filepath.Join(home, ".zshrc")
	fish := filepath.Join(home, ".config", "fish", "config.fish")

	writeFile(t, bin, "binary")
	writeFile(t, cfg, "format = \"$all\"\n")
	writeFile(t, bashrc, "alias ll='ls -l'\n\neval \"$(starship init bash)\"\n")
	writeFile(t, zshrc, "export A=1\neval \"$(starship init zsh)\"\n")
	writeFile(t, fish, "set -x B 2\nstarship init fish | source\n")

	if err := UninstallStarship(io.Discard); err != nil {
		t.Fatalf("UninstallStarship: %v", err)
	}

	if _, err := os.Stat(bin); !os.IsNotExist(err) {
		t.Errorf("binary should be removed (err=%v)", err)
	}
	if _, err := os.Stat(cfg); !os.IsNotExist(err) {
		t.Errorf("starship.toml should be renamed (err=%v)", err)
	}
	if got := readFile(t, cfg+".perci-bak"); got != "format = \"$all\"\n" {
		t.Errorf("backup content = %q", got)
	}
	for path, want := range map[string]string{
		bashrc: "alias ll='ls -l'\n\n",
		zshrc:  "export A=1\n",
		fish:   "set -x B 2\n",
	} {
		if got := readFile(t, path); got != want {
			t.Errorf("%s = %q, want %q", path, got, want)
		}
	}

	if err := UninstallStarship(io.Discard); err != nil {
		t.Fatalf("second UninstallStarship: %v", err)
	}
}

// fakeStarshipInstaller makes curl print an install script that drops a
// fake starship into ~/.local/bin; that fake writes "preset <name>" to the
// file given with -o. curl calls are counted in the returned log.
func fakeStarshipInstaller(t *testing.T) (curlLog string) {
	t.Helper()
	curlLog = filepath.Join(t.TempDir(), "curl.log")
	fakeBin(t, map[string]string{"curl": `echo called >> "` + curlLog + `"
cat <<'SCRIPT'
mkdir -p "$HOME/.local/bin"
cat > "$HOME/.local/bin/starship" <<'BIN'
#!/bin/sh
[ "$1" = preset ] && printf 'preset %s\n' "$2" > "$4"
BIN
chmod +x "$HOME/.local/bin/starship"
SCRIPT`})
	return curlLog
}

func TestInstallStarship_AppliesPresetAndHooks(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	fakeStarshipInstaller(t)
	bashrc := filepath.Join(home, ".bashrc")
	fish := filepath.Join(home, ".config", "fish", "config.fish")
	writeFile(t, bashrc, "alias a=b\n")
	writeFile(t, fish, "set -x A 1\n")
	if err := os.MkdirAll(filepath.Join(home, ".config"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := InstallStarship(context.Background(), &executor.Executor{}, io.Discard, "tokyo-night"); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filepath.Join(home, ".config", "starship.toml")); got != "preset tokyo-night\n" {
		t.Errorf("starship.toml = %q", got)
	}
	if !strings.Contains(readFile(t, bashrc), `eval "$(starship init bash)"`) {
		t.Error("bash hook missing")
	}
	if !strings.Contains(readFile(t, fish), "starship init fish | source") {
		t.Error("fish hook missing")
	}
	if _, err := os.Stat(filepath.Join(home, ".zshrc")); !os.IsNotExist(err) {
		t.Error("a missing .zshrc must not be created")
	}

	// Reapplying keeps an existing config and doesn't duplicate hooks.
	writeFile(t, filepath.Join(home, ".config", "starship.toml"), "mine\n")
	if err := InstallStarship(context.Background(), &executor.Executor{}, io.Discard, ""); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filepath.Join(home, ".config", "starship.toml")); got != "mine\n" {
		t.Errorf("existing config overwritten: %q", got)
	}
	if n := strings.Count(readFile(t, bashrc), "starship init bash"); n != 1 {
		t.Errorf("bash hook appears %d times", n)
	}
}

func TestInstallStarship_UnknownPresetDownloadsNothing(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	curlLog := fakeStarshipInstaller(t)
	if err := InstallStarship(context.Background(), &executor.Executor{}, io.Discard, "nope"); err == nil {
		t.Fatal("expected an error")
	}
	if _, err := os.Stat(curlLog); !os.IsNotExist(err) {
		t.Error("curl must not run for an unknown preset")
	}
}

func TestInstallStarship_DownloadFailureFails(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	fakeBin(t, map[string]string{"curl": "exit 6"})
	if err := InstallStarship(context.Background(), &executor.Executor{}, io.Discard, ""); err == nil {
		t.Error("expected an error")
	}
}
