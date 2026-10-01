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

func TestBuildAppWrapperScript_PHP(t *testing.T) {
	got := buildAppWrapperScript("meuapp", "php", "/ws/html/meuapp", "/var/www/html", wrapperUser)

	if !strings.Contains(got, `CONTAINER='meuapp'`) {
		t.Errorf("missing container name, got:\n%s", got)
	}
	if !strings.Contains(got, `WS_HOST='/ws/html/meuapp'`) {
		t.Errorf("missing host path, got:\n%s", got)
	}
	if !strings.Contains(got, `WS_CONT='/var/www/html'`) {
		t.Errorf("missing container path, got:\n%s", got)
	}
	if !strings.Contains(got, `EXEC_AS=(-u 'www-data' -e HOME=/tmp)`) {
		t.Errorf("PHP tools must run as www-data, got:\n%s", got)
	}
	if !strings.Contains(got, "docker exec -it \"${EXEC_AS[@]}\" \"${CONTAINER}\" php \"${ARGS[@]}\"") {
		t.Errorf("missing interactive exec of the right tool, got:\n%s", got)
	}
	if !strings.Contains(got, "docker exec -i \"${EXEC_AS[@]}\" \"${CONTAINER}\" php \"${ARGS[@]}\"") {
		t.Errorf("missing non-interactive exec of the right tool, got:\n%s", got)
	}
}

func TestBuildAppWrapperScript_Node(t *testing.T) {
	got := buildAppWrapperScript("frontend", "npm", "/ws/html/frontend", "/app", "")

	if !strings.Contains(got, `CONTAINER='frontend'`) {
		t.Errorf("missing container name, got:\n%s", got)
	}
	if !strings.Contains(got, `WS_CONT='/app'`) {
		t.Errorf("expected the Node container path (/app), got:\n%s", got)
	}
	if !strings.Contains(got, "EXEC_AS=()") {
		t.Errorf("a Node-only container already runs as node, got:\n%s", got)
	}
	if !strings.Contains(got, "docker exec -it \"${EXEC_AS[@]}\" \"${CONTAINER}\" npm \"${ARGS[@]}\"") {
		t.Errorf("missing interactive exec of npm, got:\n%s", got)
	}
}

func TestAppWrapperSingleQuote(t *testing.T) {
	got := appWrapperSingleQuote("it's a path")
	want := `'it'\''s a path'`
	if got != want {
		t.Errorf("appWrapperSingleQuote(%q) = %q, want %q", "it's a path", got, want)
	}
}
