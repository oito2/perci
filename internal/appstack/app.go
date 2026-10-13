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
	"path/filepath"
	"strings"
	"sync"

	"github.com/oito2/perci/internal/config"
	"github.com/oito2/perci/internal/executor"
	"github.com/oito2/perci/internal/ui"
)

// AppHTMLDir returns {workspace}/localhost/html/<folder>: the app's own
// project root, mounted at /var/www/html inside its own container (PHP-
// family types) or /app (AppTypeNode), and (as part of the wider
// localhost/html tree) read-only inside Nginx.
func AppHTMLDir(workspace, folder string) string {
	return filepath.Join(workspace, "localhost", "html", folder)
}

// AppDataDir returns {workspace}/localhost/data/<folder>, only
// created/mounted for AppTypeMoodle (moodledata). It is outside
// localhost/html, so it is never inside the tree Nginx mounts and can't be
// served over the web.
func AppDataDir(workspace, folder string) string {
	return filepath.Join(workspace, "localhost", "data", folder)
}

// AppLogDir returns {workspace}/localhost/logs/<folder> — one log
// directory per project.
func AppLogDir(workspace, folder string) string {
	return filepath.Join(workspace, "localhost", "logs", folder)
}

// AppComboAPIDir returns {workspace}/localhost/html/<folder>/api: the PHP
// half of an AppTypePHPNode combo container, mounted at /var/www/html.
func AppComboAPIDir(workspace, folder string) string {
	return filepath.Join(AppHTMLDir(workspace, folder), "api")
}

// AppComboAppDir returns {workspace}/localhost/html/<folder>/app — the
// Node half of an AppTypePHPNode combo container, mounted at /app.
func AppComboAppDir(workspace, folder string) string {
	return filepath.Join(AppHTMLDir(workspace, folder), "app")
}

// AppExists reports whether a container named folder exists, in any state
// (running or stopped).
func AppExists(ctx context.Context, exe *executor.Executor, folder string) bool {
	return ContainerStatus(ctx, exe, folder) != ""
}

// RemoveApp force-removes the app's container, running or stopped. It does
// not touch html/data on disk and does not update cfg.Docker.Apps or
// Nginx's routing; the caller does both (see recreateAppContainer/
// DeleteApp), after confirming with the user.
func RemoveApp(ctx context.Context, exe *executor.Executor, stdout io.Writer, folder string) error {
	return removeContainer(ctx, exe, stdout, folder, "container "+folder)
}

// dbAccessEnv returns the DB_* env assignments injected into an app
// container when AppContainer.DBAccess is true: the shared MariaDB
// credentials. DBAccess grants no dedicated database or user of its own,
// just these credentials for whatever app-side code reads DB settings from
// the environment; no config.php or other app config is generated from
// them.
//
// Every Container Aplicativo shares docker-php-network with MariaDB
// regardless of DBAccess, so DBAccess only controls whether these
// variables are handed to the app. DB_NAME is always the shared "dev_db"
// every MariaDB container creates.
func dbAccessEnv(mariadb config.MariaDBConfig) []string {
	return []string{
		"DB_HOST=" + MariaDBContainerName,
		"DB_PORT=3306",
		"DB_USER=" + mariadb.DBUser,
		"DB_PASS=" + mariadb.DBPass,
		"DB_NAME=dev_db",
	}
}

// envNames turns KEY=VALUE assignments into `-e KEY` docker run arguments.
// Given a bare name, docker copies the value from its own client
// environment, so callers pass the assignments themselves through
// executor.Options.Env instead of the argv: a process's command line is
// readable by every local user, its environment only by the same user.
// Used for anything carrying a credential.
func envNames(env []string) []string {
	args := make([]string, 0, 2*len(env))
	for _, e := range env {
		name, _, _ := strings.Cut(e, "=")
		args = append(args, "-e", name)
	}
	return args
}

// phpConfMount is the fixed in-container path every PHP-family container's
// per-app memory_limit override (WritePHPMemoryLimitConf) is bind-mounted
// at.
const phpConfMount = "/usr/local/etc/php/conf.d/" + phpMemoryLimitOverrideFile

