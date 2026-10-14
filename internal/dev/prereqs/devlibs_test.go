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
	"context"
	"slices"
	"testing"

	"github.com/oito2/perci/internal/distro"
	"github.com/oito2/perci/internal/executor"
)

// The development libraries a Tauri v2 app needs, on each family.
func TestPackagesFor_DevLibraries(t *testing.T) {
	tests := []struct {
		id             string
		debian, fedora []string
	}{
		{"build-essential", []string{"build-essential"}, []string{"gcc", "gcc-c++", "make"}},
		{"webkitgtk41dev", []string{"libwebkit2gtk-4.1-dev"}, []string{"webkit2gtk4.1-devel"}},
		{"libxdo", []string{"libxdo-dev"}, []string{"libxdo-devel"}},
		{"openssl", []string{"openssl", "libssl-dev"}, []string{"openssl", "openssl-devel"}},
		{"librsvg", []string{"librsvg2-bin", "librsvg2-dev"}, []string{"librsvg2-tools", "librsvg2-devel"}},
	}
	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			if got := packagesFor(tt.id, distro.Debian); !slices.Equal(got, tt.debian) {
				t.Errorf("debian = %v, want %v", got, tt.debian)
			}
			if got := packagesFor(tt.id, distro.Fedora); !slices.Equal(got, tt.fedora) {
				t.Errorf("fedora = %v, want %v", got, tt.fedora)
			}
		})
	}
}

// An item whose command is present but whose development headers are
// missing shows as not installed, so Instalar can add the headers.
func TestIsInstalled_NeedsTheDevelopmentHeaders(t *testing.T) {
	exe := executor.New(nil, nil)
	fakeBin(t, map[string]string{"openssl": "", "rsvg-convert": "", "secret-tool": "", "gcc": "", "make": "", "pkg-config": "exit 1"})
	for _, id := range []string{"openssl", "librsvg", "libsecret", "build-essential"} {
		if isInstalled(context.Background(), exe, id) {
			t.Errorf("%s installed without its headers (or g++)", id)
		}
	}
	fakeBin(t, map[string]string{"openssl": "", "rsvg-convert": "", "secret-tool": "", "gcc": "", "g++": "", "make": "",
		"pkg-config": `case "$2" in openssl|librsvg-2.0|libsecret-1|webkit2gtk-4.1) exit 0;; esac; exit 1`})
	for _, id := range []string{"openssl", "librsvg", "libsecret", "build-essential", "webkitgtk41dev"} {
		if !isInstalled(context.Background(), exe, id) {
			t.Errorf("%s not detected", id)
		}
	}
}
