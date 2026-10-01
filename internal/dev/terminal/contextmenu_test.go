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
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func findTerminal(t *testing.T, name string) Terminal {
	t.Helper()
	for _, term := range Catalogue {
		if term.Name == name {
			return term
		}
	}
	t.Fatalf("terminal %q not in Catalogue", name)
	return Terminal{}
}

// TestSyncContextMenuEntries checks that the entries follow the installed
// state: created for installed terminals (with the expected modes), removed
// for the others, and a second sync is a no-op.
func TestSyncContextMenuEntries(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	kitty := findTerminal(t, "Kitty")
	alacritty := findTerminal(t, "Alacritty")

	// Alacritty starts with stale entries that must go away.
	AddContextMenuEntries(alacritty, io.Discard)

	installed := map[string]bool{"Kitty": true}
	SyncContextMenuEntries(installed, io.Discard)
	SyncContextMenuEntries(installed, io.Discard)

	for _, c := range []struct {
		path string
		perm os.FileMode
	}{
		{nautilusScriptPath(kitty, home), 0o755},
		{nemoScriptPath(kitty, home), 0o755},
		{dolphinMenuPath(kitty, home), 0o644},
	} {
		info, err := os.Stat(c.path)
		if err != nil {
			t.Fatalf("expected %s: %v", c.path, err)
		}
		if info.Mode().Perm() != c.perm {
			t.Errorf("%s: mode %v, want %v", c.path, info.Mode().Perm(), c.perm)
		}
	}
	for _, p := range []string{
		nautilusScriptPath(alacritty, home),
		nemoScriptPath(alacritty, home),
		dolphinMenuPath(alacritty, home),
	} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("%s should have been removed (err=%v)", p, err)
		}
	}

	SyncContextMenuEntries(map[string]bool{}, io.Discard)
	if _, err := os.Stat(dolphinMenuPath(kitty, home)); !os.IsNotExist(err) {
		t.Errorf("Kitty entries should be removed once it is no longer installed")
	}
}

// TestContextMenuContent checks the generated command lines, including the
// Flatpak MenuExec override and per-word quoting.
func TestContextMenuContent(t *testing.T) {
	blackbox := findTerminal(t, "Black Box")
	wantCmd := `'flatpak' 'run' 'com.raggesilver.BlackBox' '--working-directory'`

	if s := nautilusScript(blackbox); !strings.Contains(s, wantCmd+` "$DIR"`) {
		t.Errorf("nautilus script missing %q:\n%s", wantCmd, s)
	}
	if s := nemoScript(blackbox); !strings.Contains(s, wantCmd+` "$DIR"`) {
		t.Errorf("nemo script missing %q:\n%s", wantCmd, s)
	}
	d := dolphinMenu(blackbox)
	for _, want := range []string{
		"Exec=" + wantCmd + " %f\n",
		"Name=Abrir no Black Box\n",
		"Actions=perci-open-black-box;\n",
		"[Desktop Action perci-open-black-box]\n",
	} {
		if !strings.Contains(d, want) {
			t.Errorf("dolphin menu missing %q:\n%s", want, d)
		}
	}
}

// TestContextMenuScriptsAreValidBash parses every generated script with
// bash -n.
func TestContextMenuScriptsAreValidBash(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}
	for _, term := range Catalogue {
		for _, s := range []string{nautilusScript(term), nemoScript(term)} {
			cmd := exec.Command("bash", "-n")
			cmd.Stdin = strings.NewReader(s)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Errorf("%s: bash -n failed: %v\n%s", term.Name, err, out)
			}
		}
	}
}