// hostGatewayAlias makes host.docker.internal resolve to the host inside a
// container: Docker Engine on Linux only defines it when asked.
const hostGatewayAlias = "host.docker.internal:host-gateway"

// bindMount returns the `--mount` argument pair for a bind mount of source
// at target, read-only when readonly. Unlike `-v`, the source and target
// are separate key=value fields, so a ':' in either cannot be misparsed.
// A ',' or '"' in either would still split the value; mountableError
// refuses such paths before they get here.
func bindMount(source, target string, readonly bool) []string {
	spec := "type=bind,source=" + source + ",target=" + target
	if readonly {
		spec += ",readonly"
	}
	return []string{"--mount", spec}
}

// mountableError returns an error naming the first of paths that cannot be
// written into a `--mount` value (it holds a ',', a '"' or a line break).
func mountableError(paths ...string) error {
	for _, p := range paths {
		if strings.ContainsAny(p, ",\"\r\n") {
			return fmt.Errorf("o caminho %q não pode ser montado no contêiner: remova vírgulas, aspas e quebras de linha do nome da pasta", p)
		}
	}
	return nil
}

// appRunArgs builds the "docker run" argument list for a Container
// Aplicativo. dataDir is only included when non-empty (AppTypeMoodle).
// Unit-testable without a Docker daemon.
func appRunArgs(folder, image, htmlDir, dataDir, logDir, phpConfPath string, dbEnv []string) []string {
	args := []string{
		"run", "-d",
		"--name", folder,
		"--network", NetworkName,
		"--restart", "unless-stopped",
		"--add-host", hostGatewayAlias,
	}
	args = append(args, bindMount(htmlDir, "/var/www/html", false)...)
	if dataDir != "" {
		args = append(args, bindMount(dataDir, "/var/www/data", false)...)
	}
	args = append(args, bindMount(logDir, "/var/log/php", false)...)
	args = append(args, bindMount(phpConfPath, phpConfMount, true)...)
	args = append(args, envNames(dbEnv)...) // values go through the client env, see envNames
	args = append(args, "--", image)
	return args
}

// nodeAppRunArgs builds the "docker run" argument list for an AppTypeNode
// container. Unit-testable without a Docker daemon.
//
// Runs as the image's "node" user (UID-matched to the host at build time,
// see EnsureNodeImage) so files the dev server writes into the
// bind-mounted project folder aren't root-owned on the host. devCommand
// becomes the container's CMD via `sh -c`; it is free text, passed as a
// single argv element to sh -c inside the container, and exe.Run never
// invokes a host shell to parse it.
func nodeAppRunArgs(folder, image, appDir, devCommand string) []string {
	args := []string{
		"run", "-d",
		"--name", folder,
		"--network", NetworkName,
		"--restart", "unless-stopped",
		"--user", "node",
	}
	args = append(args, bindMount(appDir, "/app", false)...)
	return append(args, "--", image, "sh", "-c", devCommand)
}

// comboAppRunArgs builds the "docker run" argument list for an
// AppTypePHPNode container: a single multi-process container (supervisord
// manages php-fpm, the dev server and the optional worker, see
// comboSupervisordConf) with both halves of the project mounted. Unlike
// nodeAppRunArgs, this container runs as root (no --user flag), because
// supervisord needs root to drop privileges per-program. devCommand is
// passed as the DEV_COMMAND environment variable, which supervisord's
// %(ENV_DEV_COMMAND)s substitution reads inside the container.
// workerCommand is passed as WORKER_COMMAND, always set (even to ""), and
// is read at run time by [program:worker]'s shell wrapper. Neither is
// interpolated into any file or host-side shell string.
func comboAppRunArgs(folder, image, apiDir, appDir, logDir, phpConfPath, devCommand, workerCommand string, dbEnv []string) []string {
	args := []string{
		"run", "-d",
		"--name", folder,
		"--network", NetworkName,
		"--restart", "unless-stopped",
		"--add-host", hostGatewayAlias,
	}
	args = append(args, bindMount(apiDir, "/var/www/html", false)...)
	args = append(args, bindMount(appDir, "/app", false)...)
	args = append(args, bindMount(logDir, "/var/log/php", false)...)
	args = append(args, bindMount(phpConfPath, phpConfMount, true)...)
	args = append(args, "-e", "DEV_COMMAND="+devCommand, "-e", "WORKER_COMMAND="+workerCommand)
	args = append(args, envNames(dbEnv)...) // values go through the client env, see envNames
	args = append(args, "--", image)
	return args
}

