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

// appRunArgs builds the "docker run" argument list for a Container
// Aplicativo. dataDir is only included when non-empty (AppTypeMoodle).
// Unit-testable without a Docker daemon.
func appRunArgs(folder, image, htmlDir, dataDir, logDir, phpConfPath string, dbEnv []string) []string {
	args := []string{
		"run", "-d",
		"--name", folder,
		"--network", NetworkName,
		"--restart", "unless-stopped",
		"-v", htmlDir + ":/var/www/html",
	}
	if dataDir != "" {
		args = append(args, "-v", dataDir+":/var/www/data")
	}
	args = append(args, "-v", logDir+":/var/log/php")
	args = append(args, "-v", phpConfPath+":"+phpConfMount+":ro")
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
	return []string{
		"run", "-d",
		"--name", folder,
		"--network", NetworkName,
		"--restart", "unless-stopped",
		"--user", "node",
		"-v", appDir + ":/app",
		"--",
		image,
		"sh", "-c", devCommand,
	}
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
		"-v", apiDir + ":/var/www/html",
		"-v", appDir + ":/app",
		"-v", logDir + ":/var/log/php",
		"-v", phpConfPath + ":" + phpConfMount + ":ro",
		"-e", "DEV_COMMAND=" + devCommand,
		"-e", "WORKER_COMMAND=" + workerCommand,
	}
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

// CreateApp creates one Container Aplicativo: ensures NetworkName and the
// right base image (PHP for PHP-family types, Node for AppTypeNode, the
// PHP+Node combo image for AppTypePHPNode), creates its project folder(s)
// (+data/log for PHP-family types), starts its container, writes its CLI
// wrappers, persists app into cfg.Docker.Apps (replacing any existing
// entry with the same Folder), and regenerates + reloads Nginx's routing
// to include it.
//
// app.Folder and app.URL must already be validated by the caller
// (ValidAppFolder, ValidAppURL); CreateApp re-checks everything anyway:
// PHPVersion (ValidPHPVersion, plus MoodleVersion/
// ValidPHPVersionForMoodleVersion for AppTypeMoodle) for PHP-family types,
// NodeVersion/DevCommand/DevPort for AppTypeNode, and both for
// AppTypePHPNode.
func CreateApp(ctx context.Context, exe *executor.Executor, stdout io.Writer, app config.AppContainer) error {
	stackMu.Lock()
	defer stackMu.Unlock()
	apps, err := createAppContainer(ctx, exe, stdout, app)
	if err != nil {
		return err
	}
	return ReloadNginxConfig(ctx, exe, stdout, apps)
}

