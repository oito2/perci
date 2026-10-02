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

package checklist

import (
	"errors"
	"io"
	"strings"
	"testing"
)

// A failing item doesn't stop the others, and Apply reports it.
func TestApply_ReportsFailuresWithoutStopping(t *testing.T) {
	var done []string
	install := func(s string) error {
		done = append(done, "+"+s)
		if s == "b" {
			return errors.New("boom")
		}
		return nil
	}
	remove := func(s string) error { done = append(done, "-"+s); return nil }

	err := Apply(io.Discard, []string{"a", "b", "c"}, func(s string) string { return s }, []string{"a", "b"}, []string{"c"}, install, remove)
	if err == nil || !strings.Contains(err.Error(), "instalar b") {
		t.Fatalf("Apply = %v, want the failure of b", err)
	}
	if got := strings.Join(done, ","); got != "+a,+b,-c" {
		t.Errorf("items processed = %s, want every one of them", got)
	}
	if err := Apply(io.Discard, []string{"a"}, func(s string) string { return s }, []string{"a"}, nil, func(string) error { return nil }, remove); err != nil {
		t.Errorf("all successes must return nil, got %v", err)
	}
}