// replaceOrAppendApp returns apps with app inserted: replacing any existing
// entry with the same Folder (a recreate), or appended as a new one.
// Returns a fresh slice and never mutates apps in place.
func replaceOrAppendApp(apps []config.AppContainer, app config.AppContainer) []config.AppContainer {
	for i, a := range apps {
		if a.Folder == app.Folder {
			out := append([]config.AppContainer(nil), apps...)
			out[i] = app
			return out
		}
	}
	return append(append([]config.AppContainer(nil), apps...), app)
}

// removeAppByFolder returns apps with the entry matching folder removed, if
// any. Returns a fresh slice and never mutates apps in place.
func removeAppByFolder(apps []config.AppContainer, folder string) []config.AppContainer {
	out := make([]config.AppContainer, 0, len(apps))
	for _, a := range apps {
		if a.Folder != folder {
			out = append(out, a)
		}
	}
	return out
}

// FindAppByFolder looks up the cfg.Docker.Apps entry for folder.
func FindAppByFolder(apps []config.AppContainer, folder string) (config.AppContainer, bool) {
	for _, a := range apps {
		if a.Folder == folder {
			return a, true
		}
	}
	return config.AppContainer{}, false
}

// stackMu serializes the operations that change the stack and regenerate
// Nginx's routing from it (create/recreate/delete an app, import), so
// interleaved operations cannot write default.conf from a stale app list.
var stackMu sync.Mutex

// reservedFolders are the infrastructure containers' own names: an app
// folder is also its container name, so an app called "nginx" would collide
// with the stack's Nginx.
var reservedFolders = map[string]bool{NginxContainerName: true, MariaDBContainerName: true}

// ValidateApp checks every field of app on its own, without Docker or
// config, so callers can refuse bad input before removing anything.
func ValidateApp(app config.AppContainer) error {
	if !ValidAppFolder.MatchString(app.Folder) {
		return fmt.Errorf("nome de pasta inválido: use apenas letras, números, hífen e underscore (1-64 caracteres)")
	}
	if !ValidAppURL.MatchString(app.URL) {
		return fmt.Errorf("URL inválida: use um único rótulo seguido de .localhost (ex: meuapp.localhost)")
	}
	if !ValidAppType(app.Type) {
		return fmt.Errorf("tipo de aplicativo desconhecido: %s", app.Type)
	}
	needsNodeFields := app.Type == config.AppTypeNode || app.Type == config.AppTypePHPNode
	if needsNodeFields {
		if !ValidNodeVersion(app.NodeVersion) {
			return fmt.Errorf("versão Node não suportada: %s", app.NodeVersion)
		}
		if app.DevCommand == "" {
			return fmt.Errorf("comando de start não pode ficar vazio")
		}
		if !ValidDevPort(app.DevPort) {
			return fmt.Errorf("porta do dev server inválida: %d", app.DevPort)
		}
	}
	if app.Type == config.AppTypeNode && app.DBAccess {
		return fmt.Errorf("acesso a banco não é suportado para aplicativos Node")
	}
	if app.Type != config.AppTypeNode && !ValidPHPVersion(app.PHPVersion) {
		return fmt.Errorf("versão PHP não suportada: %s", app.PHPVersion)
	}
	if app.Type != config.AppTypeNode && app.PHPMemoryLimit != "" && !ValidPHPMemoryLimit.MatchString(app.PHPMemoryLimit) {
		return fmt.Errorf("limite de memória PHP inválido: %s", app.PHPMemoryLimit)
	}
	if app.Type == config.AppTypeMoodle {
		if !ValidMoodleVersion(app.MoodleVersion) {
			return fmt.Errorf("versão do Moodle não suportada: %s", app.MoodleVersion)
		}
		if !ValidPHPVersionForMoodleVersion(app.MoodleVersion, app.PHPVersion) {
			return fmt.Errorf("PHP %s não é compatível com Moodle %s", app.PHPVersion, app.MoodleVersion)
		}
	}

	if reservedFolders[strings.ToLower(app.Folder)] {
		return fmt.Errorf("o nome de pasta %q é reservado para a infraestrutura do Perci — escolha outro", app.Folder)
	}
	return nil
}

