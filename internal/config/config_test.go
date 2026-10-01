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

package config_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/oito2/perci/internal/config"
)

// Without a config file Load returns an empty Config — WorkspacePath
// stays "" (not chosen yet) — and Workspace resolves ~/workspace.
func TestLoadWithoutFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.WorkspacePath != "" {
		t.Errorf("WorkspacePath = %q, want empty", cfg.WorkspacePath)
	}
	if ws, err := cfg.Workspace(); err != nil || ws != filepath.Join(home, "workspace") {
		t.Errorf("Workspace() = %q, %v", ws, err)
	}
	cfg.WorkspacePath = "/srv/ws"
	if ws, _ := cfg.Workspace(); ws != "/srv/ws" {
		t.Errorf("Workspace() = %q, want the configured path", ws)
	}
}

// saveConfig writes cfg through config.Update, the only write path.
func saveConfig(cfg *config.Config) error {
	return config.Update(func(c *config.Config) error {
		*c = *cfg
		return nil
	})
}

func TestSaveLoad(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	want := &config.Config{
		WorkspacePath: "/srv/workspace",
	}

	if err := saveConfig(want); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.WorkspacePath != want.WorkspacePath {
		t.Errorf("WorkspacePath: got %q, want %q", got.WorkspacePath, want.WorkspacePath)
	}
}

