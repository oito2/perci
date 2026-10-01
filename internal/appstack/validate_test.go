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

import "testing"

func TestValidAppFolder(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"simple lowercase", "meucurso", true},
		{"with hyphen and underscore", "meu-curso_2026", true},
		{"single char", "a", true},
		{"64 letters", repeat('a', 64), true},
		{"65 letters, one over the limit", repeat('a', 65), false},
		{"empty", "", false},
		{"space", "meu curso", false},
		{"slash", "meu/curso", false},
		{"dot", "meu.curso", false},
		{"semicolon (nginx injection attempt)", "app;evil", false},
		{"braces (nginx block injection attempt)", "app}{", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidAppFolder.MatchString(tt.in); got != tt.want {
				t.Errorf("ValidAppFolder.MatchString(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestValidAppURL(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"simple", "meucurso.localhost", true},
		{"with hyphen", "meu-curso.localhost", true},
		{"single char label", "a.localhost", true},
		{"digits", "app123.localhost", true},
		{"nested label rejected (outside the wildcard cert)", "meu.curso.localhost", false},
		{"missing .localhost suffix", "meucurso.com", false},
		{"leading hyphen", "-app.localhost", false},
		{"trailing hyphen", "app-.localhost", false},
		{"uppercase rejected", "MeuCurso.localhost", false},
		{"empty label", ".localhost", false},
		{"empty", "", false},
		{"space (nginx server_name injection attempt)", "app evil.localhost", false},
		{"semicolon (nginx directive injection attempt)", "app;evil.localhost", false},
		{"wildcard literal rejected", "*.localhost", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidAppURL.MatchString(tt.in); got != tt.want {
				t.Errorf("ValidAppURL.MatchString(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestValidPHPMemoryLimit(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"plain M suffix", "512M", true},
		{"plain M suffix lowercase", "768m", true},
		{"G suffix", "1G", true},
		{"K suffix", "65536K", true},
		{"bare bytes", "268435456", true},
		{"unlimited", "-1", true},
		{"empty rejected (caller decides the default separately)", "", false},
		{"negative other than -1 rejected", "-2", false},
		{"non-numeric rejected", "lots", false},
		{"trailing garbage rejected", "512MB", false},
		{"space (php.ini/shell injection attempt)", "512M; rm -rf /", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidPHPMemoryLimit.MatchString(tt.in); got != tt.want {
				t.Errorf("ValidPHPMemoryLimit.MatchString(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func repeat(c byte, n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = c
	}
	return string(b)
}