// ValidateAppInStack is ValidateApp plus the checks against the rest of the
// stack (a URL already used by another app), for callers about to remove a
// running container before recreating it.
func ValidateAppInStack(app config.AppContainer) error {
	if err := ValidateApp(app); err != nil {
		return err
	}
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("carregar config: %w", err)
	}
	return checkURLUnique(app, cfg.Docker.Apps)
}

// checkURLUnique refuses app.URL when another app (different folder)
// already routes it, since two server blocks with one server_name leave the
// second app unreachable.
func checkURLUnique(app config.AppContainer, apps []config.AppContainer) error {
	for _, other := range apps {
		if other.Folder != app.Folder && strings.EqualFold(other.URL, app.URL) {
			return fmt.Errorf("a URL %s já é usada pelo aplicativo %q", app.URL, other.Folder)
		}
	}
	return nil
}

// appPrep is what prepareApp resolved for an app: the workspace folder and
// the DB_* assignments its container gets (nil without DBAccess).
type appPrep struct {
	workspace string
	dbEnv     []string
}

// prepareApp does everything an app needs before its container can be
// created and that may take minutes or fail: it validates app, ensures
// NetworkName, creates the project/data/log folders and builds the base
// image. It never touches a running container, so callers run it before
// removing the old one and without holding stackMu.
func prepareApp(ctx context.Context, exe *executor.Executor, stdout io.Writer, app config.AppContainer) (appPrep, error) {
	if err := ValidateApp(app); err != nil {
		return appPrep{}, err
	}
	if !isDockerInstalled(ctx, exe) {
		return appPrep{}, fmt.Errorf("docker não instalado")
	}

	cfg, err := config.Load()
	if err != nil {
		return appPrep{}, fmt.Errorf("carregar config: %w", err)
	}
	if err := checkURLUnique(app, cfg.Docker.Apps); err != nil {
		return appPrep{}, err
	}

	workspace, err := resolveWorkspace(cfg)
	if err != nil {
		return appPrep{}, err
	}

	prep := appPrep{workspace: workspace}
	if app.DBAccess {
		if cfg.Docker.MariaDB.DBUser == "" {
			return appPrep{}, fmt.Errorf(`nenhum Contêiner MariaDB configurado ainda — crie um em "Gerenciar Docker :: Criar Contêiner MariaDB" antes de habilitar acesso a banco`)
		}
		prep.dbEnv = dbAccessEnv(cfg.Docker.MariaDB)
	}

	if err := EnsureNetwork(ctx, exe, stdout); err != nil {
		return appPrep{}, err
	}

	dirs := []string{AppHTMLDir(workspace, app.Folder)}
	switch app.Type {
	case config.AppTypeNode:
	case config.AppTypePHPNode:
		dirs = append(dirs, AppComboAPIDir(workspace, app.Folder), AppComboAppDir(workspace, app.Folder), AppLogDir(workspace, app.Folder))
	default:
		dirs = append(dirs, AppLogDir(workspace, app.Folder))
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return appPrep{}, fmt.Errorf("criar %s: %w", dir, err)
		}
	}
	if app.Type == config.AppTypeMoodle {
		if err := ensureMoodleDataDir(stdout, AppDataDir(workspace, app.Folder)); err != nil {
			return appPrep{}, err
		}
	}

	switch app.Type {
	case config.AppTypeNode:
		err = EnsureNodeImage(ctx, exe, stdout, app.NodeVersion)
	case config.AppTypePHPNode:
		err = EnsureComboImage(ctx, exe, stdout, app.PHPVersion, app.NodeVersion)
	default:
		err = EnsureImage(ctx, exe, stdout, app.PHPVersion)
	}
	if err != nil {
		return appPrep{}, err
	}
	return prep, nil
}

