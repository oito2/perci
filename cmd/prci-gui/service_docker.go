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

package main

// Bound methods for the "Docker" category — "Criar Container" and
// "Gerenciar Containers", a thin layer over internal/appstack and
// internal/manager/db (MariaDB Backup/Restore).

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/oito2/perci/internal/appstack"
	"github.com/oito2/perci/internal/config"
	"github.com/oito2/perci/internal/fsutil"
	"github.com/oito2/perci/internal/manager/db"
	"github.com/oito2/perci/internal/ui"
)

// DockerService binds the "Docker" category: Criar Container e Gerenciar
// Containers.
type DockerService struct {
	serviceBase
}

// ── Criar Container ──────────────────────────────────────────────────────

// DockerCreateCatalog is what "Docker :: Criar Container" needs to render
// the "Tipo de contêiner" select and the version pickers shared by the app
// types. NginxExists/MariaDBExists disable those two options in the select
// (both are singletons): appstack itself doesn't prevent creating a second
// container with the same name, that's enforced here in the GUI.
type DockerCreateCatalog struct {
	NginxExists    bool     `json:"nginxExists"`
	MariaDBExists  bool     `json:"mariadbExists"`
	MariaDBReady   bool     `json:"mariadbReady"` // controls the "Habilitar acesso ao banco" checkbox
	MkcertReady    bool     `json:"mkcertReady"`  // false = creating the Nginx container now will call "mkcert -install" for the first time
	PHPVersions    []string `json:"phpVersions"`
	NodeVersions   []string `json:"nodeVersions"`
	MoodleVersions []string `json:"moodleVersions"`
}

func (s *DockerService) GetDockerCreateCatalog() DockerCreateCatalog {
	cfg, err := config.Load()
	mariadbReady := err == nil && cfg.Docker.MariaDB.DBUser != ""
	return DockerCreateCatalog{
		NginxExists:    appstack.NginxExists(context.Background(), s.exe),
		MariaDBExists:  appstack.MariaDBExists(context.Background(), s.exe),
		MariaDBReady:   mariadbReady,
		MkcertReady:    appstack.WildcardCertReady(),
		PHPVersions:    appstack.SupportedPHPVersions,
		NodeVersions:   appstack.SupportedNodeVersions,
		MoodleVersions: appstack.MoodleVersions(),
	}
}

// GetPHPVersionsForMoodleVersion returns the PHP versions valid for a given
// Moodle version range (appstack.PHPVersionsForMoodleVersion) — the
// "Versão PHP" select re-fetches this every time "Versão do Moodle"
// changes.
func (s *DockerService) GetPHPVersionsForMoodleVersion(moodleVersion string) []string {
	return appstack.PHPVersionsForMoodleVersion(moodleVersion)
}

// GenMariaDBPassword returns a fresh random password (appstack.GenPassword)
// to pre-fill the "Criar Container MariaDB" form's "Senha" field with.
func (s *DockerService) GenMariaDBPassword() string {
	return appstack.GenPassword()
}

// ContainerFormRequest carries every field any of the 7 container kinds'
// create/edit form might send — unused fields for a given Kind are simply
// left zero/empty, mirroring config.AppContainer's own "one struct, unused
// fields stay empty per type" shape. Kind is "nginx"/"mariadb", or one of
// config.AppType* ("moodle"/"php"/"generic"/"node"/"php_node") for the 5
// Container Aplicativo variants.
type ContainerFormRequest struct {
	Kind string `json:"kind"`

	// MariaDB
	DBUser string `json:"dbUser"`
	DBPass string `json:"dbPass"`

	// Container Aplicativo (shared by all 5 types, fields unused by a
	// given type stay empty/zero)
	Name           string `json:"name"`
	Folder         string `json:"folder"`
	URL            string `json:"url"`
	PHPVersion     string `json:"phpVersion"`
	DBAccess       bool   `json:"dbAccess"`
	MoodleVersion  string `json:"moodleVersion"`
	NodeVersion    string `json:"nodeVersion"`
	DevCommand     string `json:"devCommand"`
	DevPort        int    `json:"devPort"`
	PHPMemoryLimit string `json:"phpMemoryLimit"`
	WorkerCommand  string `json:"workerCommand"`
}

