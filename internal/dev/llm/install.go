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
	"github.com/oito2/perci/internal/executor"
)

// InstallOne installs a single LLM CLI tool.
func InstallOne(ctx context.Context, exe *executor.Executor, stdout io.Writer, l LLM) error {
	opts := executor.Options{Stdout: stdout, Stderr: stdout}
	switch l.Cmd {
	case "claude":
		if err := exe.Run(ctx, opts, "bash", "-c", `set -Eeuo pipefail; curl -fsSL https://claude.ai/install.sh | bash`); err != nil {
			return err
		}
		localbin.EnsureInPath(stdout)
		return nil
	case "agy":
		if err := exe.Run(ctx, opts, "bash", "-c", `set -Eeuo pipefail; curl -fsSL https://antigravity.google/cli/install.sh | bash`); err != nil {
			return err
		}
		localbin.EnsureInPath(stdout)
		return nil
	case "codex":
		return localbin.RunNPMGlobal(ctx, exe, stdout, "install", "@openai/codex")
	case "opencode":
		if err := exe.Run(ctx, opts, "bash", "-c", `set -Eeuo pipefail; curl -fsSL https://opencode.ai/install | bash`); err != nil {
			return err
		}
		localbin.EnsureInPath(stdout)
		return nil
	}
	return fmt.Errorf("instalador desconhecido para %s", l.Name)
}
