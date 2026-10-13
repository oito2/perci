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

	"github.com/oito2/perci/internal/dev/localbin"
	"github.com/oito2/perci/internal/dev/nettimeout"
	"github.com/oito2/perci/internal/executor"
)

// codexPackage is the npm package installed for the Codex CLI, pinned to
// a release.
const codexPackage = "@openai/codex@0.162.1"

// InstallOne installs a single LLM CLI tool.
func InstallOne(ctx context.Context, exe *executor.Executor, stdout io.Writer, l LLM) error {
	opts := executor.Options{Stdout: stdout, Stderr: stdout}
	ctx, cancel := nettimeout.Context(ctx)
	defer cancel()
	switch l.Cmd {
	case "claude":
		return runInstaller(ctx, exe, stdout, opts, "https://claude.ai/install.sh")
	case "agy":
		return runInstaller(ctx, exe, stdout, opts, "https://antigravity.google/cli/install.sh")
	case "codex":
		return localbin.RunNPMGlobal(ctx, exe, stdout, "install", codexPackage)
	case "opencode":
		return runInstaller(ctx, exe, stdout, opts, "https://opencode.ai/install")
	}
	return fmt.Errorf("instalador desconhecido para %s", l.Name)
}

// runInstaller downloads the vendor install script at url with bounded
// curl timeouts, runs it with bash and puts ~/.local/bin on PATH.
func runInstaller(ctx context.Context, exe *executor.Executor, stdout io.Writer, opts executor.Options, url string) error {
	script := `set -Eeuo pipefail; curl -fsSL ` + nettimeout.CurlShellFlags + ` ` + executor.ShellQuote(url) + ` | bash`
	if err := exe.Run(ctx, opts, "bash", "-c", script); err != nil {
		return err
	}
	localbin.EnsureInPath(stdout)
	return nil
}
