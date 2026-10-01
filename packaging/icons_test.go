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

package packaging

import (
	"bytes"
	"image/png"
	"os"
	"strings"
	"testing"

	"github.com/oito2/perci/internal/config"
)

// Every color offered in "Ícone do Aplicativo" must ship every size, as a
// square PNG of exactly that size — the hicolor directory name promises it.
func TestIconsExistForEveryColorAndSize(t *testing.T) {
	for _, color := range config.AppIcons() {
		for _, size := range IconSizes {
			data, err := Icon(color, size)
			if err != nil {
				t.Fatalf("Icon(%q, %d): %v", color, size, err)
			}
			cfg, err := png.DecodeConfig(bytes.NewReader(data))
			if err != nil {
				t.Fatalf("Icon(%q, %d) is not a PNG: %v", color, size, err)
			}
			if cfg.Width != size || cfg.Height != size {
				t.Errorf("Icon(%q, %d) is %dx%d, want %dx%d", color, size, cfg.Width, cfg.Height, size, size)
			}
		}
	}
}

func TestIconRelPath(t *testing.T) {
	if got, want := IconRelPath(48), "48x48/apps/perci.png"; got != want {
		t.Errorf("IconRelPath(48) = %q, want %q", got, want)
	}
}

func TestMustIcon(t *testing.T) {
	want, err := Icon("blue", 512)
	if err != nil {
		t.Fatal(err)
	}
	if got := MustIcon("blue", 512); !bytes.Equal(got, want) {
		t.Error("MustIcon returned different bytes than Icon")
	}
	defer func() {
		if recover() == nil {
			t.Error("MustIcon must panic for an icon that isn't embedded")
		}
	}()
	MustIcon("green", 512)
}

// The menu entry's Icon= key must name the files installed under hicolor.
func TestDesktopEntryUsesIconName(t *testing.T) {
	data, err := os.ReadFile("perci.desktop")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "\nIcon="+IconName+"\n") {
		t.Errorf("perci.desktop must have Icon=%s:\n%s", IconName, data)
	}
}
