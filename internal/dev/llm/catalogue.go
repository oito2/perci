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
	"sync"

	"github.com/oito2/perci/internal/dev/localbin"
	"github.com/oito2/perci/internal/executor"
)

// LLM describes an AI CLI tool managed by perci.gnl.
type LLM struct {
	Name string
	Cmd  string // binary name checked via `which`
}

// Catalogue lists all LLM CLIs managed by perci.gnl.
var Catalogue = []LLM{
	{Name: "Claude Code CLI", Cmd: "claude"},
	{Name: "Antigravity CLI", Cmd: "agy"},
	{Name: "Codex CLI", Cmd: "codex"},
	{Name: "OpenCode CLI", Cmd: "opencode"},
}

// InstalledMap returns which LLMs are currently installed (by Name).
func InstalledMap(ctx context.Context, exe *executor.Executor) map[string]bool {
	result := make(map[string]bool, len(Catalogue))
	var mu sync.Mutex
	// Concurrent: a command not on PATH costs a shell sourcing nvm.
	_ = executor.RunConcurrent(ctx, Catalogue, len(Catalogue), func(l LLM) {
		if localbin.Which(ctx, exe, l.Cmd) {
			mu.Lock()
			result[l.Name] = true
			mu.Unlock()
		}
	})
	return result
}
