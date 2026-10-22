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
	"bytes"
	"context"
	"fmt"
	"io"
	"strconv"
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
// ps -a` lists every container of the host, and the statuses of names are
// picked from it; a name with no container is simply absent from the
// result. `docker ps` only fails when the Docker daemon can't be reached
// (daemon stopped, no permission on its socket, docker not installed), so
// an error means the statuses are unknown, not that the containers are
// missing; it carries docker's trimmed stderr.
func ContainerStatuses(ctx context.Context, exe *executor.Executor, names []string) (map[string]string, error) {
	result := make(map[string]string, len(names))
	if len(names) == 0 {
		return result, nil
	}

	var stderr bytes.Buffer
	out, err := exe.Output(ctx, executor.Options{Stderr: &stderr}, "docker", "ps", "-a", "--no-trunc", "--format", "{{.Names}}\t{{.State}}")
	if err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return nil, fmt.Errorf("%w: %s", err, msg)
		}
		return nil, err
	}
	all := parseContainerStatuses(out)
	for _, name := range names {
		if status, ok := all[name]; ok {
			result[name] = status
		}
	}
	return result, nil
}

// parseContainerStatuses parses `docker ps --format '{{.Names}}\t{{.State}}'`
// output (one "name\tstatus" per line; a leading "/" is dropped) into a
// name->status map, without needing a Docker daemon.
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
// Nginx, MariaDB and any registered Container Aplicativo: plain `docker
// start/stop/restart` by name. Any other container name is refused.
func StartContainer(ctx context.Context, exe *executor.Executor, stdout io.Writer, name string) error {
	if err := requireStackContainer(name); err != nil {
		return err
	}
	if err := exe.Run(ctx, executor.Options{Stdout: stdout, Stderr: stdout}, "docker", "start", "--", name); err != nil {
		return fmt.Errorf("iniciar %s: %w", name, err)
	}
	return nil
}

func StopContainer(ctx context.Context, exe *executor.Executor, stdout io.Writer, name string) error {
	if err := requireStackContainer(name); err != nil {
		return err
	}
	if err := exe.Run(ctx, executor.Options{Stdout: stdout, Stderr: stdout}, "docker", "stop", "--", name); err != nil {
		return fmt.Errorf("parar %s: %w", name, err)
	}
	return nil
}

func RestartContainer(ctx context.Context, exe *executor.Executor, stdout io.Writer, name string) error {
	if err := requireStackContainer(name); err != nil {
		return err
	}
	if err := exe.Run(ctx, executor.Options{Stdout: stdout, Stderr: stdout}, "docker", "restart", "--", name); err != nil {
		return fmt.Errorf("reiniciar %s: %w", name, err)
	}
	return nil
}

// ContainerLogs returns the last tail lines of the logs of name (stdout
// and stderr together), for Nginx, MariaDB or a registered Container
// Aplicativo; any other container name is refused.
func ContainerLogs(ctx context.Context, exe *executor.Executor, name string, tail int) (string, error) {
	if err := requireStackContainer(name); err != nil {
		return "", err
	}
	var buf bytes.Buffer
	err := exe.Run(ctx, executor.Options{Stdout: &buf, Stderr: &buf}, "docker", "logs", "--tail", strconv.Itoa(tail), "--", name)
	return buf.String(), err
}
