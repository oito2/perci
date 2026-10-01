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
	"path/filepath"
	"strings"
	"testing"

	"github.com/oito2/perci/internal/config"
)

func TestAppDirs(t *testing.T) {
	if got, want := AppHTMLDir("/ws", "meuapp"), filepath.Join("/ws", "localhost", "html", "meuapp"); got != want {
		t.Errorf("AppHTMLDir() = %q, want %q", got, want)
	}
	if got, want := AppDataDir("/ws", "curso1"), filepath.Join("/ws", "localhost", "data", "curso1"); got != want {
		t.Errorf("AppDataDir() = %q, want %q", got, want)
	}
	if got, want := AppLogDir("/ws", "meuapp"), filepath.Join("/ws", "localhost", "logs", "meuapp"); got != want {
		t.Errorf("AppLogDir() = %q, want %q", got, want)
	}
	if got, want := AppComboAPIDir("/ws", "projeto"), filepath.Join("/ws", "localhost", "html", "projeto", "api"); got != want {
		t.Errorf("AppComboAPIDir() = %q, want %q", got, want)
	}
	if got, want := AppComboAppDir("/ws", "projeto"), filepath.Join("/ws", "localhost", "html", "projeto", "app"); got != want {
		t.Errorf("AppComboAppDir() = %q, want %q", got, want)
	}
}