func TestSaveCreatesDir(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	if err := saveConfig(&config.Config{WorkspacePath: "/tmp/ws"}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	configFile := filepath.Join(tmp, ".perci", "config.yaml")
	if _, err := os.Stat(configFile); err != nil {
		t.Errorf("config file not found at %s: %v", configFile, err)
	}
}

func TestExpandPath(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	got, err := config.ExpandPath("~/projects")
	if err != nil {
		t.Fatalf("ExpandPath: %v", err)
	}
	want := filepath.Join(tmp, "projects")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSaveLoadDockerConfig(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	want := &config.Config{
		WorkspacePath: "/srv/workspace",
		Docker: config.DockerConfig{
			NginxCreated: true,
			MariaDB: config.MariaDBConfig{
				DBUser:     "moodle",
				DBPass:     "secret",
				DBRootPass: "rootsecret",
			},
			Apps: []config.AppContainer{
				{
					Name:       "Meu Curso",
					Folder:     "meu-curso",
					Type:       config.AppTypeMoodle,
					URL:        "meu-curso.localhost",
					PHPVersion: "8.3",
					DBAccess:   true,
				},
				{
					Name:       "API",
					Folder:     "api",
					Type:       config.AppTypeGeneric,
					URL:        "api.localhost",
					PHPVersion: "8.4",
					DBAccess:   false,
				},
			},
		},
	}

	if err := saveConfig(want); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if got.Docker.NginxCreated != want.Docker.NginxCreated {
		t.Errorf("Docker.NginxCreated: got %v, want %v", got.Docker.NginxCreated, want.Docker.NginxCreated)
	}
	if got.Docker.MariaDB != want.Docker.MariaDB {
		t.Errorf("Docker.MariaDB: got %+v, want %+v", got.Docker.MariaDB, want.Docker.MariaDB)
	}
	if len(got.Docker.Apps) != len(want.Docker.Apps) {
		t.Fatalf("Docker.Apps: got %d entries, want %d", len(got.Docker.Apps), len(want.Docker.Apps))
	}
	for i := range want.Docker.Apps {
		if got.Docker.Apps[i] != want.Docker.Apps[i] {
			t.Errorf("Docker.Apps[%d]: got %+v, want %+v", i, got.Docker.Apps[i], want.Docker.Apps[i])
		}
	}
}

func TestSaveLoadNodeAppFields(t *testing.T) {
	// AppTypeNode/AppTypePHPNode round-trip through NodeVersion/DevCommand/
	// DevPort, same as every other AppContainer field.
	t.Setenv("HOME", t.TempDir())

	want := &config.Config{
		WorkspacePath: "/srv/workspace",
		Docker: config.DockerConfig{
			Apps: []config.AppContainer{
				{
					Name:        "Frontend",
					Folder:      "meuapp",
					Type:        config.AppTypeNode,
					URL:         "meuapp.localhost",
					NodeVersion: "24",
					DevCommand:  "npm run dev",
					DevPort:     5173,
				},
				{
					Name:        "Projeto Fullstack",
					Folder:      "projeto",
					Type:        config.AppTypePHPNode,
					URL:         "projeto.localhost",
					PHPVersion:  "8.3",
					NodeVersion: "22",
					DevCommand:  "npm run dev",
					DevPort:     5173,
					DBAccess:    true,
				},
			},
		},
	}

	if err := saveConfig(want); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got.Docker.Apps) != len(want.Docker.Apps) {
		t.Fatalf("Docker.Apps: got %d entries, want %d", len(got.Docker.Apps), len(want.Docker.Apps))
	}
	for i := range want.Docker.Apps {
		if got.Docker.Apps[i] != want.Docker.Apps[i] {
			t.Errorf("Docker.Apps[%d]: got %+v, want %+v", i, got.Docker.Apps[i], want.Docker.Apps[i])
		}
	}
}

func TestDockerConfigOmittedWhenEmpty(t *testing.T) {
	// A config saved before this field existed — or with no appstack usage
	// yet — must round-trip without ever growing a `docker:` section on
	// disk.
	t.Setenv("HOME", t.TempDir())

	if err := saveConfig(&config.Config{WorkspacePath: "/srv/workspace"}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	path := filepath.Join(os.Getenv("HOME"), ".perci", "config.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if strings.Contains(string(data), "docker:") {
		t.Errorf("expected no docker: section in config.yaml when Docker is empty, got:\n%s", data)
	}
}

func TestExpandPathBareTilde(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	got, err := config.ExpandPath("~")
	if err != nil {
		t.Fatalf("ExpandPath: %v", err)
	}
	if got != tmp {
		t.Errorf("ExpandPath(\"~\") = %q, want %q", got, tmp)
	}
}

func TestExpandPathAbsolute(t *testing.T) {
	got, err := config.ExpandPath("/absolute/path")
	if err != nil {
		t.Fatalf("ExpandPath: %v", err)
	}
	if got != "/absolute/path" {
		t.Errorf("absolute path should be unchanged, got %q", got)
	}
}

func TestGUIThemeOrDefault(t *testing.T) {
	cases := []struct {
		name     string
		guiTheme string
		want     string
	}{
		{name: "unset falls back to default", guiTheme: "", want: config.DefaultGUITheme},
		{name: "valid theme kept as-is", guiTheme: "synthwave", want: "synthwave"},
		{
			// A TUI theme name (internal/theme.Available) must never leak
			// through here — GUITheme and Theme are deliberately separate
			// fields/namespaces.
			name:     "unrecognized name falls back to default",
			guiTheme: "Dracula",
			want:     config.DefaultGUITheme,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{GUITheme: tc.guiTheme}
			if got := cfg.GUIThemeOrDefault(); got != tc.want {
				t.Errorf("GUIThemeOrDefault() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSidebarLogoOrDefault(t *testing.T) {
	cases := []struct {
		name, sidebarLogo, want string
	}{
		{"unset falls back to default", "", config.DefaultSidebarLogo},
		{"valid value kept as-is", "pink", "pink"},
		{"unrecognized value falls back to default", "green", config.DefaultSidebarLogo},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{SidebarLogo: tc.sidebarLogo}
			if got := cfg.SidebarLogoOrDefault(); got != tc.want {
				t.Errorf("SidebarLogoOrDefault() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestAppIconOrDefault(t *testing.T) {
	cases := []struct {
		name, appIcon, want string
	}{
		{"unset falls back to default", "", config.DefaultAppIcon},
		{"valid value kept as-is", "pink", "pink"},
		{"unrecognized value falls back to default", "green", config.DefaultAppIcon},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{AppIcon: tc.appIcon}
			if got := cfg.AppIconOrDefault(); got != tc.want {
				t.Errorf("AppIconOrDefault() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestPruneMissingRepoFolders(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	existing := t.TempDir()
	missing := filepath.Join(t.TempDir(), "sumiu")

	if err := saveConfig(&config.Config{RepoFolders: []string{existing, missing}}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if err := config.PruneMissingRepoFolders(); err != nil {
		t.Fatalf("PruneMissingRepoFolders: %v", err)
	}

	got, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if want := []string{existing}; len(got.RepoFolders) != len(want) || got.RepoFolders[0] != want[0] {
		t.Errorf("RepoFolders after prune = %v, want %v", got.RepoFolders, want)
	}
}

func TestGUIThemesContainsDecidedSet(t *testing.T) {
	// The exact 8 themes decided with the user on 2026-09-10 — locking this
	// down so an accidental edit to GUIThemes() doesn't silently drop or add
	// a theme.
	want := []string{"light", "dark", "cupcake", "synthwave", "retro", "valentine", "halloween", "garden"}
	got := config.GUIThemes()
	if len(got) != len(want) {
		t.Fatalf("GUIThemes() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("GUIThemes()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// config.yaml carries credentials: the file must be 0600 and ~/.perci 0700,
// including a pre-existing 0755 directory from older versions.
func TestSave_TightensPermissions(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".perci")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := saveConfig(&config.Config{WorkspacePath: "/tmp/ws"}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	dirInfo, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := dirInfo.Mode().Perm(); got != 0o700 {
		t.Errorf("~/.perci mode = %o, want 700", got)
	}
	fileInfo, err := os.Stat(filepath.Join(dir, "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if got := fileInfo.Mode().Perm(); got != 0o600 {
		t.Errorf("config.yaml mode = %o, want 600", got)
	}
}

// Update serializes load-mutate-save: concurrent appends are never lost.
func TestUpdate_ConcurrentAppendsAreNotLost(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	const n = 50
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if err := config.Update(func(cfg *config.Config) error {
				cfg.RepoFolders = append(cfg.RepoFolders, fmt.Sprintf("/repo/%d", i))
				return nil
			}); err != nil {
				t.Errorf("Update: %v", err)
			}
		}(i)
	}
	wg.Wait()

	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.RepoFolders) != n {
		t.Errorf("got %d repo folders, want %d", len(cfg.RepoFolders), n)
	}
}

// A corrupted config.yaml is an error, never silently replaced by defaults.
func TestLoad_InvalidYAML(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".perci"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".perci", "config.yaml"), []byte("docker: [unclosed"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := config.Load(); err == nil {
		t.Fatal("expected a parse error")
	}
}

// When mutate fails, Update writes nothing.
func TestUpdate_MutateErrorWritesNothing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := saveConfig(&config.Config{WorkspacePath: "/keep"}); err != nil {
		t.Fatal(err)
	}
	wantErr := errors.New("boom")
	err := config.Update(func(c *config.Config) error {
		c.WorkspacePath = "/changed"
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want the mutate error", err)
	}
	cfg, err := config.Load()
	if err != nil || cfg.WorkspacePath != "/keep" {
		t.Errorf("config changed: %+v, %v", cfg, err)
	}
}
