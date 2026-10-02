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
	"strings"
	"testing"
)

func TestComboImageName(t *testing.T) {
	if got, want := ComboImageName("8.2", "24"), "perci-php82-node24"; got != want {
		t.Errorf("ComboImageName(%q, %q) = %q, want %q", "8.2", "24", got, want)
	}
}

func TestComboSupervisordConf(t *testing.T) {
	// [supervisord] is mandatory: comboDockerfile's CMD passes this file
	// straight to supervisord's -c, so it must be a complete config, not
	// just a program-definitions fragment.
	if !strings.HasPrefix(comboSupervisordConf, "[supervisord]") {
		t.Errorf("expected comboSupervisordConf to start with a [supervisord] section, got:\n%s", comboSupervisordConf)
	}
	if !strings.Contains(comboSupervisordConf, "command=php-fpm -F") {
		t.Errorf("missing php-fpm program, got:\n%s", comboSupervisordConf)
	}
	if !strings.Contains(comboSupervisordConf, "command=%(ENV_DEV_COMMAND)s") {
		t.Errorf("missing dev-server program driven by the DEV_COMMAND env var, got:\n%s", comboSupervisordConf)
	}
	if !strings.Contains(comboSupervisordConf, "directory=/app") {
		t.Errorf("expected the dev-server program to run from /app, got:\n%s", comboSupervisordConf)
	}
	if !strings.Contains(comboSupervisordConf, "user=www-data") {
		t.Errorf("expected the dev-server program to drop to www-data (UID-matched to the host), got:\n%s", comboSupervisordConf)
	}
	// php-fpm's [program:] block must NOT set user=: its master process
	// needs to start as root and drops its own workers to www-data
	// internally via its pool config.
	phpFPMBlock := comboSupervisordConf[:strings.Index(comboSupervisordConf, "[program:dev-server]")]
	if strings.Contains(phpFPMBlock, "user=") {
		t.Errorf("php-fpm's program block must not set user= (its master process needs to start as root), got:\n%s", phpFPMBlock)
	}

	// [program:worker] must exist and read $WORKER_COMMAND from its own
	// inherited environment at run time (a plain shell expansion, not
	// supervisord's %(ENV_X)s config-parse-time substitution), so it starts
	// cleanly even when the app sets no WorkerCommand at all.
	if !strings.Contains(comboSupervisordConf, "[program:worker]") {
		t.Errorf("missing [program:worker] section, got:\n%s", comboSupervisordConf)
	}
	if !strings.Contains(comboSupervisordConf, `$WORKER_COMMAND`) {
		t.Errorf("expected the worker program to read WORKER_COMMAND from its shell environment, got:\n%s", comboSupervisordConf)
	}
	if !strings.Contains(comboSupervisordConf, "sleep infinity") {
		t.Errorf("expected the worker program to idle (sleep infinity) rather than fail to start when WORKER_COMMAND is empty, got:\n%s", comboSupervisordConf)
	}
}

func TestComboDockerfile(t *testing.T) {
	if !strings.Contains(comboDockerfile, "FROM php:${PHP_VERSION}-fpm") {
		t.Errorf("expected comboDockerfile to build on the same PHP base as the standalone PHP image, got:\n%s", comboDockerfile)
	}
	if !strings.Contains(comboDockerfile, "COPY php.ini /usr/local/etc/php/php.ini") {
		t.Errorf("expected comboDockerfile to reuse the tuned php.ini (same as EnsureImage), got:\n%s", comboDockerfile)
	}
	if !strings.Contains(comboDockerfile, "setup_${NODE_VERSION}.x") {
		t.Errorf("expected comboDockerfile to install Node via the NodeSource setup script for the chosen version, got:\n%s", comboDockerfile)
	}
	if !strings.Contains(comboDockerfile, "supervisor") {
		t.Errorf("expected comboDockerfile to install the supervisor package, got:\n%s", comboDockerfile)
	}
	if !strings.Contains(comboDockerfile, `COPY supervisord.conf /etc/supervisor/conf.d/supervisord.conf`) {
		t.Errorf("expected comboDockerfile to bake in supervisord.conf, got:\n%s", comboDockerfile)
	}
	if !strings.Contains(comboDockerfile, `CMD ["supervisord", "-n", "-c", "/etc/supervisor/conf.d/supervisord.conf"]`) {
		t.Errorf("expected comboDockerfile to override CMD to launch supervisord instead of php-fpm directly, got:\n%s", comboDockerfile)
	}
}
