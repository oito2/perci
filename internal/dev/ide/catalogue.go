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

package ide

import (
	"context"
	"sync"

	"github.com/oito2/perci/internal/dev/localbin"
	"github.com/oito2/perci/internal/executor"
)

// IDE describes an editor managed by perci.gnl.
type IDE struct {
	Name string
	Cmd  string
}

// Catalogue lists all IDEs managed by perci.gnl.
var Catalogue = []IDE{
	{Name: "Zed Editor", Cmd: "zed"},
	{Name: "VS Code", Cmd: "code"},
	{Name: "VSCodium", Cmd: "codium"},
}

// InstalledMap returns which IDEs are currently installed (by Name),
// detected through localbin.Which.
func InstalledMap(ctx context.Context, exe *executor.Executor) map[string]bool {
	result := make(map[string]bool, len(Catalogue))
	var mu sync.Mutex
	// Concurrent: a command not on PATH costs a shell sourcing nvm.
	_ = executor.RunConcurrent(ctx, Catalogue, len(Catalogue), func(e IDE) {
		if localbin.Which(ctx, exe, e.Cmd) {
			mu.Lock()
			result[e.Name] = true
			mu.Unlock()
		}
	})
	return result
}
