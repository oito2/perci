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

package localbin

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/oito2/perci/internal/executor"
	"github.com/oito2/perci/internal/shellrc"
	"github.com/oito2/perci/internal/ui"
)

const (
	exportLine     = `export PATH="$HOME/.local/bin:$PATH"`
	fishExportLine = `fish_add_path $HOME/.local/bin`
)

// shellPrelude puts ~/.local/bin on PATH and loads nvm (which activates
// the default Node version) — what a new login shell would see, so tools
// installed during this session are found without restarting Perci.
const shellPrelude = `export NVM_DIR="$HOME/.nvm"; ` +
	`case ":$PATH:" in *":$HOME/.local/bin:"*) ;; *) PATH="$HOME/.local/bin:$PATH" ;; esac; export PATH; ` +
	`[ -s "$NVM_DIR/nvm.sh" ] && . "$NVM_DIR/nvm.sh" >/dev/null 2>&1; `

// inheritedPath is the PATH Perci was started with — what the user's
// shells already provide. EnsureInPath checks it, not the current PATH,
// which RefreshPath always extends with ~/.local/bin.
var inheritedPath = os.Getenv("PATH")

// RefreshPath replaces this process's PATH with the one shellPrelude
// builds. The GUI never rereads ~/.bashrc, so without this a tool
// installed into ~/.local/bin or through nvm stays "not installed" — and
// every exe.Run of it fails — until Perci is restarted. Run at startup and
// after every action; a failure keeps the current PATH.
func RefreshPath(ctx context.Context, exe *executor.Executor) {
	if exe.DryRun {
		return
	}
	out, err := exe.Output(ctx, executor.Options{}, "bash", "-c", shellPrelude+`printf '%s' "$PATH"`)
	if err != nil || strings.TrimSpace(out) == "" {
		return
	}
	_ = os.Setenv("PATH", strings.TrimSpace(out))
}

// EnsureInPath adds $HOME/.local/bin to the user's shell rc file
// (shellrc.File: ~/.bashrc, ~/.zshrc or fish's config.fish) when it is not
// already present in PATH. Several tools (Claude Code, Kitty, Antigravity CLI) install
// their binaries there and require the directory to be on PATH to be found.
func EnsureInPath(stdout io.Writer) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	localBin := filepath.Join(home, ".local", "bin")

	for _, p := range filepath.SplitList(inheritedPath) {
		if p == localBin {
			return
		}
	}

	rc := shellrc.File(home)
	line := exportLine
	if filepath.Base(rc) == "config.fish" {
		line = fishExportLine
	}
	if _, err := shellrc.AppendIfMissing(rc, "", line); err != nil {
		ui.Warning(stdout, "Adicione manualmente a "+rc+": "+line)
	}
}

// Which reports whether cmd is resolvable with shellPrelude's PATH.
// A plain `which` only sees the PATH captured when Perci started, so it
// misses binaries that nvm-based npm installs place under
// ~/.nvm/versions/node/<version>/bin/ during the same session.
// Checked in-process first (RefreshPath already put both on the GUI's
// PATH); the shell is only spawned for a command not found that way.
func Which(ctx context.Context, exe *executor.Executor, cmd string) bool {
	if exe.CommandAvailable(ctx, cmd) {
		return true
	}
	script := shellPrelude + `command -v "$1"`
	_, err := exe.Output(ctx, executor.Options{}, "bash", "-c", script, "--", cmd)
	return err == nil
}

// RunNPMGlobal runs `npm <action> -g <pkg>`, sourcing nvm when available.
// action must be "install" or "uninstall". Never requires sudo since nvm
// installs are user-local.
func RunNPMGlobal(ctx context.Context, exe *executor.Executor, stdout io.Writer, action, pkg string) error {
	script := `
set -e
export NVM_DIR="$HOME/.nvm"
[ -s "$NVM_DIR/nvm.sh" ] && . "$NVM_DIR/nvm.sh"
if ! command -v npm &>/dev/null; then
    printf 'Node.js não encontrado. Execute DevStuff → Instalar Pré-requisitos primeiro.\n' >&2
    exit 1
fi
npm "$1" -g "$2"
`
	return exe.Run(ctx, executor.Options{Stdout: stdout, Stderr: stdout}, "bash", "-c", script, "--", action, pkg)
}