// moodleDataMode is the permission of an app's moodledata folder: it holds
// sessions and uploads, so other local users get no access.
const moodleDataMode = 0o750

// ensureMoodleDataDir creates dir with moodleDataMode and tightens the
// permissions of one that already exists; a folder it cannot chmod only
// produces a warning.
func ensureMoodleDataDir(stdout io.Writer, dir string) error {
	if err := os.MkdirAll(dir, moodleDataMode); err != nil {
		return fmt.Errorf("criar %s: %w", dir, err)
	}
	if err := os.Chmod(dir, moodleDataMode); err != nil {
		ui.Warning(stdout, "Não foi possível restringir as permissões de "+dir+": "+err.Error())
	}
	return nil
}

// CreateApp creates one Container Aplicativo: ensures NetworkName and the
// right base image (PHP for PHP-family types, Node for AppTypeNode, the
// PHP+Node combo image for AppTypePHPNode), creates its project folder(s)
// (+data/log for PHP-family types), starts its container, writes its CLI
// wrappers, persists app into cfg.Docker.Apps (replacing any existing
// entry with the same Folder), and regenerates + reloads Nginx's routing
// to include it.
//
// The image build runs before stackMu is taken, so a long build does not
// block the other stack operations.
//
// app.Folder and app.URL must already be validated by the caller
// (ValidAppFolder, ValidAppURL); CreateApp re-checks everything anyway:
// PHPVersion (ValidPHPVersion, plus MoodleVersion/
// ValidPHPVersionForMoodleVersion for AppTypeMoodle) for PHP-family types,
// NodeVersion/DevCommand/DevPort for AppTypeNode, and both for
// AppTypePHPNode.
func CreateApp(ctx context.Context, exe *executor.Executor, stdout io.Writer, app config.AppContainer) error {
	prep, err := prepareApp(ctx, exe, stdout, app)
	if err != nil {
		return err
	}
	stackMu.Lock()
	defer stackMu.Unlock()
	apps, err := startAppContainer(ctx, exe, stdout, app, prep)
	if err != nil {
		return err
	}
	return ReloadNginxConfig(ctx, exe, stdout, apps)
}

