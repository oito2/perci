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

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestWriteAutostartDesktopFile writes the XDG autostart .desktop entry,
// checks it has the fields a hidden-on-login autostart entry needs
// (NoDisplay, Exec), then removes it and confirms it's gone.
func TestWriteAutostartDesktopFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := writeAutostartDesktopFile(); err != nil {
		t.Fatalf("writeAutostartDesktopFile: %v", err)
	}

	path := filepath.Join(home, ".config", "autostart", "perci.desktop")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ler arquivo gerado: %v", err)
	}
	content := string(data)
	if !strings.HasPrefix(content, "[Desktop Entry]") {
		t.Error("arquivo .desktop malformado — deveria começar com [Desktop Entry]")
	}
	if !strings.Contains(content, "NoDisplay=true") {
		t.Error("autostart deve ter NoDisplay=true (não deve aparecer no menu de apps, só rodar oculto no login)")
	}
	if !strings.Contains(content, "Exec=") {
		t.Error("arquivo .desktop sem linha Exec=")
	}

	if err := removeAutostartDesktopFile(); err != nil {
		t.Fatalf("removeAutostartDesktopFile: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("arquivo deveria ter sido removido")
	}
}

func TestRemoveAutostartDesktopFile_NeverEnabled(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := removeAutostartDesktopFile(); err != nil {
		t.Errorf("remover um autostart que nunca existiu não deveria ser erro: %v", err)
	}
}

// TestOpenCompactWindow_UnknownTab confirms that openCompactWindow silently
// ignores an unknown tab (compactTabs[tab] == false) instead of creating a
// window — doesn't depend on any real Wails window/app, it only exercises
// the early return that happens before any Wails API call.
func TestOpenCompactWindow_UnknownTab(t *testing.T) {
	a := &TrayService{}
	// Must not try to use a.wailsApp (nil) for an invalid tab — doing so
	// would panic (nil pointer dereference) instead of simply returning.
	a.openCompactWindow("aba-que-nao-existe")
	if a.compactWindow != nil {
		t.Error("openCompactWindow com aba inválida não deveria criar nenhuma janela")
	}
}

func TestDesktopExecQuote(t *testing.T) {
	cases := map[string]string{
		"/usr/local/bin/prci":  `"/usr/local/bin/prci"`,
		"/home/u/My Apps/prci": `"/home/u/My Apps/prci"`,
		`/opt/a"b`:             `"/opt/a\\"b"`,
		"/opt/$HOME/`x`":       "\"/opt/\\\\$HOME/\\\\`x\\\\`\"",
		`/opt/back\slash`:      `"/opt/back\\\\slash"`,
		"/opt/100%":            `"/opt/100%%"`,
	}
	for in, want := range cases {
		got, err := desktopExecQuote(in)
		if err != nil || got != want {
			t.Errorf("desktopExecQuote(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	if _, err := desktopExecQuote("/opt/a\nb"); err == nil {
		t.Error("a newline can't be represented in a .desktop value and must be refused")
	}
}

// The autostart entry starts Perci hidden in the tray (main.go honors it).
func TestWriteAutostartDesktopFile_StartsHidden(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := writeAutostartDesktopFile(); err != nil {
		t.Fatal(err)
	}
	path, _ := autostartDesktopPath()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "Exec=") {
			if !strings.HasPrefix(line, `Exec="`) || !strings.HasSuffix(line, `" `+hiddenFlag) {
				t.Errorf("Exec line = %q, want a quoted binary followed by %s", line, hiddenFlag)
			}
			return
		}
	}
	t.Fatal("no Exec= line")
}
