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

package terminal

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"

	"github.com/oito2/perci/internal/executor"
	"github.com/oito2/perci/internal/shellrc"
	"github.com/oito2/perci/internal/ui"
)

var StarshipPresets = []string{
	"gruvbox-rainbow",
	"tokyo-night",
	"pastel-powerline",
	"pure-preset",
}

// InstallStarship downloads Starship to ~/.local/bin, optionally applies a preset.
func InstallStarship(ctx context.Context, exe *executor.Executor, stdout io.Writer, preset string) error {
	if preset == "" {
		preset = StarshipPresets[0]
	}
	// Validated before anything is downloaded.
	if !slices.Contains(StarshipPresets, preset) {
		return fmt.Errorf("preset do starship desconhecido: %q", preset)
	}
	script := `
set -Eeuo pipefail
mkdir -p "$HOME/.local/bin"
curl -fsSL https://starship.rs/install.sh | sh -s -- --yes --bin-dir "$HOME/.local/bin"
`
	if err := exe.Run(ctx, executor.Options{Stdout: stdout, Stderr: stdout}, "bash", "-c", script); err != nil {
		return err
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	if _, statErr := os.Stat(filepath.Join(home, ".config", "starship.toml")); os.IsNotExist(statErr) {
		starshipBin := filepath.Join(home, ".local", "bin", "starship")
		cfgPath := filepath.Join(home, ".config", "starship.toml")
		if pErr := exe.Run(ctx, executor.Options{Stdout: stdout, Stderr: stdout},
			starshipBin, "preset", preset, "-o", cfgPath); pErr != nil {
			ui.Warning(stdout, "Falha ao aplicar preset: "+pErr.Error())
		}
	}

	ConfigureStarshipShells(stdout)
	return nil
}

// UninstallStarship removes the Starship binary and shell init hooks. The
// config file is renamed to starship.toml.perci-bak, not deleted.
func UninstallStarship(stdout io.Writer) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	_ = os.Remove(filepath.Join(home, ".local", "bin", "starship"))
	cfg := filepath.Join(home, ".config", "starship.toml")
	if _, statErr := os.Stat(cfg); statErr == nil {
		if err := os.Rename(cfg, cfg+".perci-bak"); err != nil {
			ui.Warning(stdout, "Não foi possível renomear "+cfg+": "+err.Error())
		} else {
			ui.Info(stdout, "Configuração do Starship preservada em "+cfg+".perci-bak")
		}
	}

	stripStarshipLine(filepath.Join(home, ".bashrc"), `eval "$(starship init bash)"`)
	stripStarshipLine(filepath.Join(home, ".zshrc"), `eval "$(starship init zsh)"`)
	stripStarshipLine(filepath.Join(home, ".config", "fish", "config.fish"), `starship init fish | source`)

	ui.Info(stdout, "Configurações do Starship removidas dos shells.")
	return nil
}

// ConfigureStarshipShells appends the init hook to each shell config that exists.
func ConfigureStarshipShells(stdout io.Writer) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}

	type entry struct {
		rc   string
		line string
	}
	shells := []entry{
		{filepath.Join(home, ".bashrc"), `eval "$(starship init bash)"`},
		{filepath.Join(home, ".zshrc"), `eval "$(starship init zsh)"`},
	}
	for _, s := range shells {
		if _, statErr := os.Stat(s.rc); statErr == nil {
			if appendLineIfMissing(s.rc, s.line) {
				ui.Info(stdout, "Starship registrado em "+s.rc)
			}
		}
	}

	fishConfig := filepath.Join(home, ".config", "fish", "config.fish")
	if _, statErr := os.Stat(fishConfig); statErr == nil {
		if appendLineIfMissing(fishConfig, `starship init fish | source`) {
			ui.Info(stdout, "Starship registrado em "+fishConfig)
		}
	}
}

func appendLineIfMissing(path, line string) bool {
	appended, err := shellrc.AppendIfMissing(path, "", line)
	return err == nil && appended
}

// stripStarshipLine removes line from path, if present, via
// shellrc.RemoveEntry. Best-effort: errors are ignored.
func stripStarshipLine(path, line string) {
	shellrc.RemoveEntry([]string{path}, "", line)
}
