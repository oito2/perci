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
	"testing"

	managerai "github.com/oito2/perci/internal/manager/ai"
)

// The screen receives every project type with its models and hint.
func TestGetAIContextPresets(t *testing.T) {
	got := (&DevToolsService{}).GetAIContextPresets()
	want := managerai.Presets()
	if len(got) != len(want) || len(got) == 0 {
		t.Fatalf("got %d presets, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].Name != want[i].Name || len(got[i].Models) != len(want[i].Models) || got[i].Hint != want[i].Hint {
			t.Errorf("preset %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}
