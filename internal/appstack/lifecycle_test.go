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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oito2/perci/internal/executor"
)

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
			"docker ps names without a slash",
			"nginx\trunning\nloja\texited",
			map[string]string{"nginx": "running", "loja": "exited"},
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

// fakeDockerPS puts a `docker` script first on PATH that prints out for
// `docker ps` and fails with stderr msg when fail is set.
func fakeDockerPS(t *testing.T, out string, fail bool, msg string) {
	t.Helper()
	bin := t.TempDir()
	script := "#!/bin/sh\n"
	if fail {
		script += "echo '" + msg + "' >&2\nexit 1\n"
	} else {
		script += "[ \"$1\" = ps ] || exit 1\nprintf '" + out + "'\n"
	}
	if err := os.WriteFile(filepath.Join(bin, "docker"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestContainerStatuses(t *testing.T) {
	exe := executor.New(nil, nil)

	t.Run("missing container is absent, others keep their status", func(t *testing.T) {
		fakeDockerPS(t, `nginx\trunning\nother\texited\n`, false, "")
		got, err := ContainerStatuses(t.Context(), exe, []string{"nginx", "loja"})
		if err != nil {
			t.Fatalf("ContainerStatuses: unexpected error %v", err)
		}
		if len(got) != 1 || got["nginx"] != "running" {
			t.Errorf("got %v, want map[nginx:running]", got)
		}
	})

	t.Run("unreachable docker is an error carrying stderr", func(t *testing.T) {
		fakeDockerPS(t, "", true, "permission denied while trying to connect to the docker API")
		got, err := ContainerStatuses(t.Context(), exe, []string{"nginx"})
		if err == nil {
			t.Fatalf("got %v, nil error; want an error", got)
		}
		if !strings.Contains(err.Error(), "permission denied") {
			t.Errorf("error %q does not carry docker's stderr", err)
		}
	})
}