func (r ContainerFormRequest) toAppContainer() config.AppContainer {
	return config.AppContainer{
		Name:           r.Name,
		Folder:         r.Folder,
		Type:           r.Kind,
		URL:            r.URL,
		PHPVersion:     r.PHPVersion,
		DBAccess:       r.DBAccess,
		MoodleVersion:  r.MoodleVersion,
		NodeVersion:    r.NodeVersion,
		DevCommand:     r.DevCommand,
		DevPort:        r.DevPort,
		PHPMemoryLimit: r.PHPMemoryLimit,
		WorkerCommand:  r.WorkerCommand,
	}
}

// kindNginx/kindMariaDB/kindApp are the 3 dispatch-family values used
// throughout this file (ContainerRow.Family, and the "kind"/"family"
// parameter of every Create/Update/Remove/Recreate/Edit method below) —
// named instead of repeating the literals, so a typo in a new `case` is a
// compile error instead of a silently-unmatched string.
const (
	kindNginx   = "nginx"
	kindMariaDB = "mariadb"
	kindApp     = "app"
)

// isAppKind reports whether kind is one of the 5 Container Aplicativo
// variants (config.AppType*), as opposed to "nginx"/"mariadb".
func isAppKind(kind string) bool {
	switch kind {
	case config.AppTypeMoodle, config.AppTypePHP, config.AppTypeGeneric, config.AppTypeNode, config.AppTypePHPNode:
		return true
	}
	return false
}

// CreateDockerContainer dispatches req.Kind to the right appstack.Create*
// — shows the Execução tab (s.runAction) like every other "long-running
// command" screen in the GUI, since building a base image on first use of
// a PHP/Node version can take a while.
func (s *DockerService) CreateDockerContainer(req ContainerFormRequest) error {
	return s.runAction(func(stdout io.Writer) error {
		switch {
		case req.Kind == kindNginx:
			if appstack.NginxExists(context.Background(), s.exe) {
				return fmt.Errorf("o contêiner Nginx já existe")
			}
			_, err := appstack.CreateNginx(context.Background(), s.exe, nil, stdout)
			return err
		case req.Kind == kindMariaDB:
			if appstack.MariaDBExists(context.Background(), s.exe) {
				return fmt.Errorf("o contêiner MariaDB já existe")
			}
			return appstack.CreateMariaDB(context.Background(), s.exe, stdout, req.DBUser, req.DBPass)
		case isAppKind(req.Kind):
			return appstack.CreateApp(context.Background(), s.exe, stdout, req.toAppContainer())
		default:
			return fmt.Errorf("tipo de contêiner desconhecido: %s", req.Kind)
		}
	})
}

// ── Gerenciar Containers — tabela ────────────────────────────────────────

// ContainerRow is one row of "Docker :: Gerenciar Containers".
type ContainerRow struct {
	// Family is the dispatch family ("nginx" | "mariadb" | "app") — which
	// Recriar/Remover/Editar function to call. Named distinctly from
	// ContainerFormRequest.Kind (the specific app subtype, e.g. "moodle"),
	// which is a different field with different values despite the two
	// structs sharing the same "create/edit a container" screens.
	Family string `json:"family"`
	Type   string `json:"type"`   // "nginx" | "mariadb" | config.AppType* — for the table's label/icon
	Name   string `json:"name"`   // display label ("Nginx", "MariaDB", or AppContainer.Name)
	Folder string `json:"folder"` // the real Docker container name
	Status string `json:"status"` // Docker's raw value: running/exited/created/paused/restarting/dead/"" (not found)
	// DockerError is set (and Status left "") when the Docker daemon could
	// not be queried, so the status is unknown rather than "not found".
	DockerError string `json:"dockerError,omitempty"`
	CanEdit     bool   `json:"canEdit"`
	HasDB       bool   `json:"hasDb"` // only true for "mariadb" — enables the extra Backup/Restore icons
}

