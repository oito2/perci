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

package selfupdate

import "testing"

func TestIsNewer(t *testing.T) {
	cases := []struct {
		latest, current string
		want            bool
	}{
		{"v1.2.0", "1.1.9", true},
		{"v1.2.0", "v1.2.0", false},
		{"v1.2.0", "1.10.0", false}, // numeric, not string, comparison
		{"v1.10.0", "1.9.0", true},
		{"v1.2.0", "1.3.0", false}, // local build newer: no downgrade
		{"v2.0.0", "2.0.0-beta.1", true},
		{"v2.0.0-beta.2", "2.0.0-beta.1", true},
		{"v2.0.0-beta.10", "2.0.0-beta.9", true},
		{"v2.0.0-beta", "2.0.0-alpha", true},
		{"v2.0.0-beta.1", "2.0.0", false},
		{"v2.0.0-alpha.1", "2.0.0-alpha", true},
		{"v1.0.0+build.5", "1.0.0", false},
		{"v1.0.0", "dev", true}, // not semver: any difference counts
		{"dev", "dev", false},
		{"v01.0.0", "1.0.0", true}, // leading zero isn't semver → plain difference
	}
	for _, tc := range cases {
		if got := IsNewer(tc.latest, tc.current); got != tc.want {
			t.Errorf("IsNewer(%q, %q) = %v, want %v", tc.latest, tc.current, got, tc.want)
		}
	}
}
