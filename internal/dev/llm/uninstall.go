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

package llm

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/oito2/perci/internal/dev/localbin"
	"github.com/oito2/perci/internal/executor"
	"github.com/oito2/perci/internal/shellrc"
	"github.com/oito2/perci/internal/ui"
)

// UninstallOne uninstalls a single LLM CLI tool.
func UninstallOne(ctx context.Context, exe *executor.Executor, stdout io.Writer, l LLM) error {
	switch l.Cmd {
	case "claude":
		dataDir := claudeDataDir(exe)
		if err := removeBinary(ctx, exe, stdout, l.Cmd); err != nil {
			return err
		}
		if dataDir == "" {
			return nil
		}
		ui.Info(stdout, "Removendo as versões baixadas em "+dataDir+"...")
		return exe.Run(ctx, executor.Options{Stdout: stdout, Stderr: stdout}, "rm", "-rf", "--", dataDir)
	case "opencode":
		if err := removeBinary(ctx, exe, stdout, l.Cmd); err != nil {
			return err
		}
		return removeOpenCodeLeftovers(ctx, exe, stdout)
	case "agy":
		// Installed by their own curl scripts (install.go), which ship no
		// uninstaller: the binary found on PATH is removed.
		return removeBinary(ctx, exe, stdout, l.Cmd)
	case "codex":
		return localbin.RunNPMGlobal(ctx, exe, stdout, "uninstall", "@openai/codex")
	}
	return fmt.Errorf("desinstalador desconhecido para %s", l.Name)
}

// claudeDataDir returns ~/.local/share/claude when the claude found on PATH
// is the native installer's symlink into it — every downloaded version
// lives there (hundreds of MB), and removing only the link left all of it
// behind. "" otherwise (e.g. installed through npm): only the binary goes.
// User settings (~/.claude, ~/.claude.json) are never touched.
func claudeDataDir(exe *executor.Executor) string {
	p, ok := exe.Which("claude")
	if !ok {
		return ""
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	target, err := filepath.EvalSymlinks(p)
	if err != nil {
		return ""
	}
	dataDir := filepath.Join(home, ".local", "share", "claude")
	if rel, err := filepath.Rel(dataDir, target); err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return ""
	}
	return dataDir
}

// removeOpenCodeLeftovers undoes what OpenCode's official installer adds
// besides the binary: its ~/.opencode/bin directory (~/.opencode itself only
// when left empty) and the two lines it appends to the shell's rc file —
// "# opencode" plus a PATH line for that directory. Only those exact lines
// are removed, from every rc file the installer may have picked. Settings
// in ~/.config/opencode are kept.
func removeOpenCodeLeftovers(ctx context.Context, exe *executor.Executor, stdout io.Writer) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	base := filepath.Join(home, ".opencode")
	binDir := filepath.Join(base, "bin")
	opts := executor.Options{Stdout: stdout, Stderr: stdout}
	if err := exe.Run(ctx, opts, "rm", "-rf", "--", binDir); err != nil {
		return err
	}
	// Absent when OpenCode came from elsewhere (e.g. npm): nothing to report.
	if _, statErr := os.Stat(base); statErr == nil {
		if err := exe.Run(ctx, opts, "rmdir", "--ignore-fail-on-non-empty", "--", base); err != nil {
			ui.Warning(stdout, "Não foi possível remover "+base+": "+err.Error())
		}
	}

	xdg := os.Getenv("XDG_CONFIG_HOME")
	if xdg == "" {
		xdg = filepath.Join(home, ".config")
	}
	zdot := os.Getenv("ZDOTDIR")
	if zdot == "" {
		zdot = home
	}
	candidates := shellrc.Dedup([]string{
		filepath.Join(home, ".bashrc"), filepath.Join(home, ".bash_profile"), filepath.Join(home, ".profile"),
		filepath.Join(xdg, "bash", ".bashrc"), filepath.Join(xdg, "bash", ".bash_profile"),
		filepath.Join(zdot, ".zshrc"), filepath.Join(zdot, ".zshenv"),
		filepath.Join(xdg, "zsh", ".zshrc"), filepath.Join(xdg, "zsh", ".zshenv"),
		filepath.Join(home, ".config", "fish", "config.fish"),
	})
	for _, entry := range []string{"export PATH=" + binDir + ":$PATH", "fish_add_path " + binDir} {
		for _, res := range shellrc.RemoveEntry(candidates, "# opencode", entry) {
			if res.Err != nil {
				ui.Warning(stdout, "Não foi possível atualizar "+res.Path+": "+res.Err.Error())
				continue
			}
			ui.Info(stdout, "PATH do OpenCode removido de "+res.Path)
		}
	}
	return nil
}

// removeBinary deletes cmd's resolved path; a cmd not on PATH is reported,
// not silently skipped.
func removeBinary(ctx context.Context, exe *executor.Executor, stdout io.Writer, cmd string) error {
	p, ok := exe.Which(cmd)
	if !ok {
		ui.Warning(stdout, cmd+" não foi encontrado no PATH — nada a remover.")
		return nil
	}
	return exe.Run(ctx, executor.Options{Stdout: stdout, Stderr: stdout}, "rm", "-f", "--", p)
}