// GetContainerRows lists every container registered in config.yaml (the
// source of truth — never derived from `docker ps`), enriched with its
// live status via appstack.ContainerStatuses — one batched call for every
// container at once, not a `docker inspect` subprocess per container. When
// Docker can't be queried, every row carries the reason in DockerError.
func (s *DockerService) GetContainerRows() []ContainerRow {
	cfg, err := config.Load()
	if err != nil {
		return nil
	}
	ctx := context.Background()

	var rows []ContainerRow
	if cfg.Docker.NginxCreated {
		rows = append(rows, ContainerRow{
			Family: kindNginx, Type: "nginx", Name: "Nginx", Folder: appstack.NginxContainerName,
		})
	}
	if cfg.Docker.MariaDB.DBUser != "" {
		rows = append(rows, ContainerRow{
			Family: kindMariaDB, Type: "mariadb", Name: "MariaDB", Folder: appstack.MariaDBContainerName,
			CanEdit: true, HasDB: true,
		})
	}
	for _, app := range cfg.Docker.Apps {
		rows = append(rows, ContainerRow{
			Family: kindApp, Type: app.Type, Name: app.Name, Folder: app.Folder, CanEdit: true,
		})
	}

	names := make([]string, len(rows))
	for i, row := range rows {
		names[i] = row.Folder
	}
	statuses, err := appstack.ContainerStatuses(ctx, s.exe, names)
	for i, row := range rows {
		if err != nil {
			rows[i].DockerError = err.Error()
			continue
		}
		rows[i].Status = statuses[row.Folder]
	}
	return rows
}

// GetContainerForEdit returns the current field values for the "Editar"
// modal, pre-filling it. kind must be "mariadb" or "app" (Nginx has no
// Editar).
func (s *DockerService) GetContainerForEdit(kind, folder string) (ContainerFormRequest, error) {
	cfg, err := config.Load()
	if err != nil {
		return ContainerFormRequest{}, err
	}
	switch kind {
	case kindMariaDB:
		return ContainerFormRequest{
			Kind: kindMariaDB, DBUser: cfg.Docker.MariaDB.DBUser, DBPass: cfg.Docker.MariaDB.DBPass,
		}, nil
	case kindApp:
		app, found := appstack.FindAppByFolder(cfg.Docker.Apps, folder)
		if !found {
			return ContainerFormRequest{}, fmt.Errorf("contêiner aplicativo %q não encontrado", folder)
		}
		return ContainerFormRequest{
			Kind: app.Type, Name: app.Name, Folder: app.Folder, URL: app.URL,
			PHPVersion: app.PHPVersion, DBAccess: app.DBAccess, MoodleVersion: app.MoodleVersion,
			NodeVersion: app.NodeVersion, DevCommand: app.DevCommand, DevPort: app.DevPort,
			PHPMemoryLimit: app.PHPMemoryLimit, WorkerCommand: app.WorkerCommand,
		}, nil
	}
	return ContainerFormRequest{}, fmt.Errorf("tipo sem edição: %s", kind)
}

// UpdateDockerContainer applies an edit by removing and recreating the
// container with the new parameters. Treated as a long-running operation
// (it may rebuild the base image) — same handling as Recriar, shows the
// Execução tab.
func (s *DockerService) UpdateDockerContainer(req ContainerFormRequest) error {
	return s.runAction(func(stdout io.Writer) error {
		ctx := context.Background()
		switch {
		case req.Kind == kindMariaDB:
			// Checked before removing anything: a refusal must leave the
			// running database untouched.
			if err := appstack.CheckMariaDBCredentials(stdout, req.DBUser, req.DBPass); err != nil {
				return err
			}
			if appstack.MariaDBExists(ctx, s.exe) {
				if err := appstack.RemoveMariaDB(ctx, s.exe, stdout); err != nil {
					return err
				}
			}
			return appstack.CreateMariaDB(ctx, s.exe, stdout, req.DBUser, req.DBPass)
		case isAppKind(req.Kind):
			// Validated and built before the running container is removed:
			// invalid input or a failed build must not leave the app down.
			return appstack.UpdateApp(ctx, s.exe, stdout, req.toAppContainer())
		default:
			return fmt.Errorf("tipo sem edição: %s", req.Kind)
		}
	})
}

