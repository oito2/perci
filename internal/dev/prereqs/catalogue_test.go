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
	"testing"

	"github.com/oito2/perci/internal/distro"
)

func TestLibfuse2Pkg(t *testing.T) {
	tests := []struct {
		name   string
		family string
		want   string
	}{
		{"fedora", distro.Fedora, "fuse-libs"},
		{"debian", distro.Debian, "libfuse2t64"},
		{"unknown family falls back to debian package", "arch", "libfuse2t64"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := libfuse2Pkg(tt.family); got != tt.want {
				t.Errorf("libfuse2Pkg(%q) = %q, want %q", tt.family, got, tt.want)
			}
		})
	}
}

func TestMkcertPkgs(t *testing.T) {
	tests := []struct {
		name   string
		family string
		want   []string
	}{
		{"fedora", distro.Fedora, []string{"mkcert", "nss-tools"}},
		{"debian", distro.Debian, []string{"mkcert", "libnss3-tools"}},
		{"unknown family falls back to debian packages", "arch", []string{"mkcert", "libnss3-tools"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mkcertPkgs(tt.family)
			if len(got) != len(tt.want) {
				t.Fatalf("mkcertPkgs(%q) = %v, want %v", tt.family, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("mkcertPkgs(%q)[%d] = %q, want %q", tt.family, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestDockerPkgs(t *testing.T) {
	tests := []struct {
		name   string
		family string
		want   []string
	}{
		{"fedora", distro.Fedora, []string{"moby-engine", "docker-compose", "docker-buildx"}},
		{"debian", distro.Debian, []string{"docker.io", "docker-compose-v2", "docker-buildx"}},
		{"unknown family falls back to debian packages", "arch", []string{"docker.io", "docker-compose-v2", "docker-buildx"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dockerPkgs(tt.family)
			if len(got) != len(tt.want) {
				t.Fatalf("dockerPkgs(%q) = %v, want %v", tt.family, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("dockerPkgs(%q)[%d] = %q, want %q", tt.family, i, got[i], tt.want[i])
				}
			}
		})
	}
}
