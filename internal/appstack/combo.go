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
// combo, e.g. "perci-php82-node24" for ("8.2", "24") — built once per
// (PHP version, Node version) pair and reused by every AppTypePHPNode
// container on that pair.
func ComboImageName(phpVersion, nodeVersion string) string {
	return "perci-php" + phpTag(phpVersion) + "-node" + nodeVersion
}

// comboSupervisordConf is baked into every combo image as-is — its content
// never varies per app, only the DEV_COMMAND environment variable
// supervisord itself resolves at container-start time does: supervisord's
// own %(ENV_X)s syntax reads literally from the process environment
// (confirmed against supervisord.org's docs, 2026-08-21 — "if you reference an environment
// variable that doesn't exist, supervisord will fail to start", so
// comboAppRunArgs must always pass -e DEV_COMMAND=...). That's why this can
// be a build-time constant instead of something written per-app and
// bind-mounted the way nginx.conf/01-permissions.sql are: the *value* is
// per-container, but the *config* isn't.
//
// [program:php-fpm] has no user= — php-fpm's master process needs to start
// as root (same as it already implicitly does for every other PHP-family
// container here), and its own pool config (inherited from the official
// php-fpm image) already drops its *worker* processes to www-data
// internally. [program:dev-server] does set user=www-data — npm/vite has
// no privilege-dropping of its own, and www-data's UID is already
// usermod'd to match the host in phpDockerfile (image.go), so this reuses
// that existing user instead of creating a second one just for Node.
//
// command=%(ENV_DEV_COMMAND)s is NOT run through a shell — supervisord
// splits it into argv the way a shell would (respecting quotes), but never
// interprets shell operators (&&, ;, pipes, $VAR). Simple commands
// ("npm run dev") work directly; anyone who needs a compound command can
// still type e.g. `sh -c "cd /app && npm run dev"` as their own
// DevCommand — supervisord's quoting handles that correctly too, since the
// whole quoted string becomes one argv element passed to sh -c.
//
// [supervisord] is mandatory — confirmed against a real "Error: .ini file
// does not include supervisord section" from every AppTypePHPNode
// container during testing (2026-08-24): comboDockerfile's CMD
// points supervisord's -c straight at this file (not Debian's own
// /etc/supervisor/supervisord.conf, which carries that section plus an
// [include] pulling in conf.d/*.conf), so this file has to be a complete,
// self-contained config, not just a program-definitions fragment.
// nodaemon isn't set here — comboDockerfile's CMD already passes
// supervisord its own -n flag for that.
//
// [program:worker] is an optional extra background process — a queue/job
// consumer such as Symfony Messenger or a Laravel queue worker
// (config.AppContainer.WorkerCommand). It's baked into every combo image
// unconditionally (like [program:dev-server]) because the image is shared
// across every AppTypePHPNode container on that (PHP, Node) pair — it
// can't be conditionally included per app at build time. Unlike
// dev-server, though, its command isn't `%(ENV_WORKER_COMMAND)s` directly:
// supervisord requires every %(ENV_X)s it references to exist in the
// process environment or it refuses to start at all (confirmed against
// supervisord.org's docs, 2026-08-21 — see dev-server's own comment above),
// which would break every combo app that leaves WorkerCommand empty (the
// common case). So this program's own `command=` is a fixed shell wrapper
// that reads $WORKER_COMMAND from its inherited container environment at
// *run* time instead of supervisord's config-parse-time substitution —
// comboAppRunArgs always sets that env var (possibly to an empty string).
// When empty, the wrapper execs `sleep infinity` — an idle placeholder,
// not a real workload — instead of leaving the field unset entirely, so
// the program still starts cleanly and never busy-loop-restarts. When set,
// it runs through its own sh -c, so a command with variables, && or a pipe
// works as typed (a bare exec $WORKER_COMMAND word-split it). Found
// missing while working on the learnerflow app's async job queue,
// 2026-08-28.
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

// comboDockerfile extends phpImageDockerfile (PHP + php.ini, the same
// foundation the standalone PHP image already builds — see image.go) with
// Node (via the official NodeSource apt setup script — the standard way to
// install a specific Node major version via apt on Debian, confirmed
// 2026-08-21) and supervisor (Debian's own `supervisor` package), plus the
// baked-in supervisord.conf above. CMD is overridden to launch supervisord
// instead of php-fpm directly — supervisord then starts php-fpm itself as
// one of its two managed programs. The base image's ENTRYPOINT
// (docker-php-entrypoint, inherited from the official php-fpm image via
// phpDockerfile) is untouched: it execs whatever CMD it's given verbatim
// once its argv doesn't match one of its own recognized php-specific
// patterns, which "supervisord" doesn't.
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
// it's missing or out of date — same rules as EnsureImage (see
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