// ── Ações rápidas (sem terminal — spinner na própria linha) ──────────────
//
// Iniciar/Parar/Reiniciar/Remover don't open the Execução tab — they run
// synchronously from the JS side (await), which shows a loading spinner in
// the Status column while it waits and, on error, a modal — none of these
// go through the log-line/step/action-done event mechanism the rest of the
// GUI uses.

func (s *DockerService) StartDockerContainer(folder string) error {
	return runExclusive(func() error {
		return appstack.StartContainer(context.Background(), s.exe, io.Discard, folder)
	})
}

func (s *DockerService) StopDockerContainer(folder string) error {
	return runExclusive(func() error {
		return appstack.StopContainer(context.Background(), s.exe, io.Discard, folder)
	})
}

func (s *DockerService) RestartDockerContainer(folder string) error {
	return runExclusive(func() error {
		return appstack.RestartContainer(context.Background(), s.exe, io.Discard, folder)
	})
}

// RemoveDockerContainer deletes the container (if running/stopped) and its
// entry in config.yaml — appstack.Delete{Nginx,MariaDB,App}, called only
// after the frontend's confirmation modal.
func (s *DockerService) RemoveDockerContainer(kind, folder string) error {
	return runExclusive(func() error {
		ctx := context.Background()
		switch kind {
		case kindNginx:
			return appstack.DeleteNginx(ctx, s.exe, io.Discard)
		case kindMariaDB:
			return appstack.DeleteMariaDB(ctx, s.exe, io.Discard)
		case kindApp:
			return appstack.DeleteApp(ctx, s.exe, io.Discard, folder)
		}
		return fmt.Errorf("tipo desconhecido: %s", kind)
	})
}

// RecreateDockerContainer dispatches to RecreateNginx/RecreateMariaDB/
// RecreateApp — shows the Execução tab, called only after the frontend's
// confirmation modal.
func (s *DockerService) RecreateDockerContainer(kind, folder string) error {
	return s.runAction(func(stdout io.Writer) error {
		ctx := context.Background()
		switch kind {
		case kindNginx:
			_, err := appstack.RecreateNginx(ctx, s.exe, nil, stdout)
			return err
		case kindMariaDB:
			return appstack.RecreateMariaDB(ctx, s.exe, stdout)
		case kindApp:
			return appstack.RecreateApp(ctx, s.exe, stdout, folder)
		}
		return fmt.Errorf("tipo desconhecido: %s", kind)
	})
}

// ── Atualizar imagem ──────────────────────────────────────────────────────

// UpdateDockerContainerImage updates the image of one container: Nginx,
// MariaDB, or an app together with every app sharing its Perci image (see
// updateImages).
func (s *DockerService) UpdateDockerContainerImage(kind, folder string) error {
	return s.runAction(func(stdout io.Writer) error {
		var target appstack.ImageUpdateTarget
		switch kind {
		case kindNginx:
			target.Nginx = true
		case kindMariaDB:
			target.MariaDB = true
		case kindApp:
			t, err := appstack.AppImageTarget(folder)
			if err != nil {
				return err
			}
			target = t
		default:
			return fmt.Errorf("tipo desconhecido: %s", kind)
		}
		return s.updateImages(stdout, target)
	})
}

// UpdateAllDockerImages updates the images of every registered container
// (see updateImages).
func (s *DockerService) UpdateAllDockerImages() error {
	return s.runAction(func(stdout io.Writer) error {
		target, err := appstack.AllImagesTarget()
		if err != nil {
			return err
		}
		return s.updateImages(stdout, target)
	})
}

