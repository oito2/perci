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
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/oito2/perci/internal/executor"
)

// ContainerStatus returns a Container Aplicativo/Nginx/MariaDB's current
// Docker status (docker inspect's .State.Status: "running", "exited",
// "created", "paused", "restarting", "dead"), or "" if no container by
// that name exists (e.g. it was removed outside perci with `docker rm`).
func ContainerStatus(ctx context.Context, exe *executor.Executor, name string) string {
	out, err := exe.Output(ctx, executor.Options{}, "docker", "inspect", "-f", "{{.State.Status}}", "--", name)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// ContainerStatuses is the batched form of ContainerStatus: one `docker
// inspect` for every name at once instead of one subprocess per name.
//
// `docker inspect` exits non-zero as soon as any requested name does not
// exist, but still prints every successfully inspected container to
// stdout. The command runs via `bash -c ... || true` so the exit code stays
// 0 and exe.Output still returns the captured statuses; a missing
// container is simply absent from the result. Names are shell-quoted with
// ShellQuote.
func ContainerStatuses(ctx context.Context, exe *executor.Executor, names []string) map[string]string {
	result := make(map[string]string, len(names))
	if len(names) == 0 {
		return result
	}

	quoted := make([]string, len(names))
	for i, n := range names {
		quoted[i] = executor.ShellQuote(n)
	}
	script := "docker inspect -f '{{.Name}}\t{{.State.Status}}' -- " + strings.Join(quoted, " ") + " 2>/dev/null || true"

	out, _ := exe.Output(ctx, executor.Options{}, "bash", "-c", script)
	for name, status := range parseContainerStatuses(out) {
		result[name] = status
	}
	return result
}

// parseContainerStatuses parses `docker inspect -f '{{.Name}}\t{{.State.Status}}'`
// output (one "/name\tstatus" per line) into a name->status map, without
// needing a Docker daemon.
func parseContainerStatuses(out string) map[string]string {
	result := make(map[string]string)
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		name, status, ok := strings.Cut(line, "\t")
		if !ok {
			continue
		}
		result[strings.TrimPrefix(name, "/")] = status
	}
	return result
}

// removeContainer force-removes name via `docker rm -f`, wrapping any
// failure with label in the returned error. Shared by
// RemoveApp/RemoveNginx/RemoveMariaDB.
func removeContainer(ctx context.Context, exe *executor.Executor, stdout io.Writer, name, label string) error {
	if err := exe.Run(ctx, executor.Options{Stdout: stdout, Stderr: stdout}, "docker", "rm", "-f", "--", name); err != nil {
		return fmt.Errorf("remover %s: %w", label, err)
	}
	return nil
}

// StartContainer, StopContainer and RestartContainer work identically for
// Nginx, MariaDB and any Container Aplicativo: plain `docker
// start/stop/restart` by name.
func StartContainer(ctx context.Context, exe *executor.Executor, stdout io.Writer, name string) error {
	if err := exe.Run(ctx, executor.Options{Stdout: stdout, Stderr: stdout}, "docker", "start", "--", name); err != nil {
		return fmt.Errorf("iniciar %s: %w", name, err)
	}
	return nil
}

func StopContainer(ctx context.Context, exe *executor.Executor, stdout io.Writer, name string) error {
	if err := exe.Run(ctx, executor.Options{Stdout: stdout, Stderr: stdout}, "docker", "stop", "--", name); err != nil {
		return fmt.Errorf("parar %s: %w", name, err)
	}
	return nil
}

func RestartContainer(ctx context.Context, exe *executor.Executor, stdout io.Writer, name string) error {
	if err := exe.Run(ctx, executor.Options{Stdout: stdout, Stderr: stdout}, "docker", "restart", "--", name); err != nil {
		return fmt.Errorf("reiniciar %s: %w", name, err)
	}
	return nil
}
