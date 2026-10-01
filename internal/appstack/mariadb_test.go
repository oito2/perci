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
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/oito2/perci/internal/config"
)

func TestMariaDBDataDir(t *testing.T) {
	got := MariaDBDataDir("/home/u/workspace")
	want := filepath.Join("/home/u/workspace", "localhost", "databases", "mariadb")
	if got != want {
		t.Errorf("MariaDBDataDir() = %q, want %q", got, want)
	}
}

func TestMariaDBInitDir(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	got, err := MariaDBInitDir()
	if err != nil {
		t.Fatalf("MariaDBInitDir: %v", err)
	}
	want := filepath.Join(tmp, ".perci", "appstack", "mariadb", "init")
	if got != want {
		t.Errorf("MariaDBInitDir() = %q, want %q", got, want)
	}
}

func TestWriteGrantSQL(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	dir, err := writeGrantSQL("dev_user")
	if err != nil {
		t.Fatalf("writeGrantSQL: %v", err)
	}

	wantDir := filepath.Join(tmp, ".perci", "appstack", "mariadb", "init")
	if dir != wantDir {
		t.Errorf("writeGrantSQL() dir = %q, want %q", dir, wantDir)
	}

	data, err := os.ReadFile(filepath.Join(dir, "01-permissions.sql"))
	if err != nil {
		t.Fatalf("read written sql: %v", err)
	}
	got := string(data)
	if !strings.Contains(got, "GRANT ALL PRIVILEGES ON *.* TO `dev_user`@'%' WITH GRANT OPTION;") {
		t.Errorf("missing expected GRANT statement, got:\n%s", got)
	}
	if !strings.Contains(got, "FLUSH PRIVILEGES;") {
		t.Errorf("missing FLUSH PRIVILEGES, got:\n%s", got)
	}
}

func TestMariaDBRunArgs(t *testing.T) {
	env := mariadbEnv("dev_user", "dev_pass", "root_pass")
	got := mariadbRunArgs(
		"/home/u/workspace/localhost/databases/mariadb",
		"/home/u/.perci/appstack/mariadb/init",
		env,
	)

	wantContains := []string{
		"--name", MariaDBContainerName,
		"--network", NetworkName,
		"-p", "127.0.0.1:3306:3306",
		"/home/u/workspace/localhost/databases/mariadb:/var/lib/mysql",
		"/home/u/.perci/appstack/mariadb/init:/docker-entrypoint-initdb.d",
		"-e MYSQL_ROOT_PASSWORD -e MYSQL_USER -e MYSQL_PASSWORD",
		"MYSQL_DATABASE=dev_db",
		MariaDBImage,
	}
	joined := strings.Join(got, " ")
	for _, w := range wantContains {
		if !strings.Contains(joined, w) {
			t.Errorf("mariadbRunArgs() missing %q in:\n%v", w, got)
		}
	}
	// Credentials travel only through the client env (executor.Options.Env).
	for _, secret := range []string{"dev_pass", "root_pass", "dev_user"} {
		if strings.Contains(joined, secret) {
			t.Errorf("credential %q must not be in the argv:\n%v", secret, got)
		}
	}
	for _, want := range []string{"MYSQL_ROOT_PASSWORD=root_pass", "MYSQL_USER=dev_user", "MYSQL_PASSWORD=dev_pass"} {
		if !slices.Contains(env, want) {
			t.Errorf("mariadbEnv() missing %q: %v", want, env)
		}
	}
	if got[0] != "run" || got[1] != "-d" {
		t.Errorf("expected detached run, got args starting with %v", got[:2])
	}
}

func TestValidDBIdentifierReuse(t *testing.T) {
	// ValidDBIdentifier must accept a typical MariaDB user identifier and
	// reject anything with spaces, hyphens, or over 32 characters.
	tests := []struct {
		in   string
		want bool
	}{
		{"dev_user", true},
		{"admin", true},
		{"", false},
		{"has space", false},
		{"has-hyphen", false},
		{strings.Repeat("a", 33), false},
	}
	for _, tt := range tests {
		if got := ValidDBIdentifier.MatchString(tt.in); got != tt.want {
			t.Errorf("ValidDBIdentifier.MatchString(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestCheckMariaDBCredentials(t *testing.T) {
	workspace := t.TempDir()
	dataDir := MariaDBDataDir(workspace)
	known := &config.Config{Docker: config.DockerConfig{MariaDB: config.MariaDBConfig{DataUser: "dev_user", DataPass: "s3cr3t"}}}

	// Empty data directory: any credentials are fine.
	if err := checkMariaDBCredentials(io.Discard, known, workspace, "other", "x"); err != nil {
		t.Fatalf("uninitialized data dir must accept new credentials: %v", err)
	}

	if err := os.MkdirAll(filepath.Join(dataDir, "mysql"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := checkMariaDBCredentials(io.Discard, known, workspace, "dev_user", "s3cr3t"); err != nil {
		t.Errorf("same credentials must pass: %v", err)
	}
	if err := checkMariaDBCredentials(io.Discard, known, workspace, "dev_user", "changed"); err == nil {
		t.Error("a different password over initialized data must be refused")
	}
	if err := checkMariaDBCredentials(io.Discard, known, workspace, "new_user", "s3cr3t"); err == nil {
		t.Error("a different user over initialized data must be refused")
	}

	// Legacy config with no recorded credentials: can't check, warn only.
	var out strings.Builder
	if err := checkMariaDBCredentials(&out, &config.Config{}, workspace, "any", "x"); err != nil {
		t.Errorf("unknown credentials must only warn: %v", err)
	}
	if !strings.Contains(out.String(), "não sabe") {
		t.Errorf("expected a warning, got %q", out.String())
	}
}