// startAppContainer runs app's container from its prepared image and
// folders, writes its CLI wrappers and records app in cfg.Docker.Apps. The
// caller holds stackMu.
func startAppContainer(ctx context.Context, exe *executor.Executor, stdout io.Writer, app config.AppContainer, prep appPrep) ([]config.AppContainer, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("carregar config: %w", err)
	}
	if err := checkURLUnique(app, cfg.Docker.Apps); err != nil {
		return nil, err
	}

	workspace := prep.workspace
	projectDir := AppHTMLDir(workspace, app.Folder)
	runOpts := executor.Options{Stdout: stdout, Stderr: stdout, Env: prep.dbEnv}
	ui.Info(stdout, "Criando contêiner "+app.Folder+"...")

	switch app.Type {
	case config.AppTypeNode:
		if err := mountableError(projectDir); err != nil {
			return nil, err
		}
		args := nodeAppRunArgs(app.Folder, NodeImageName(app.NodeVersion), projectDir, app.DevCommand)
		if err := exe.Run(ctx, runOpts, "docker", args...); err != nil {
			return nil, fmt.Errorf("criar contêiner %s: %w", app.Folder, err)
		}
		removeAppWrappers(app.Folder)
		writeNodeAppWrappers(app.Folder, projectDir, "", stdout)

	case config.AppTypePHPNode:
		apiDir := AppComboAPIDir(workspace, app.Folder)
		comboAppDir := AppComboAppDir(workspace, app.Folder)
		logDir := AppLogDir(workspace, app.Folder)
		phpConfPath, err := WritePHPMemoryLimitConf(app.Folder, app.PHPMemoryLimit)
		if err != nil {
			return nil, err
		}
		if err := mountableError(apiDir, comboAppDir, logDir, phpConfPath); err != nil {
			return nil, err
		}

		args := comboAppRunArgs(app.Folder, ComboImageName(app.PHPVersion, app.NodeVersion), apiDir, comboAppDir, logDir, phpConfPath, app.DevCommand, app.WorkerCommand, prep.dbEnv)
		if err := exe.Run(ctx, runOpts, "docker", args...); err != nil {
			return nil, fmt.Errorf("criar contêiner %s: %w", app.Folder, err)
		}
		removeAppWrappers(app.Folder)
		writeAppWrappers(app.Folder, apiDir, stdout)
		writeNodeAppWrappers(app.Folder, comboAppDir, wrapperUser, stdout)

	default:
		var dataDir string
		if app.Type == config.AppTypeMoodle {
			dataDir = AppDataDir(workspace, app.Folder)
		}
		logDir := AppLogDir(workspace, app.Folder)
		phpConfPath, err := WritePHPMemoryLimitConf(app.Folder, app.PHPMemoryLimit)
		if err != nil {
			return nil, err
		}
		if err := mountableError(projectDir, dataDir, logDir, phpConfPath); err != nil {
			return nil, err
		}

		args := appRunArgs(app.Folder, ImageName(app.PHPVersion), projectDir, dataDir, logDir, phpConfPath, prep.dbEnv)
		if err := exe.Run(ctx, runOpts, "docker", args...); err != nil {
			return nil, fmt.Errorf("criar contêiner %s: %w", app.Folder, err)
		}
		removeAppWrappers(app.Folder)
		writeAppWrappers(app.Folder, projectDir, stdout)
	}

	var apps []config.AppContainer
	if err := config.Update(func(cfg *config.Config) error {
		cfg.Docker.Apps = replaceOrAppendApp(cfg.Docker.Apps, app)
		apps = cfg.Docker.Apps
		return nil
	}); err != nil {
		return nil, fmt.Errorf("contêiner %s criado, mas falha ao salvar configurações (pode ficar órfão da lista): %w", app.Folder, err)
	}

	return apps, nil
}

// replaceAppContainer removes app's current container, if any, and starts
// it again from its prepared image. The caller holds stackMu and has run
// prepareApp, so a failed build never reaches the removal.
func replaceAppContainer(ctx context.Context, exe *executor.Executor, stdout io.Writer, app config.AppContainer, prep appPrep) ([]config.AppContainer, error) {
	if AppExists(ctx, exe, app.Folder) {
		if err := RemoveApp(ctx, exe, stdout, app.Folder); err != nil {
			return nil, err
		}
	}
	return startAppContainer(ctx, exe, stdout, app, prep)
}

// UpdateApp applies an edit to a Container Aplicativo: it validates app
// against the rest of the stack and builds whatever image it needs before
// the running container is touched, then replaces the container with one
// created from the new parameters and reloads Nginx's routing.
func UpdateApp(ctx context.Context, exe *executor.Executor, stdout io.Writer, app config.AppContainer) error {
	if err := ValidateAppInStack(app); err != nil {
		return err
	}
	prep, err := prepareApp(ctx, exe, stdout, app)
	if err != nil {
		return err
	}
	stackMu.Lock()
	defer stackMu.Unlock()
	apps, err := replaceAppContainer(ctx, exe, stdout, app, prep)
	if err != nil {
		return err
	}
	return ReloadNginxConfig(ctx, exe, stdout, apps)
}