func TestDBAccessEnv(t *testing.T) {
	got := dbAccessEnv(config.MariaDBConfig{DBUser: "dev_user", DBPass: "s3cr3t"})
	want := []string{
		"DB_HOST=" + MariaDBContainerName,
		"DB_PORT=3306",
		"DB_USER=dev_user",
		"DB_PASS=s3cr3t",
		"DB_NAME=dev_db",
	}
	if len(got) != len(want) {
		t.Fatalf("dbAccessEnv() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("dbAccessEnv()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestAppRunArgs_NoData(t *testing.T) {
	got := appRunArgs("meuapp", "perci-php82", "/ws/localhost/html/meuapp", "", "/ws/localhost/logs/meuapp", "/perci/php-conf/meuapp/zz-perci-overrides.ini", nil)

	joined := strings.Join(got, " ")
	for _, w := range []string{
		"--name", "meuapp",
		"--network", NetworkName,
		"/ws/localhost/html/meuapp:/var/www/html",
		"/ws/localhost/logs/meuapp:/var/log/php",
		"/perci/php-conf/meuapp/zz-perci-overrides.ini:" + phpConfMount + ":ro",
		"perci-php82",
	} {
		if !strings.Contains(joined, w) {
			t.Errorf("appRunArgs() missing %q in:\n%v", w, got)
		}
	}
	if strings.Contains(joined, "/var/www/data") {
		t.Errorf("expected no data volume when dataDir is empty, got:\n%v", got)
	}
	if got[len(got)-2] != "--" {
		t.Errorf("expected image to be preceded by \"--\", got %v", got[len(got)-3:])
	}
}

func TestAppRunArgs_WithDataAndDBEnv(t *testing.T) {
	got := appRunArgs("curso1", "perci-php82", "/ws/html/curso1", "/ws/data/curso1", "/ws/logs/curso1", "/perci/php-conf/curso1/zz-perci-overrides.ini",
		[]string{"DB_HOST=mariadb", "DB_USER=dev_user", "DB_PASS=s3cr3t"})

	joined := strings.Join(got, " ")
	if !strings.Contains(joined, "/ws/data/curso1:/var/www/data") {
		t.Errorf("expected data volume when dataDir is set, got:\n%v", got)
	}
	// Names only: docker copies the values from its client env, so the
	// credentials never show up in the process's command line.
	if !strings.Contains(joined, "-e DB_HOST -e DB_USER -e DB_PASS") {
		t.Errorf("expected DB env vars to be passed by name via -e, got:\n%v", got)
	}
	if strings.Contains(joined, "s3cr3t") || strings.Contains(joined, "=mariadb") {
		t.Errorf("DB values must not be in the argv, got:\n%v", got)
	}
}

func TestNodeAppRunArgs(t *testing.T) {
	got := nodeAppRunArgs("frontend", "perci-node24", "/ws/html/frontend", "npm run dev")

	joined := strings.Join(got, " ")
	for _, w := range []string{
		"--name", "frontend",
		"--network", NetworkName,
		"--user", "node",
		"/ws/html/frontend:/app",
		"perci-node24",
	} {
		if !strings.Contains(joined, w) {
			t.Errorf("nodeAppRunArgs() missing %q in:\n%v", w, got)
		}
	}
	if got[0] != "run" || got[1] != "-d" {
		t.Errorf("expected detached run, got args starting with %v", got[:2])
	}
	if got[len(got)-3] != "sh" || got[len(got)-2] != "-c" || got[len(got)-1] != "npm run dev" {
		t.Errorf("expected the image to be followed by \"sh -c <devCommand>\" as CMD, got tail %v", got[len(got)-3:])
	}
	// devCommand must land as a single argv element (not split by spaces)
	// so a multi-word command survives intact — confirmed above by the
	// exact tail check, not just a substring match.
}

func TestComboAppRunArgs(t *testing.T) {
	got := comboAppRunArgs("projeto", "perci-php82-node24", "/ws/html/projeto/api", "/ws/html/projeto/app", "/ws/logs/projeto",
		"/perci/php-conf/projeto/zz-perci-overrides.ini", "npm run dev", "php bin/console messenger:consume async",
		[]string{"DB_HOST=mariadb", "DB_USER=dev_user"})

	joined := strings.Join(got, " ")
	for _, w := range []string{
		"--name", "projeto",
		"--network", NetworkName,
		"/ws/html/projeto/api:/var/www/html",
		"/ws/html/projeto/app:/app",
		"/ws/logs/projeto:/var/log/php",
		"/perci/php-conf/projeto/zz-perci-overrides.ini:" + phpConfMount + ":ro",
		"-e DEV_COMMAND=npm run dev",
		"-e WORKER_COMMAND=php bin/console messenger:consume async",
		"-e DB_HOST -e DB_USER",
		"perci-php82-node24",
	} {
		if !strings.Contains(joined, w) {
			t.Errorf("comboAppRunArgs() missing %q in:\n%v", w, got)
		}
	}
	if got[0] != "run" || got[1] != "-d" {
		t.Errorf("expected detached run, got args starting with %v", got[:2])
	}
	if strings.Contains(joined, "--user") {
		t.Errorf("combo container must run as root (no --user flag) — supervisord itself needs root to drop privileges per-program, got:\n%v", got)
	}
}

func TestComboAppRunArgs_NoDBEnv(t *testing.T) {
	got := comboAppRunArgs("projeto", "perci-php82-node24", "/ws/api", "/ws/app", "/ws/logs", "/perci/php-conf/projeto/zz-perci-overrides.ini", "npm run dev", "", nil)
	joined := strings.Join(got, " ")
	if strings.Contains(joined, "DB_HOST") {
		t.Errorf("expected no DB_* env vars when dbEnv is nil, got:\n%v", got)
	}
	// WORKER_COMMAND must still be passed, even empty — [program:worker]'s
	// own shell wrapper reads it at run time (see comboSupervisordConf).
	if !strings.Contains(joined, "-e WORKER_COMMAND=") {
		t.Errorf("expected WORKER_COMMAND to always be passed (even empty), got:\n%v", got)
	}
}

func TestRemoveAppByFolder(t *testing.T) {
	existing := []config.AppContainer{
		{Folder: "app1", URL: "app1.localhost"},
		{Folder: "app2", URL: "app2.localhost"},
	}

	t.Run("removes matching folder", func(t *testing.T) {
		got := removeAppByFolder(existing, "app1")
		if len(got) != 1 || got[0].Folder != "app2" {
			t.Errorf("removeAppByFolder(existing, \"app1\") = %+v, want only app2 left", got)
		}
		if len(existing) != 2 {
			t.Errorf("expected original slice untouched, got len %d", len(existing))
		}
	})

	t.Run("no match leaves everything", func(t *testing.T) {
		got := removeAppByFolder(existing, "nope")
		if len(got) != 2 {
			t.Errorf("removeAppByFolder with no match = %+v, want both apps untouched", got)
		}
	})
}

func TestFindAppByFolder(t *testing.T) {
	apps := []config.AppContainer{
		{Folder: "app1", URL: "app1.localhost"},
		{Folder: "app2", URL: "app2.localhost"},
	}

	got, found := FindAppByFolder(apps, "app2")
	if !found || got.URL != "app2.localhost" {
		t.Errorf("FindAppByFolder(apps, \"app2\") = (%+v, %v), want (app2 entry, true)", got, found)
	}

	_, found = FindAppByFolder(apps, "nope")
	if found {
		t.Errorf("FindAppByFolder(apps, \"nope\") found = true, want false")
	}
}

func TestReplaceOrAppendApp(t *testing.T) {
	existing := []config.AppContainer{
		{Folder: "app1", URL: "app1.localhost", PHPVersion: "8.1"},
		{Folder: "app2", URL: "app2.localhost", PHPVersion: "8.2"},
	}

	t.Run("appends new", func(t *testing.T) {
		got := replaceOrAppendApp(existing, config.AppContainer{Folder: "app3", URL: "app3.localhost"})
		if len(got) != 3 {
			t.Fatalf("expected 3 apps, got %d", len(got))
		}
		if got[2].Folder != "app3" {
			t.Errorf("expected app3 appended, got %+v", got[2])
		}
		if len(existing) != 2 {
			t.Errorf("expected original slice untouched, got len %d", len(existing))
		}
	})

	t.Run("replaces existing by folder", func(t *testing.T) {
		got := replaceOrAppendApp(existing, config.AppContainer{Folder: "app1", URL: "app1.localhost", PHPVersion: "8.4"})
		if len(got) != 2 {
			t.Fatalf("expected 2 apps (replace, not append), got %d", len(got))
		}
		if got[0].PHPVersion != "8.4" {
			t.Errorf("expected app1 replaced with PHPVersion 8.4, got %+v", got[0])
		}
		if existing[0].PHPVersion != "8.1" {
			t.Errorf("expected original slice untouched, got %+v", existing[0])
		}
	})
}
