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
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oito2/perci/internal/config"
)

func TestExportConfig_NothingToExport(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	if err := ExportConfig(filepath.Join(tmp, "export.yaml")); err == nil {
		t.Error("ExportConfig() with an empty cfg.Docker = nil error, want an error")
	}
}

func TestExportConfig_RoundTrip(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	want := config.Config{
		WorkspacePath: "/srv/workspace",
		Docker: config.DockerConfig{
			NginxCreated: true,
			MariaDB: config.MariaDBConfig{
				DBUser:     "dev_user",
				DBPass:     "s3cr3t",
				DBRootPass: "r00t",
			},
			Apps: []config.AppContainer{
				{Name: "Curso 1", Folder: "curso1", Type: config.AppTypeMoodle, URL: "curso1.localhost", PHPVersion: "8.2", DBAccess: true},
			},
		},
	}
	if err := config.Update(func(c *config.Config) error { *c = want; return nil }); err != nil {
		t.Fatalf("seed config.Save: %v", err)
	}

	exportPath := filepath.Join(tmp, "export.yaml")
	if err := ExportConfig(exportPath); err != nil {
		t.Fatalf("ExportConfig: %v", err)
	}

	// Sensitive content (MariaDB credentials): the file must be 0600.
	info, err := os.Stat(exportPath)
	if err != nil {
		t.Fatalf("stat export: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("export file mode = %o, want 0600", perm)
	}

	got, err := LoadExportedConfig(exportPath)
	if err != nil {
		t.Fatalf("LoadExportedConfig: %v", err)
	}
	if got.WorkspacePath != want.WorkspacePath {
		t.Errorf("WorkspacePath: got %q, want %q", got.WorkspacePath, want.WorkspacePath)
	}
	if got.Docker.NginxCreated != want.Docker.NginxCreated {
		t.Errorf("Docker.NginxCreated: got %v, want %v", got.Docker.NginxCreated, want.Docker.NginxCreated)
	}
	if got.Docker.MariaDB != want.Docker.MariaDB {
		t.Errorf("Docker.MariaDB: got %+v, want %+v", got.Docker.MariaDB, want.Docker.MariaDB)
	}
	if len(got.Docker.Apps) != 1 || got.Docker.Apps[0] != want.Docker.Apps[0] {
		t.Errorf("Docker.Apps: got %+v, want %+v", got.Docker.Apps, want.Docker.Apps)
	}
}

func TestLoadExportedConfig_MissingFile(t *testing.T) {
	if _, err := LoadExportedConfig("/does/not/exist.yaml"); err == nil {
		t.Error("LoadExportedConfig(missing file) = nil error, want an error")
	}
}

func TestLoadExportedConfig_InvalidYAML(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "bad.yaml")
	if err := os.WriteFile(path, []byte("not: valid: yaml: [\n"), 0o600); err != nil {
		t.Fatalf("seed bad yaml: %v", err)
	}
	if _, err := LoadExportedConfig(path); err == nil {
		t.Error("LoadExportedConfig(invalid yaml) = nil error, want an error")
	}
}

func TestResolveImportWorkspace(t *testing.T) {
	t.Run("adopts exported path when local is unset", func(t *testing.T) {
		got, err := resolveImportWorkspace("", "/srv/workspace")
		if err != nil {
			t.Fatalf("resolveImportWorkspace: %v", err)
		}
		if got != "/srv/workspace" {
			t.Errorf("got %q, want /srv/workspace", got)
		}
	})

	t.Run("keeps local when it matches exported", func(t *testing.T) {
		got, err := resolveImportWorkspace("/srv/workspace", "/srv/workspace")
		if err != nil {
			t.Fatalf("resolveImportWorkspace: %v", err)
		}
		if got != "/srv/workspace" {
			t.Errorf("got %q, want /srv/workspace", got)
		}
	})

	t.Run("keeps local when exported is unset", func(t *testing.T) {
		got, err := resolveImportWorkspace("/srv/workspace", "")
		if err != nil {
			t.Fatalf("resolveImportWorkspace: %v", err)
		}
		if got != "/srv/workspace" {
			t.Errorf("got %q, want /srv/workspace", got)
		}
	})

	t.Run("refuses on mismatch", func(t *testing.T) {
		_, err := resolveImportWorkspace("/home/u/workspace", "/srv/workspace")
		if !errors.Is(err, ErrWorkspaceMismatch) {
			t.Errorf("resolveImportWorkspace mismatch error = %v, want errors.Is(_, ErrWorkspaceMismatch)", err)
		}
		if !strings.Contains(err.Error(), "/srv/workspace") || !strings.Contains(err.Error(), "/home/u/workspace") {
			t.Errorf("expected both paths named in the error, got: %v", err)
		}
	})
}

func validPHPApp(folder, url string) config.AppContainer {
	return config.AppContainer{Name: folder, Folder: folder, URL: url, Type: config.AppTypePHP, PHPVersion: "8.2"}
}

func TestValidateApp_ReservedFolders(t *testing.T) {
	for _, folder := range []string{"nginx", "mariadb", "NGINX"} {
		if err := ValidateApp(validPHPApp(folder, "x.localhost")); err == nil {
			t.Errorf("folder %q collides with an infrastructure container and must be refused", folder)
		}
	}
	if err := ValidateApp(validPHPApp("meuapp", "meuapp.localhost")); err != nil {
		t.Errorf("a normal app must be valid: %v", err)
	}
}

func TestCheckURLUnique(t *testing.T) {
	apps := []config.AppContainer{validPHPApp("a", "site.localhost")}
	if err := checkURLUnique(validPHPApp("b", "SITE.localhost"), apps); err == nil {
		t.Error("a URL already used by another app must be refused")
	}
	if err := checkURLUnique(validPHPApp("a", "site.localhost"), apps); err != nil {
		t.Errorf("an app keeping its own URL (edit/recreate) must pass: %v", err)
	}
}

// Nothing from an invalid file may be applied: validateExported runs first.
func TestValidateExported(t *testing.T) {
	ok := ExportedConfig{WorkspacePath: "/home/u/workspace", Docker: config.DockerConfig{
		MariaDB: config.MariaDBConfig{DBUser: "dev_user"},
		Apps:    []config.AppContainer{validPHPApp("a", "a.localhost"), validPHPApp("b", "b.localhost")},
	}}
	if err := validateExported(ok); err != nil {
		t.Fatalf("valid export rejected: %v", err)
	}

	bad := map[string]func(*ExportedConfig){
		"relative workspace": func(e *ExportedConfig) { e.WorkspacePath = "workspace" },
		"bad db user":        func(e *ExportedConfig) { e.Docker.MariaDB.DBUser = "x; DROP" },
		"invalid app":        func(e *ExportedConfig) { e.Docker.Apps[0].Folder = "../etc" },
		"duplicate folder":   func(e *ExportedConfig) { e.Docker.Apps[1].Folder = "a" },
		"duplicate url":      func(e *ExportedConfig) { e.Docker.Apps[1].URL = "a.localhost" },
		"reserved folder":    func(e *ExportedConfig) { e.Docker.Apps[0].Folder = "nginx" },
	}
	for name, mutate := range bad {
		e := ok
		e.Docker.Apps = append([]config.AppContainer(nil), ok.Docker.Apps...)
		mutate(&e)
		if err := validateExported(e); err == nil {
			t.Errorf("%s: expected validateExported to refuse", name)
		}
	}
}