// createAppContainer is CreateApp minus the final ReloadNginxConfig call,
// so ImportConfig can recreate N apps and trigger one nginx reload at the
// end, using the final apps list. Returns the updated cfg.Docker.Apps list
// on success.
func createAppContainer(ctx context.Context, exe *executor.Executor, stdout io.Writer, app config.AppContainer) ([]config.AppContainer, error) {
	if err := ValidateApp(app); err != nil {
		return nil, err
	}
	if !isDockerInstalled(ctx, exe) {
		return nil, fmt.Errorf("docker não instalado")
	}

	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("carregar config: %w", err)
	}
	if err := checkURLUnique(app, cfg.Docker.Apps); err != nil {
		return nil, err
	}

	workspace, err := resolveWorkspace(cfg)
	if err != nil {
		return nil, err
	}

	// DBAccess is checked unconditionally rather than gated on app.Type, so a
	// hand-edited config enabling it on a Node app is rejected instead of
	// silently doing nothing.
	var dbEnv []string
	if app.DBAccess {
		if cfg.Docker.MariaDB.DBUser == "" {
			return nil, fmt.Errorf(`nenhum Contêiner MariaDB configurado ainda — crie um em "Gerenciar Docker :: Criar Contêiner MariaDB" antes de habilitar acesso a banco`)
		}
		dbEnv = dbAccessEnv(cfg.Docker.MariaDB)
	}

	if err := EnsureNetwork(ctx, exe, stdout); err != nil {
		return nil, err
	}

	projectDir := AppHTMLDir(workspace, app.Folder)
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		return nil, fmt.Errorf("criar %s: %w", projectDir, err)
	}

	ui.Info(stdout, "Criando contêiner "+app.Folder+"...")

	switch app.Type {
	case config.AppTypeNode:
		if err := EnsureNodeImage(ctx, exe, stdout, app.NodeVersion); err != nil {
			return nil, err
		}
		args := nodeAppRunArgs(app.Folder, NodeImageName(app.NodeVersion), projectDir, app.DevCommand)
		if err := exe.Run(ctx, executor.Options{Stdout: stdout, Stderr: stdout}, "docker", args...); err != nil {
			return nil, fmt.Errorf("criar contêiner %s: %w", app.Folder, err)
		}
		writeNodeAppWrappers(app.Folder, projectDir, "", stdout)

	case config.AppTypePHPNode:
		if err := EnsureComboImage(ctx, exe, stdout, app.PHPVersion, app.NodeVersion); err != nil {
			return nil, err
		}

		apiDir := AppComboAPIDir(workspace, app.Folder)
		if err := os.MkdirAll(apiDir, 0o755); err != nil {
			return nil, fmt.Errorf("criar %s: %w", apiDir, err)
		}
		comboAppDir := AppComboAppDir(workspace, app.Folder)
		if err := os.MkdirAll(comboAppDir, 0o755); err != nil {
			return nil, fmt.Errorf("criar %s: %w", comboAppDir, err)
		}
		logDir := AppLogDir(workspace, app.Folder)
		if err := os.MkdirAll(logDir, 0o755); err != nil {
			return nil, fmt.Errorf("criar %s: %w", logDir, err)
		}
		phpConfPath, err := WritePHPMemoryLimitConf(app.Folder, app.PHPMemoryLimit)
		if err != nil {
			return nil, err
		}

		args := comboAppRunArgs(app.Folder, ComboImageName(app.PHPVersion, app.NodeVersion), apiDir, comboAppDir, logDir, phpConfPath, app.DevCommand, app.WorkerCommand, dbEnv)
		if err := exe.Run(ctx, executor.Options{Stdout: stdout, Stderr: stdout, Env: dbEnv}, "docker", args...); err != nil {
			return nil, fmt.Errorf("criar contêiner %s: %w", app.Folder, err)
		}
		writeAppWrappers(app.Folder, apiDir, stdout)
		writeNodeAppWrappers(app.Folder, comboAppDir, wrapperUser, stdout)

	default:
		if err := EnsureImage(ctx, exe, stdout, app.PHPVersion); err != nil {
			return nil, err
		}

		var dataDir string
		if app.Type == config.AppTypeMoodle {
			dataDir = AppDataDir(workspace, app.Folder)
			if err := os.MkdirAll(dataDir, 0o755); err != nil {
				return nil, fmt.Errorf("criar %s: %w", dataDir, err)
			}
		}

		logDir := AppLogDir(workspace, app.Folder)
		if err := os.MkdirAll(logDir, 0o755); err != nil {
			return nil, fmt.Errorf("criar %s: %w", logDir, err)
		}
		phpConfPath, err := WritePHPMemoryLimitConf(app.Folder, app.PHPMemoryLimit)
		if err != nil {
			return nil, err
		}

		args := appRunArgs(app.Folder, ImageName(app.PHPVersion), projectDir, dataDir, logDir, phpConfPath, dbEnv)
		if err := exe.Run(ctx, executor.Options{Stdout: stdout, Stderr: stdout, Env: dbEnv}, "docker", args...); err != nil {
			return nil, fmt.Errorf("criar contêiner %s: %w", app.Folder, err)
		}
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

// RecreateApp is CreateApp's "Recriar" flavor: reuses the AppContainer
// already persisted in cfg.Docker.Apps for folder, with no form and no
// confirmation of its own. Errors if folder isn't registered.
func RecreateApp(ctx context.Context, exe *executor.Executor, stdout io.Writer, folder string) error {
	stackMu.Lock()
	defer stackMu.Unlock()
	apps, err := recreateAppContainer(ctx, exe, stdout, folder)
	if err != nil {
		return err
	}
	return ReloadNginxConfig(ctx, exe, stdout, apps)
}

// recreateAppContainer is RecreateApp minus the final ReloadNginxConfig
// call, so ImportConfig can recreate N apps and trigger one nginx reload at
// the end.
func recreateAppContainer(ctx context.Context, exe *executor.Executor, stdout io.Writer, folder string) ([]config.AppContainer, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("carregar config: %w", err)
	}
	app, found := FindAppByFolder(cfg.Docker.Apps, folder)
	if !found {
		return nil, fmt.Errorf("contêiner aplicativo %q não está registrado em cfg.Docker.Apps", folder)
	}
	// Validated before the running container is removed, so a bad entry
	// (hand-edited config, an import) does not leave the app down.
	if err := ValidateApp(app); err != nil {
		return nil, err
	}
	if AppExists(ctx, exe, folder) {
		if err := RemoveApp(ctx, exe, stdout, folder); err != nil {
			return nil, err
		}
	}
	return createAppContainer(ctx, exe, stdout, app)
}

// DeleteApp removes the app's container (if any), drops its entry from
// cfg.Docker.Apps, and regenerates + reloads Nginx's routing without it.
// html/data on disk are never touched. The caller is responsible for
// confirming with the user first.
func DeleteApp(ctx context.Context, exe *executor.Executor, stdout io.Writer, folder string) error {
	stackMu.Lock()
	defer stackMu.Unlock()
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
