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
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/oito2/perci/internal/config"
	"github.com/oito2/perci/internal/executor"
	"github.com/oito2/perci/internal/fsutil"
	"github.com/oito2/perci/internal/ui"
)

// ExportedConfig is the file format ExportConfig writes and ImportConfig
// reads: a portable snapshot of the whole Docker stack definition (Nginx
// flag, MariaDB credentials, every Container Aplicativo) plus the
// WorkspacePath it was created against.
//
// WorkspacePath travels alongside cfg.Docker specifically so ImportConfig
// can refuse to import onto a machine whose own WorkspacePath doesn't
// match (see ErrWorkspaceMismatch) — every AppContainer's Folder is only
// meaningful relative to a workspace, and importing onto the wrong one
// would create containers bind-mounting paths that don't hold whatever
// html/data the user actually expects there. This whole feature exists for
// a workflow (confirmed with the user, 2026-08-21) where the workspace
// itself is already kept in sync across machines by an external tool
// (e.g. MegaSync) at the same absolute path everywhere — this file rides
// along the same idea, but travels separately, since its own location
// isn't fixed inside the workspace (the user chooses it each time, same as
// the existing MariaDB backup/restore path field).
type ExportedConfig struct {
	WorkspacePath string              `yaml:"workspace_path"`
	Docker        config.DockerConfig `yaml:"docker"`
}

// ExportConfig writes the current cfg.Docker plus cfg.WorkspacePath to
// path, as YAML.
//
// MariaDB credentials are included in plaintext — by explicit user
// decision (2026-08-21): the exported file is expected to travel over a
// channel the user already controls (their own sync tool), and without the
// credentials each machine would end up with different ones, defeating the
// point of a fast replica.
func ExportConfig(path string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("carregar config: %w", err)
	}
	if !cfg.Docker.NginxCreated && cfg.Docker.MariaDB.DBUser == "" && len(cfg.Docker.Apps) == 0 {
		return fmt.Errorf("nada para exportar — nenhum contêiner Docker registrado ainda")
	}

	exported := ExportedConfig{
		WorkspacePath: cfg.WorkspacePath,
		Docker:        cfg.Docker,
	}
	data, err := yaml.Marshal(exported)
	if err != nil {
		return fmt.Errorf("gerar yaml: %w", err)
	}
	// Contains MariaDB credentials — same permission discipline as
	// config.yaml itself: 0600 even when overwriting an existing file (a
	// plain os.WriteFile would keep a previous 0644), written atomically.
	if err := fsutil.WriteFileAtomic(path, data, 0o600); err != nil {
		return fmt.Errorf("escrever %s: %w", path, err)
	}
	return nil
}

// LoadExportedConfig reads and parses path — split out from ImportConfig so
// the GUI can show the caller a summary (or a clear "file not found"/parse
// error) before asking for confirmation, ahead of anything destructive.
func LoadExportedConfig(path string) (ExportedConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ExportedConfig{}, fmt.Errorf("ler %s: %w", path, err)
	}
	var exported ExportedConfig
	if err := yaml.Unmarshal(data, &exported); err != nil {
		return ExportedConfig{}, fmt.Errorf("parsear %s: %w", path, err)
	}
	return exported, nil
}

// ErrWorkspaceMismatch is returned (wrapped) by ImportConfig/
// resolveImportWorkspace when the importing machine already has a
// WorkspacePath configured and it disagrees with exported.WorkspacePath —
// see the ExportedConfig doc comment for why that's refused rather than
// silently overridden.
var ErrWorkspaceMismatch = errors.New("workspace path não corresponde ao da exportação")

// resolveImportWorkspace decides what cfg.WorkspacePath should become after
// importing: adopting exportedWorkspacePath when the local config has none
// yet (first import on a freshly set up machine), or refusing with
// ErrWorkspaceMismatch when the two disagree. Split out from ImportConfig
// so this decision is unit-testable without a real Docker daemon.
// validateExported checks everything an import would apply: every app
// (ValidateApp, plus unique folders and URLs), the MariaDB user, and an
// absolute workspace path — the file comes from outside (a synced folder),
// so nothing in it is trusted to be well-formed.
func validateExported(e ExportedConfig) error {
	if e.WorkspacePath != "" && !filepath.IsAbs(e.WorkspacePath) {
		return fmt.Errorf("workspace_path deve ser um caminho absoluto: %q", e.WorkspacePath)
	}
	if u := e.Docker.MariaDB.DBUser; u != "" && !ValidDBIdentifier.MatchString(u) {
		return fmt.Errorf("usuário do banco inválido: %q", u)
	}
	folders := map[string]bool{}
	for _, app := range e.Docker.Apps {
		if err := ValidateApp(app); err != nil {
			return fmt.Errorf("aplicativo %q: %w", app.Folder, err)
		}
		if folders[app.Folder] {
			return fmt.Errorf("aplicativo %q aparece mais de uma vez", app.Folder)
		}
		folders[app.Folder] = true
		if err := checkURLUnique(app, e.Docker.Apps); err != nil {
			return err
		}
	}
	return nil
}

