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

func TestParseContainerStatuses(t *testing.T) {
	cases := []struct {
		name string
		out  string
		want map[string]string
	}{
		{"empty output", "", map[string]string{}},
		{
			"single line",
			"/nginx\trunning",
			map[string]string{"nginx": "running"},
		},
		{
			"multiple lines",
			"/nginx\trunning\n/mariadb\texited\n/myapp\tcreated",
			map[string]string{"nginx": "running", "mariadb": "exited", "myapp": "created"},
		},
		{
			"malformed line without tab is skipped",
			"/nginx\trunning\nnot-a-valid-line\n/mariadb\texited",
			map[string]string{"nginx": "running", "mariadb": "exited"},
		},
		{
			"trailing blank lines are ignored",
			"/nginx\trunning\n\n",
			map[string]string{"nginx": "running"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseContainerStatuses(tc.out)
			if len(got) != len(tc.want) {
				t.Fatalf("parseContainerStatuses(%q) = %v, want %v", tc.out, got, tc.want)
			}
			for name, status := range tc.want {
				if got[name] != status {
					t.Errorf("parseContainerStatuses(%q)[%q] = %q, want %q", tc.out, name, got[name], status)
				}
			}
		})
	}
}
