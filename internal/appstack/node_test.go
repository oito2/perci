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

package appstack

import (
	"strings"
	"testing"
)

func TestValidNodeVersion(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"22", true},
		{"24", true},
		{"26", true},
		{"18", false},
		{"20", false},
		{"", false},
		{"latest", false},
	}
	for _, tt := range tests {
		if got := ValidNodeVersion(tt.in); got != tt.want {
			t.Errorf("ValidNodeVersion(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestValidDevPort(t *testing.T) {
	tests := []struct {
		name string
		port int
		want bool
	}{
		{"typical Vite port", 5173, true},
		{"typical webpack-dev-server port", 8080, true},
		{"typical Next.js port", 3000, true},
		{"lower bound, unprivileged", 1024, true},
		{"upper bound", 65535, true},
		{"privileged port rejected", 80, false},
		{"privileged port rejected", 1023, false},
		{"above valid TCP range", 65536, false},
		{"zero rejected", 0, false},
		{"negative rejected", -1, false},
		{"collides with php-fpm fastcgi port", 9000, false},
		{"collides with MariaDB port", 3306, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidDevPort(tt.port); got != tt.want {
				t.Errorf("ValidDevPort(%d) = %v, want %v", tt.port, got, tt.want)
			}
		})
	}
}

func TestNodeImageName(t *testing.T) {
	if got, want := NodeImageName("24"), "perci-node24"; got != want {
		t.Errorf("NodeImageName(%q) = %q, want %q", "24", got, want)
	}
}

func TestNodeDockerfile_BuildsFromOfficialImage(t *testing.T) {
	if !strings.Contains(nodeDockerfile, "FROM node:${NODE_VERSION}-bookworm") {
		t.Errorf("expected nodeDockerfile to build FROM the official node image, got:\n%s", nodeDockerfile)
	}
	if !strings.Contains(nodeDockerfile, "usermod -u ${UID} node") {
		t.Errorf("expected nodeDockerfile to usermod the \"node\" user to the host UID (matching PHPDockerfile's www-data treatment), got:\n%s", nodeDockerfile)
	}
}