// updateImages pulls and rebuilds the images of target, then recreates the
// containers whose image changed. Before MariaDB is recreated, all its
// databases are dumped to appstack.MariaDBBackupDir; MariaDB must be
// running for that, otherwise nothing is recreated.
func (s *DockerService) updateImages(stdout io.Writer, target appstack.ImageUpdateTarget) error {
	ctx := context.Background()
	plan, err := appstack.PrepareImageUpdate(ctx, s.exe, stdout, target)
	if err != nil {
		return err
	}
	if plan.Empty() {
		ui.Success(stdout, "Tudo já está na versão mais recente — nenhum contêiner foi recriado.")
		return nil
	}
	if plan.MariaDB {
		if appstack.ContainerStatus(ctx, s.exe, appstack.MariaDBContainerName) != "running" {
			return fmt.Errorf("o MariaDB precisa estar rodando para o backup automático antes da atualização — inicie-o e tente de novo")
		}
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		dir, err := appstack.MariaDBBackupDir()
		if err != nil {
			return err
		}
		dest := filepath.Join(dir, "mariadb-antes-da-atualizacao-"+time.Now().Format("2006-01-02-150405")+".sql")
		if err := db.Backup(ctx, s.exe, stdout, appstack.MariaDBContainerName, cfg.Docker.MariaDB.DBUser, cfg.Docker.MariaDB.DBPass, dest); err != nil {
			return fmt.Errorf("backup antes da atualização: %w", err)
		}
	}
	if err := appstack.ApplyImageUpdate(ctx, s.exe, stdout, plan); err != nil {
		return err
	}
	ui.Success(stdout, "Imagens atualizadas e contêineres recriados.")
	return nil
}

// ── Ver Logs ──────────────────────────────────────────────────────────────

// GetContainerLogsSnapshot fetches the last 200 lines once (docker logs
// --tail 200, no -f) — an instant modal, not a live stream. Only the logs of
// Nginx, MariaDB or a registered Container Aplicativo are returned. The
// modal's "Atualizar" button just calls this method again.
func (s *DockerService) GetContainerLogsSnapshot(folder string) (string, error) {
	return appstack.ContainerLogs(context.Background(), s.exe, folder, 200)
}

// PickLogsExportPath opens the native "Salvar Como" dialog for exporting a
// container's logs to a .txt file.
func (s *DockerService) PickLogsExportPath(defaultName string) (string, error) {
	return approvePath(s.wailsApp.Dialog.SaveFileWithOptions(&application.SaveFileDialogOptions{
		Title:    "Exportar logs",
		Filename: defaultName,
		Filters:  []application.FileFilter{{DisplayName: "Texto (*.txt)", Pattern: "*.txt"}},
	}).PromptForSingleSelection())
}

// SaveTextFile writes content to path — reused by the logs export button.
// Only to a path chosen in PickLogsExportPath (requireApprovedPath). 0o600,
// even over an existing file: content is a container log, which can carry
// secrets (env vars, connection strings) — same permission policy
// appstack.ExportConfig uses for its own backup export.
func (s *DockerService) SaveTextFile(path, content string) error {
	if err := requireApprovedPath(path); err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(path, []byte(content), 0o600)
}

// ── Exportar/Importar ─────────────────────────────────────────────────────

// TransferSelection is the frontend's choice of items on Docker →
// Exportar/Importar: the Nginx and MariaDB containers, apps by folder and
// plugin mounts by key ("<app>/<rel_path>").
type TransferSelection struct {
	Nginx   bool     `json:"nginx"`
	MariaDB bool     `json:"mariadb"`
	Apps    []string `json:"apps"`
	Plugins []string `json:"plugins"`
}

// domain converts s to the appstack selection.
func (s TransferSelection) domain() appstack.TransferSelection {
	return appstack.TransferSelection{Nginx: s.Nginx, MariaDB: s.MariaDB, Apps: s.Apps, Plugins: s.Plugins}
}