// registeredApp returns the validated cfg.Docker.Apps entry for folder.
func registeredApp(folder string) (config.AppContainer, error) {
	cfg, err := config.Load()
	if err != nil {
		return config.AppContainer{}, fmt.Errorf("carregar config: %w", err)
	}
	app, found := FindAppByFolder(cfg.Docker.Apps, folder)
	if !found {
		return config.AppContainer{}, fmt.Errorf("contêiner aplicativo %q não está registrado em cfg.Docker.Apps", folder)
	}
	// Validated before the running container is removed, so a bad entry
	// (hand-edited config, an import) does not leave the app down.
	if err := ValidateApp(app); err != nil {
		return config.AppContainer{}, err
	}
	return app, nil
}

// RecreateApp is CreateApp's "Recriar" flavor: reuses the AppContainer
// already persisted in cfg.Docker.Apps for folder, with no form and no
// confirmation of its own. Errors if folder isn't registered. The image is
// built before the running container is removed and before stackMu is
// taken.
func RecreateApp(ctx context.Context, exe *executor.Executor, stdout io.Writer, folder string) error {
	app, err := registeredApp(folder)
	if err != nil {
		return err
	}
	prep, err := prepareApp(ctx, exe, stdout, app)
	if err != nil {
		return err
	}
	stackMu.Lock()
	defer stackMu.Unlock()
	apps, err := replaceAppContainer(ctx, exe, stdout, app, prep)
	if err != nil {
		return err
	}
	return ReloadNginxConfig(ctx, exe, stdout, apps)
}

// recreateAppContainer is RecreateApp minus the final ReloadNginxConfig
// call, so ImportConfig can recreate N apps and trigger one nginx reload at
// the end. The caller holds stackMu.
func recreateAppContainer(ctx context.Context, exe *executor.Executor, stdout io.Writer, folder string) ([]config.AppContainer, error) {
	app, err := registeredApp(folder)
	if err != nil {
		return nil, err
	}
	prep, err := prepareApp(ctx, exe, stdout, app)
	if err != nil {
		return nil, err
	}
	return replaceAppContainer(ctx, exe, stdout, app, prep)
}

// requireStackContainer refuses a container name that is neither the
// stack's nginx/mariadb nor a registered Container Aplicativo, so the quick
// actions and the log view never act on an unrelated container of the host.
func requireStackContainer(name string) error {
	if name == NginxContainerName || name == MariaDBContainerName {
		return nil
	}
	return requireRegisteredApp(name)
}

// requireRegisteredApp refuses a name that is not a Container Aplicativo
// registered in cfg.Docker.Apps (nginx and mariadb included).
func requireRegisteredApp(name string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("carregar config: %w", err)
	}
	if _, ok := FindAppByFolder(cfg.Docker.Apps, name); !ok {
		return fmt.Errorf("%q não é um contêiner do Perci", name)
	}
	return nil
}

// DeleteApp removes the app's container (if any), drops its entry from
// cfg.Docker.Apps, and regenerates + reloads Nginx's routing without it.
// html/data on disk are never touched. Only a registered Container
// Aplicativo is accepted: nginx, mariadb and unknown names are refused. The
// caller is responsible for confirming with the user first.
func DeleteApp(ctx context.Context, exe *executor.Executor, stdout io.Writer, folder string) error {
	stackMu.Lock()
	defer stackMu.Unlock()
	if err := requireRegisteredApp(folder); err != nil {
		return err
	}
	if AppExists(ctx, exe, folder) {
		if err := RemoveApp(ctx, exe, stdout, folder); err != nil {
			return err
		}
	}
	var apps []config.AppContainer
	if err := config.Update(func(cfg *config.Config) error {
		cfg.Docker.Apps = removeAppByFolder(cfg.Docker.Apps, folder)
		apps = cfg.Docker.Apps
		return nil
	}); err != nil {
		return fmt.Errorf("contêiner %s removido, mas falha ao salvar configurações: %w", folder, err)
	}
	removeAppWrappers(folder)
	return ReloadNginxConfig(ctx, exe, stdout, apps)
}
