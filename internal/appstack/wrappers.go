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
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/oito2/perci/internal/dev/localbin"
	"github.com/oito2/perci/internal/ui"
)

// appTools is the list of tools wrapped per Container Aplicativo, used by
// AppTypeMoodle/AppTypePHP/AppTypeGeneric, whose project root is mounted at
// /var/www/html.
var appTools = []string{"php", "phpcs", "phpcbf", "phpunit", "composer"}

// nodeAppTools is appTools' equivalent for AppTypeNode, whose project root
// is mounted at /app: just npm.
var nodeAppTools = []string{"npm"}

// wrapperUser is who the PHP-family and PHP+Node wrappers run as. Those
// containers run as root, so the wrappers exec as www-data, whose UID
// matches the host user, keeping vendor/ and node_modules/ owned by the
// host user.
const wrapperUser = "www-data"

// writeAppWrappers creates ~/.local/bin/{php,composer,phpcs,phpcbf,
// phpunit}-<folder> for a PHP-family Container Aplicativo (Moodle/PHP/
// Generic), each pointing at that app's own container, project root at
// /var/www/html.
func writeAppWrappers(folder, htmlDir string, stdout io.Writer) {
	writeWrappers(folder, appTools, htmlDir, "/var/www/html", wrapperUser, stdout)
}

// writeNodeAppWrappers is writeAppWrappers' equivalent for AppTypeNode:
// ~/.local/bin/npm-<folder>, project root at /app. user is "" for a
// Node-only container, which already runs as the UID-matched "node" user,
// and wrapperUser for a PHP+Node one.
func writeNodeAppWrappers(folder, appDir, user string, stdout io.Writer) {
	writeWrappers(folder, nodeAppTools, appDir, "/app", user, stdout)
}

// removeAppWrappers deletes every wrapper writeAppWrappers/
// writeNodeAppWrappers may have created for folder, so no <tool>-<folder>
// is left pointing at a missing container.
func removeAppWrappers(folder string) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	for _, tool := range append(append([]string{}, appTools...), nodeAppTools...) {
		_ = os.Remove(filepath.Join(home, ".local", "bin", tool+"-"+folder))
	}
}

// writeWrappers creates ~/.local/bin/<tool>-<folder> for every tool in
// tools, each pointing at that app's own container.
func writeWrappers(folder string, tools []string, hostDir, containerDir, user string, stdout io.Writer) {
	home, err := os.UserHomeDir()
	if err != nil {
		ui.Warning(stdout, "Não foi possível determinar diretório home: "+err.Error())
		return
	}

	localBin := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(localBin, 0o755); err != nil {
		ui.Warning(stdout, "Não foi possível criar ~/.local/bin: "+err.Error())
		return
	}

	for _, tool := range tools {
		name := tool + "-" + folder
		path := filepath.Join(localBin, name)
		// Each wrapper is removed first so a symlink at path is replaced, not followed.
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			ui.Warning(stdout, "Falha ao substituir wrapper "+name+": "+err.Error())
			continue
		}
		if err := os.WriteFile(path, []byte(buildAppWrapperScript(folder, tool, hostDir, containerDir, user)), 0o755); err != nil {
			ui.Warning(stdout, "Falha ao criar wrapper "+name+": "+err.Error())
		}
	}

	localbin.EnsureInPath(stdout)
}

func appWrapperSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// buildAppWrapperScript rewrites any argument under the host project path
// to its containerDir equivalent, then execs into the container,
// interactive when attached to a real terminal and non-interactive (docker
// exec -i) otherwise. A non-empty user runs the tool as that user, with
// HOME=/tmp so tools such as npm have a writable cache directory.
func buildAppWrapperScript(container, tool, hostDir, containerDir, user string) string {
	execAs := "()"
	if user != "" {
		execAs = "(-u " + appWrapperSingleQuote(user) + " -e HOME=/tmp)"
	}
	return fmt.Sprintf(`#!/usr/bin/env bash
CONTAINER=%s
WS_HOST=%s
WS_CONT=%s
EXEC_AS=%s
ARGS=()
for arg in "$@"; do
    if [[ "$arg" == "${WS_HOST}/"* ]] || [[ "$arg" == "${WS_HOST}" ]]; then
        ARGS+=("${WS_CONT}${arg#${WS_HOST}}")
    else
        ARGS+=("$arg")
    fi
done
if [ -t 0 ] && [ -t 1 ]; then
    exec docker exec -it "${EXEC_AS[@]}" "${CONTAINER}" %s "${ARGS[@]}"
else
    exec docker exec -i "${EXEC_AS[@]}" "${CONTAINER}" %s "${ARGS[@]}"
fi
`, appWrapperSingleQuote(container), appWrapperSingleQuote(hostDir), appWrapperSingleQuote(containerDir), execAs, tool, tool)
}