func resolveImportWorkspace(localWorkspacePath, exportedWorkspacePath string) (string, error) {
	if localWorkspacePath == "" {
		return exportedWorkspacePath, nil
	}
	if exportedWorkspacePath != "" && exportedWorkspacePath != localWorkspacePath {
		return "", fmt.Errorf("%w: exportado de %q, workspace local é %q", ErrWorkspaceMismatch, exportedWorkspacePath, localWorkspacePath)
	}
	return localWorkspacePath, nil
}

// ImportConfig applies exported onto the local config.yaml (locking
// cfg.WorkspacePath via resolveImportWorkspace) and then recreates every
// container it describes — Nginx, then MariaDB, then every Container
// Aplicativo, in that dependency order — by reusing each type's own
// Recreate* function (RecreateNginx/RecreateMariaDB/RecreateApp), which
// already remove-then-create against whatever's now in cfg.Docker. Every
// PHP base image a Container Aplicativo needs is built on demand inside
// CreateApp, same as a normal creation — nothing pre-builds them here.
//
// Everything in exported is validated before anything is saved or touched
// (validateExported). After that it stops at the first error, leaving
// cfg.Docker already fully saved (so a second Import attempt — or a manual
// recreate — can pick up where it left off) but only the containers
// recreated so far actually running; this does not attempt to roll back a
// partially applied import. Apps registered locally but absent from the
// file leave the stack; their containers are listed as a warning, never
// removed.
func ImportConfig(ctx context.Context, exe *executor.Executor, stdin io.Reader, stdout io.Writer, exported ExportedConfig) error {
	if err := validateExported(exported); err != nil {
		return fmt.Errorf("arquivo de configurações inválido: %w", err)
	}

	stackMu.Lock()
	defer stackMu.Unlock()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("carregar config: %w", err)
	}

	workspacePath, err := resolveImportWorkspace(cfg.WorkspacePath, exported.WorkspacePath)
	if err != nil {
		return err
	}

	for _, local := range cfg.Docker.Apps {
		if _, kept := FindAppByFolder(exported.Docker.Apps, local.Folder); !kept {
			ui.Warning(stdout, "O aplicativo "+local.Folder+" não está no arquivo importado: sai da lista do Perci, mas o contêiner "+
				"dele (se existir) continua — remova-o com `docker rm -f "+local.Folder+"` se não precisar mais.")
		}
	}

	if err := config.Update(func(cfg *config.Config) error {
		cfg.WorkspacePath = workspacePath
		cfg.Docker = exported.Docker
		return nil
	}); err != nil {
		return fmt.Errorf("salvar config: %w", err)
	}

	if exported.Docker.NginxCreated {
		ui.Info(stdout, "Recriando Contêiner Nginx...")
		if _, err := RecreateNginx(ctx, exe, stdin, stdout); err != nil {
			return fmt.Errorf("recriar nginx: %w", err)
		}
	}

	if exported.Docker.MariaDB.DBUser != "" {
		ui.Info(stdout, "Recriando Contêiner MariaDB...")
		if err := RecreateMariaDB(ctx, exe, stdout); err != nil {
			return fmt.Errorf("recriar mariadb: %w", err)
		}
	}

	// recreateAppContainer (not RecreateApp) so N apps trigger only ONE
	// nginx reload below, using the final apps list, instead of one reload
	// per app — each app is already fully reflected in config.yaml at this
	// point (config.Update above), so an intermediate reload after every
	// single app would just rewrite the same eventual default.conf N times.
	var lastApps []config.AppContainer
	for _, app := range exported.Docker.Apps {
		ui.Info(stdout, "Recriando Contêiner Aplicativo "+app.Folder+"...")
		apps, err := recreateAppContainer(ctx, exe, stdout, app.Folder)
		if err != nil {
			return fmt.Errorf("recriar app %s: %w", app.Folder, err)
		}
		lastApps = apps
	}
	// Always reloaded, even with no apps in the file: the local list may
	// just have lost apps whose vhosts must go.
	if lastApps == nil {
		lastApps = exported.Docker.Apps
	}
	if err := ReloadNginxConfig(ctx, exe, stdout, lastApps); err != nil {
		return fmt.Errorf("recarregar nginx: %w", err)
	}

	return nil
}