// TransferApp is one Container Aplicativo listed on Exportar/Importar.
// Exists (import only) means an app with the same folder is already
// registered here and would be replaced.
type TransferApp struct {
	Folder   string `json:"folder"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	URL      string `json:"url"`
	DBAccess bool   `json:"dbAccess"`
	Exists   bool   `json:"exists"`
}

// TransferPlugin is one plugin mount listed on Exportar/Importar. On
// import, Mounted means the same mount already exists here and Reason
// says why it can't be mounted on this machine (empty when it can).
type TransferPlugin struct {
	Key       string `json:"key"`
	Component string `json:"component"`
	App       string `json:"app"`
	DevPath   string `json:"devPath"`
	Mounted   bool   `json:"mounted"`
	Reason    string `json:"reason"`
}

// TransferItems lists the items of the local config (Exportar) or of an
// export file (Importar). Nginx and MariaDB tell whether each is present;
// LocalNginx, LocalMariaDB and Commands are filled for an import only:
// what already exists here, and one "<pasta>: <comando>" entry per start
// command and "<pasta> (worker): <comando>" per worker command the
// file's apps run.
type TransferItems struct {
	WorkspacePath string           `json:"workspacePath"`
	Nginx         bool             `json:"nginx"`
	MariaDB       bool             `json:"mariadb"`
	Apps          []TransferApp    `json:"apps"`
	Plugins       []TransferPlugin `json:"plugins"`
	LocalNginx    bool             `json:"localNginx"`
	LocalMariaDB  bool             `json:"localMariaDb"`
	Commands      []string         `json:"commands"`
}

// transferItems lists the items of e; local, when not nil, marks the apps
// that already exist in it.
func transferItems(e appstack.ExportedConfig, local *config.Config) TransferItems {
	items := TransferItems{
		WorkspacePath: e.WorkspacePath,
		Nginx:         e.Docker.NginxCreated,
		MariaDB:       e.Docker.MariaDB.DBUser != "",
		Apps:          []TransferApp{},
		Plugins:       []TransferPlugin{},
		Commands:      importCommands(e),
	}
	for _, app := range e.Docker.Apps {
		row := TransferApp{Folder: app.Folder, Name: app.Name, Type: app.Type, URL: app.URL, DBAccess: app.DBAccess}
		if local != nil {
			_, row.Exists = appstack.FindAppByFolder(local.Docker.Apps, app.Folder)
		}
		items.Apps = append(items.Apps, row)
	}
	for _, m := range e.MoodlePlugins {
		items.Plugins = append(items.Plugins, TransferPlugin{Key: appstack.PluginKey(m), Component: m.Component, App: m.App, DevPath: m.DevPath})
	}
	return items
}

// importCommands lists the commands the apps in exported would run inside
// their containers, labelled by app folder.
func importCommands(exported appstack.ExportedConfig) []string {
	commands := []string{}
	for _, app := range exported.Docker.Apps {
		if app.DevCommand != "" {
			commands = append(commands, app.Folder+": "+app.DevCommand)
		}
		if app.WorkerCommand != "" {
			commands = append(commands, app.Folder+" (worker): "+app.WorkerCommand)
		}
	}
	return commands
}

// GetExportItems lists everything the local config can export (the
// Exportar tab's checklist).
func (s *DockerService) GetExportItems() (TransferItems, error) {
	cfg, err := config.Load()
	if err != nil {
		return TransferItems{}, err
	}
	items := transferItems(appstack.Exportable(cfg), nil)
	items.Commands = []string{}
	return items, nil
}

// PickExportConfigPath opens the native "Salvar Como" dialog for
// "Exportar" — folder AND filename in a single window.
func (s *DockerService) PickExportConfigPath() (string, error) {
	return approvePath(s.wailsApp.Dialog.SaveFileWithOptions(&application.SaveFileDialogOptions{
		Title:    "Exportar configurações",
		Filename: "perci-export.yaml",
		Filters:  []application.FileFilter{{DisplayName: "YAML (*.yaml)", Pattern: "*.yaml;*.yml"}},
	}).PromptForSingleSelection())
}

// ExportDockerConfig writes the selected items to path (only a path chosen
// in PickExportConfigPath) — a quick operation (just writes a YAML file),
// run synchronously without the Execução tab.
func (s *DockerService) ExportDockerConfig(path string, sel TransferSelection) error {
	if err := requireApprovedPath(path); err != nil {
		return err
	}
	return appstack.ExportSelected(path, sel.domain())
}

// PickImportConfigPath opens the native "Selecionar arquivo" dialog for
// "Importar".
func (s *DockerService) PickImportConfigPath() (string, error) {
	return approvePath(s.wailsApp.Dialog.OpenFile().
		SetTitle("Selecionar arquivo de configurações").
		AddFilter("YAML (*.yaml)", "*.yaml;*.yml").
		PromptForSingleSelection())
}

// PreviewImportConfig reads, validates and lists the items of the export
// file at path (only a path chosen in PickImportConfigPath) against this
// machine, without applying anything: the Importar tab's checklist.
func (s *DockerService) PreviewImportConfig(path string) (TransferItems, error) {
	if err := requireApprovedPath(path); err != nil {
		return TransferItems{}, err
	}
	exported, err := appstack.LoadExportedConfig(path)
	if err != nil {
		return TransferItems{}, err
	}
	if err := exported.Validate(); err != nil {
		return TransferItems{}, fmt.Errorf("arquivo de configurações inválido: %w", err)
	}
	cfg, err := config.Load()
	if err != nil {
		return TransferItems{}, err
	}
	states, err := appstack.PluginImportStates(exported)
	if err != nil {
		return TransferItems{}, err
	}
	items := transferItems(exported, cfg)
	items.LocalNginx = cfg.Docker.NginxCreated
	items.LocalMariaDB = cfg.Docker.MariaDB.DBUser != ""
	for i, st := range states {
		items.Plugins[i].Mounted = st.Mounted
		items.Plugins[i].Reason = st.Reason
	}
	return items, nil
}

// ImportDockerContainerConfig imports the selected items of the export
// file at path (appstack.ImportConfig): merges them into the local config,
// mounts the selected plugins (one password) and recreates the selected
// containers. Shows the Execução tab — it's effectively a sequence of
// Recriar calls.
func (s *DockerService) ImportDockerContainerConfig(path string, sel TransferSelection) error {
	return s.runAction(func(stdout io.Writer) error {
		if err := requireApprovedPath(path); err != nil {
			return err
		}
		exported, err := appstack.LoadExportedConfig(path)
		if err != nil {
			return err
		}
		return appstack.ImportConfig(context.Background(), s.exe, nil, stdout, exported, sel.domain())
	})
}

// ── Backup/Restore (só MariaDB) ───────────────────────────────────────────

// PickBackupPath opens the native "Salvar Como" dialog for a MariaDB dump,
// default filename with a timestamp so successive backups don't collide.
func (s *DockerService) PickBackupPath() (string, error) {
	defaultName := "mariadb-backup-" + time.Now().Format("2006-01-02-150405") + ".sql"
	return approvePath(s.wailsApp.Dialog.SaveFileWithOptions(&application.SaveFileDialogOptions{
		Title:    "Exportar backup do MariaDB",
		Filename: defaultName,
		Filters:  []application.FileFilter{{DisplayName: "SQL (*.sql)", Pattern: "*.sql"}},
	}).PromptForSingleSelection())
}

// BackupMariaDBContainer wraps internal/manager/db.Backup — shows the
// Execução tab (mysqldump can take a while on large databases, same
// treatment as Recriar).
func (s *DockerService) BackupMariaDBContainer(path string) error {
	return s.runAction(func(stdout io.Writer) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		if err := requireApprovedPath(path); err != nil {
			return err
		}
		return db.Backup(context.Background(), s.exe, stdout, appstack.MariaDBContainerName, cfg.Docker.MariaDB.DBUser, cfg.Docker.MariaDB.DBPass, path)
	})
}

// PickRestorePath opens the native "Selecionar arquivo" dialog for
// restoring a MariaDB dump.
func (s *DockerService) PickRestorePath() (string, error) {
	return approvePath(s.wailsApp.Dialog.OpenFile().
		SetTitle("Selecionar arquivo de backup do MariaDB").
		AddFilter("SQL (*.sql)", "*.sql").
		PromptForSingleSelection())
}

// RestoreMariaDBContainer wraps internal/manager/db.Restore — shows the
// Execução tab, same reason as BackupMariaDBContainer.
func (s *DockerService) RestoreMariaDBContainer(path string) error {
	return s.runAction(func(stdout io.Writer) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		if err := requireApprovedPath(path); err != nil {
			return err
		}
		return db.Restore(context.Background(), s.exe, stdout, appstack.MariaDBContainerName, cfg.Docker.MariaDB.DBUser, cfg.Docker.MariaDB.DBPass, path)
	})
}
