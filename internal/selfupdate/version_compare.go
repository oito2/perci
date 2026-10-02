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

import (
	"strconv"
	"strings"
)

// IsNewer reports whether latest is a newer version than current. Both are
// compared as semantic versions (MAJOR.MINOR.PATCH with an optional
// -prerelease and a leading "v"; build metadata is ignored), so a local build newer than the
// latest release is never offered a downgrade. When either isn't a
// semantic version (e.g. a "dev" build), any difference counts as newer.
func IsNewer(latest, current string) bool {
	l, lok := parseSemver(latest)
	c, cok := parseSemver(current)
	if !lok || !cok {
		return normalizeVersion(latest) != normalizeVersion(current)
	}
	return compareSemver(l, c) > 0
}

type semver struct {
	core [3]int
	pre  []string // nil for a release
}

func normalizeVersion(v string) string {
	return strings.TrimPrefix(strings.TrimSpace(v), "v")
}

func parseSemver(v string) (semver, bool) {
	v = normalizeVersion(v)
	if i := strings.IndexByte(v, '+'); i >= 0 {
		v = v[:i]
	}
	var s semver
	if i := strings.IndexByte(v, '-'); i >= 0 {
		if i == len(v)-1 {
			return s, false
		}
		s.pre = strings.Split(v[i+1:], ".")
		v = v[:i]
	}
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return s, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 || p == "" || (len(p) > 1 && p[0] == '0') {
			return s, false
		}
		s.core[i] = n
	}
	for _, id := range s.pre {
		if id == "" {
			return s, false
		}
	}
	return s, true
}

// compareSemver orders versions by semantic precedence: core numbers
// first, then a release ranks above any of its prereleases, then
// prerelease identifiers one by one (numeric ones numerically and below
// alphanumeric ones; more identifiers rank higher when all else is equal).
func compareSemver(a, b semver) int {
	for i := range a.core {
		if a.core[i] != b.core[i] {
			return cmpInt(a.core[i], b.core[i])
		}
	}
	switch {
	case a.pre == nil && b.pre == nil:
		return 0
	case a.pre == nil:
		return 1
	case b.pre == nil:
		return -1
	}
	for i := 0; i < len(a.pre) && i < len(b.pre); i++ {
		x, y := a.pre[i], b.pre[i]
		xn, xerr := strconv.Atoi(x)
		yn, yerr := strconv.Atoi(y)
		switch {
		case xerr == nil && yerr == nil:
			if xn != yn {
				return cmpInt(xn, yn)
			}
		case xerr == nil:
			return -1
		case yerr == nil:
			return 1
		default:
			if c := strings.Compare(x, y); c != 0 {
				return c
			}
		}
	}
	return cmpInt(len(a.pre), len(b.pre))
}

func cmpInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}
