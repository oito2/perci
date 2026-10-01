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

package db

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oito2/perci/internal/executor"
)

// fakeDocker puts a `docker` script first on PATH. `inspect` reports
// status; `exec` records its arguments, the env file's content and its
// stdin into dir, prints "DUMP" and exits with code.
func fakeDocker(t *testing.T, status string, code int) string {
	t.Helper()
	dir := t.TempDir()
	script := `#!/bin/sh
if [ "$1" = inspect ]; then printf '%s\n' '` + status + `'; exit 0; fi
printf '%s\n' "$*" > '` + dir + `/args'
while [ $# -gt 0 ]; do [ "$1" = --env-file ] && cat "$2" > '` + dir + `/env'; shift; done
[ -t 0 ] || cat > '` + dir + `/stdin'
printf 'DUMP'
exit ` + string(rune('0'+code)) + `
`
	if err := os.WriteFile(filepath.Join(dir, "docker"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return dir
}

// The dump lands in dest (0600) with the consistency flags; the password
// only ever travels in a 0600 env file that is removed afterwards.
func TestBackup_WritesDump(t *testing.T) {
	rec := fakeDocker(t, "running", 0)
	dest := filepath.Join(t.TempDir(), "backup.sql")
	var buf bytes.Buffer

	if err := Backup(context.Background(), executor.New(&buf, &buf), &buf, "mariadb", "dev", "s3cr3t", dest); err != nil {
		t.Fatalf("Backup: %v\n%s", err, buf.String())
	}
	if b, _ := os.ReadFile(dest); string(b) != "DUMP" {
		t.Errorf("dump = %q", b)
	}
	if info, _ := os.Stat(dest); info.Mode().Perm() != 0o600 {
		t.Errorf("dump mode = %v, want 0600", info.Mode().Perm())
	}
	args, _ := os.ReadFile(filepath.Join(rec, "args"))
	for _, want := range []string{"mariadb-dump", "--single-transaction", "--routines", "--events"} {
		if !strings.Contains(string(args), want) {
			t.Errorf("args missing %s: %s", want, args)
		}
	}
	if strings.Contains(string(args), "s3cr3t") {
		t.Error("the password must not be on the command line")
	}
	if env, _ := os.ReadFile(filepath.Join(rec, "env")); string(env) != "MYSQL_PWD=s3cr3t\n" {
		t.Errorf("env file = %q", env)
	}
}

// A failed dump leaves no partial backup behind.
func TestBackup_FailureRemovesDest(t *testing.T) {
	fakeDocker(t, "running", 1)
	dest := filepath.Join(t.TempDir(), "backup.sql")
	var buf bytes.Buffer
	if err := Backup(context.Background(), executor.New(&buf, &buf), &buf, "mariadb", "dev", "x", dest); err == nil {
		t.Fatal("expected an error")
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Error("partial backup left behind")
	}
}

// A stopped container is refused before anything runs.
func TestBackup_ContainerNotRunning(t *testing.T) {
	rec := fakeDocker(t, "exited", 0)
	var buf bytes.Buffer
	err := Backup(context.Background(), executor.New(&buf, &buf), &buf, "mariadb", "dev", "x", filepath.Join(t.TempDir(), "b.sql"))
	if err == nil || !strings.Contains(err.Error(), "não está em execução") {
		t.Fatalf("err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(rec, "args")); !os.IsNotExist(err) {
		t.Error("docker exec ran against a stopped container")
	}
}

// Restore feeds the backup file to mariadb on stdin.
func TestRestore_FeedsFileOnStdin(t *testing.T) {
	rec := fakeDocker(t, "running", 0)
	src := filepath.Join(t.TempDir(), "backup.sql")
	if err := os.WriteFile(src, []byte("CREATE TABLE t (id int);"), 0o600); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := Restore(context.Background(), executor.New(&buf, &buf), &buf, "mariadb", "dev", "x", src); err != nil {
		t.Fatalf("Restore: %v\n%s", err, buf.String())
	}
	if in, _ := os.ReadFile(filepath.Join(rec, "stdin")); string(in) != "CREATE TABLE t (id int);" {
		t.Errorf("stdin = %q", in)
	}
	if args, _ := os.ReadFile(filepath.Join(rec, "args")); !strings.Contains(string(args), "exec -i") {
		t.Errorf("args = %s", args)
	}
}
