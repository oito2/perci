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
	"os"

	"github.com/oito2/perci/internal/executor"
)

// ComboImageName returns the perci-managed base image tag for a PHP+Node
// combo, e.g. "perci-php82-node24" for ("8.2", "24"). The image is built
// once per (PHP version, Node version) pair and reused by every
// AppTypePHPNode container on that pair.
func ComboImageName(phpVersion, nodeVersion string) string {
	return "perci-php" + phpTag(phpVersion) + "-node" + nodeVersion
}

// comboSupervisordConf is baked into every combo image as-is. Only the
// DEV_COMMAND environment variable, which supervisord resolves at
// container-start time through its %(ENV_X)s syntax, varies per container,
// so comboAppRunArgs must always pass -e DEV_COMMAND=... (supervisord
// fails to start when a referenced variable does not exist).
//
// [program:php-fpm] has no user=: php-fpm's master process starts as root
// and its pool config drops the worker processes to www-data.
// [program:dev-server] sets user=www-data, whose UID matches the host user
// (see phpDockerfile).
//
// command=%(ENV_DEV_COMMAND)s is not run through a shell: supervisord
// splits it into argv respecting quotes, but never interprets shell
// operators (&&, ;, pipes, $VAR). A compound command can be given as e.g.
// `sh -c "cd /app && npm run dev"`.
//
// [supervisord] is mandatory because comboDockerfile's CMD points
// supervisord's -c straight at this file, so it is a complete,
// self-contained config. nodaemon is not set here because the CMD passes
// supervisord its own -n flag.
//
// [program:worker] is an optional extra background process, such as a
// queue/job consumer (config.AppContainer.WorkerCommand). It is baked into
// every combo image unconditionally, since the image is shared across
// every AppTypePHPNode container on that (PHP, Node) pair. Its command is
// a fixed shell wrapper that reads $WORKER_COMMAND from the container
// environment at run time (comboAppRunArgs always sets it, possibly to an
// empty string), so the program starts cleanly when WorkerCommand is
// empty. When empty, the wrapper execs `sleep infinity` as an idle
// placeholder, so the program never busy-loop-restarts. When set, the
// command runs through its own sh -c, so variables, && and pipes work as
// typed.
const comboSupervisordConf = `[supervisord]
logfile=/dev/null
logfile_maxbytes=0
pidfile=/var/run/supervisord.pid

[program:php-fpm]
command=php-fpm -F
autorestart=true
stdout_logfile=/dev/stdout
stdout_logfile_maxbytes=0
stderr_logfile=/dev/stderr
stderr_logfile_maxbytes=0

[program:dev-server]
command=%(ENV_DEV_COMMAND)s
directory=/app
user=www-data
autorestart=true
stdout_logfile=/dev/stdout
stdout_logfile_maxbytes=0
stderr_logfile=/dev/stderr
stderr_logfile_maxbytes=0

[program:worker]
command=sh -c "if [ -n \"$WORKER_COMMAND\" ]; then exec sh -c \"$WORKER_COMMAND\"; else exec sleep infinity; fi"
directory=/var/www/html
user=www-data
autorestart=true
stdout_logfile=/dev/stdout
stdout_logfile_maxbytes=0
stderr_logfile=/dev/stderr
stderr_logfile_maxbytes=0
`

// comboDockerfile extends phpImageDockerfile (PHP + php.ini) with Node
// (via the NodeSource apt setup script) and supervisor (Debian's
// `supervisor` package), plus the baked-in supervisord.conf above. CMD is
// overridden to launch supervisord, which starts php-fpm as one of its two
// managed programs. The base image's ENTRYPOINT (docker-php-entrypoint) is
// untouched and execs the given CMD verbatim.
const comboDockerfile = phpImageDockerfile + `
ARG NODE_VERSION=24

RUN curl -fsSL https://deb.nodesource.com/setup_${NODE_VERSION}.x | bash - \
 && apt-get update \
 && apt-get install -y --no-install-recommends nodejs supervisor \
 && rm -rf /var/lib/apt/lists/*

COPY supervisord.conf /etc/supervisor/conf.d/supervisord.conf

WORKDIR /var/www/html
CMD ["supervisord", "-n", "-c", "/etc/supervisor/conf.d/supervisord.conf"]
`

// EnsureComboImage builds ComboImageName(phpVersion, nodeVersion) when
// it is missing or out of date, with the same rules as EnsureImage (see
// ensureImage).
func EnsureComboImage(ctx context.Context, exe *executor.Executor, stdout io.Writer, phpVersion, nodeVersion string) error {
	if !ValidPHPVersion(phpVersion) {
		return fmt.Errorf("versão PHP não suportada: %s", phpVersion)
	}
	if !ValidNodeVersion(nodeVersion) {
		return fmt.Errorf("versão Node não suportada: %s", nodeVersion)
	}
	return ensureImage(ctx, exe, stdout, imageBuild{
		name:  ComboImageName(phpVersion, nodeVersion),
		first: "primeira vez que essa combinação PHP+Node é usada",
		files: map[string]string{"Dockerfile": comboDockerfile, "php.ini": phpIni, "supervisord.conf": comboSupervisordConf},
		args: []string{"PHP_VERSION=" + phpVersion, "NODE_VERSION=" + nodeVersion,
			fmt.Sprintf("UID=%d", os.Getuid())},
	})
}
