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
	"os"
	"regexp"
	"strings"
	"testing"
)

// Every binary the Makefile builds uses the production tag, which turns off
// Wails' debug mode (webview developer tools).
func TestMakefileBuildsWithProductionTag(t *testing.T) {
	data, err := os.ReadFile("../../Makefile")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !regexp.MustCompile(`(?m)^TAGS\s*:=\s*production\b`).MatchString(text) {
		t.Fatal("the Makefile must define TAGS := production")
	}
	builds := 0
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, "go build") && !strings.HasPrefix(strings.TrimSpace(line), "#") {
			builds++
			if !strings.Contains(line, "-tags $(TAGS)") {
				t.Errorf("go build without the production tag: %s", strings.TrimSpace(line))
			}
		}
	}
	if builds == 0 {
		t.Fatal("no go build line found in the Makefile")
	}
}
