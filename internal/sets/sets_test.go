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

package sets_test

import (
	"testing"

	"github.com/oito2/perci/internal/sets"
)

func TestOf(t *testing.T) {
	s := sets.Of([]string{"a", "b", "c"})
	for _, item := range []string{"a", "b", "c"} {
		if !s[item] {
			t.Errorf("expected %q to be present", item)
		}
	}
	if s["d"] {
		t.Error("unexpected item 'd' in set")
	}
}

func TestOfEmpty(t *testing.T) {
	s := sets.Of(nil)
	if len(s) != 0 {
		t.Errorf("expected empty set, got len %d", len(s))
	}
}

func TestOfDuplicates(t *testing.T) {
	s := sets.Of([]string{"x", "x", "x"})
	if len(s) != 1 {
		t.Errorf("expected 1 entry for duplicates, got %d", len(s))
	}
	if !s["x"] {
		t.Error("expected 'x' to be present")
	}
}

func TestOfSingleItem(t *testing.T) {
	s := sets.Of([]string{"only"})
	if len(s) != 1 || !s["only"] {
		t.Errorf("unexpected set contents: %v", s)
	}
}

func sameElements(t *testing.T, name string, got, want []string) {
	t.Helper()
	gotSet, wantSet := sets.Of(got), sets.Of(want)
	if len(got) != len(want) || len(gotSet) != len(wantSet) {
		t.Errorf("%s = %v, want %v", name, got, want)
		return
	}
	for k := range wantSet {
		if !gotSet[k] {
			t.Errorf("%s = %v, want %v", name, got, want)
			return
		}
	}
}

func TestDiff(t *testing.T) {
	allKeys := []string{"a", "b", "c", "d"}
	installed := sets.Of([]string{"a", "b"}) // a, b already installed; c, d are not

	cases := []struct {
		name        string
		selected    []string
		wantInstall []string
		wantRemove  []string
	}{
		{
			name:        "no change",
			selected:    []string{"a", "b"},
			wantInstall: nil,
			wantRemove:  nil,
		},
		{
			name:        "select a new one, keep the rest",
			selected:    []string{"a", "b", "c"},
			wantInstall: []string{"c"},
			wantRemove:  nil,
		},
		{
			name:        "deselect an installed one",
			selected:    []string{"a"},
			wantInstall: nil,
			wantRemove:  []string{"b"},
		},
		{
			name:        "install one, remove another, in the same pass",
			selected:    []string{"a", "d"},
			wantInstall: []string{"d"},
			wantRemove:  []string{"b"},
		},
		{
			name:        "deselect everything",
			selected:    nil,
			wantInstall: nil,
			wantRemove:  []string{"a", "b"},
		},
		{
			name:        "select everything",
			selected:    []string{"a", "b", "c", "d"},
			wantInstall: []string{"c", "d"},
			wantRemove:  nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			toInstall, toRemove := sets.Diff(allKeys, installed, tc.selected)
			sameElements(t, "toInstall", toInstall, tc.wantInstall)
			sameElements(t, "toRemove", toRemove, tc.wantRemove)
		})
	}
}

func TestDiff_SelectedNotInCatalogueIsIgnored(t *testing.T) {
	// A selectedName that isn't part of allKeys (stale/unknown entry) must
	// not show up in either result — Diff only ever reports on allKeys.
	toInstall, toRemove := sets.Diff([]string{"a"}, sets.Of(nil), []string{"a", "ghost"})
	sameElements(t, "toInstall", toInstall, []string{"a"})
	sameElements(t, "toRemove", toRemove, nil)
}
