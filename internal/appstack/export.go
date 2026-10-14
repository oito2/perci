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
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/oito2/perci/internal/config"
	"github.com/oito2/perci/internal/executor"
	"github.com/oito2/perci/internal/fsutil"
	"github.com/oito2/perci/internal/ui"
)

// ExportedConfig is the file format ExportConfig writes and ImportConfig
// reads: a portable snapshot of the Docker stack definition (Nginx flag,
// MariaDB credentials, Containers Aplicativo), the Moodle plugin mounts,
// and the WorkspacePath it was created against.
//
// WorkspacePath travels alongside the items so ImportConfig can refuse to
// import onto a machine whose own WorkspacePath doesn't match (see
// ErrWorkspaceMismatch): every AppContainer's Folder is only meaningful
// relative to a workspace. Plugin mounts carry no MEGA exclusion: the
// import writes the one the importing machine needs.
type ExportedConfig struct {
	WorkspacePath string                     `yaml:"workspace_path"`
	Docker        config.DockerConfig        `yaml:"docker"`
	MoodlePlugins []config.MoodlePluginMount `yaml:"moodle_plugins,omitempty"`
}

// TransferSelection names the items an export or an import covers: the
// Nginx and MariaDB containers, apps by folder and plugin mounts by
// PluginKey.
type TransferSelection struct {
	Nginx   bool
	MariaDB bool
	Apps    []string
	Plugins []string
}

// PluginKey identifies a plugin mount: "<app>/<rel_path>".
func PluginKey(m config.MoodlePluginMount) string {
	return m.App + "/" + filepath.ToSlash(filepath.Clean(m.RelPath))
}

// SelectAll returns the selection of every item in e.
func SelectAll(e ExportedConfig) TransferSelection {
	sel := TransferSelection{Nginx: e.Docker.NginxCreated, MariaDB: e.Docker.MariaDB.DBUser != ""}
	for _, app := range e.Docker.Apps {
		sel.Apps = append(sel.Apps, app.Folder)
	}
	for _, m := range e.MoodlePlugins {
		sel.Plugins = append(sel.Plugins, PluginKey(m))
	}
	return sel
}

// Filter returns the part of e that sel names, keeping e's workspace. A
// name that is not in e is an error.
func (e ExportedConfig) Filter(sel TransferSelection) (ExportedConfig, error) {
	out := ExportedConfig{WorkspacePath: e.WorkspacePath}
	if sel.Nginx {
		if !e.Docker.NginxCreated {
			return ExportedConfig{}, errors.New("o Nginx não faz parte das configurações")
		}
		out.Docker.NginxCreated = true
	}
	if sel.MariaDB {
		if e.Docker.MariaDB.DBUser == "" {
			return ExportedConfig{}, errors.New("o MariaDB não faz parte das configurações")
		}
		out.Docker.MariaDB = e.Docker.MariaDB
	}
	for _, folder := range sel.Apps {
		app, ok := FindAppByFolder(e.Docker.Apps, folder)
		if !ok {
			return ExportedConfig{}, fmt.Errorf("o aplicativo %q não faz parte das configurações", folder)
		}
		if _, dup := FindAppByFolder(out.Docker.Apps, folder); !dup {
			out.Docker.Apps = append(out.Docker.Apps, app)
		}
	}
	for _, key := range sel.Plugins {
		m, ok := findPluginByKey(e.MoodlePlugins, key)
		if !ok {
			return ExportedConfig{}, fmt.Errorf("o plugin montado %q não faz parte das configurações", key)
		}
		if _, dup := findPluginByKey(out.MoodlePlugins, key); !dup {
			out.MoodlePlugins = append(out.MoodlePlugins, m)
		}
	}
	return out, nil
}

// empty reports whether e holds no item.
func (e ExportedConfig) empty() bool {
	return !e.Docker.NginxCreated && e.Docker.MariaDB.DBUser == "" && len(e.Docker.Apps) == 0 && len(e.MoodlePlugins) == 0
}

// findPluginByKey returns the mount whose PluginKey is key.
func findPluginByKey(mounts []config.MoodlePluginMount, key string) (config.MoodlePluginMount, bool) {
	for _, m := range mounts {
		if PluginKey(m) == key {
			return m, true
		}
	}
	return config.MoodlePluginMount{}, false
}

// ErrNothingToExport is returned by ExportConfig when nothing is registered
// yet (or nothing was selected).
var ErrNothingToExport = errors.New("nada para exportar — nenhum contêiner Docker ou plugin montado selecionado")

// Exportable returns everything the local config can export: the Docker
// stack, the plugin mounts without their MEGA exclusions, and the
// workspace.
func Exportable(cfg *config.Config) ExportedConfig {
	e := ExportedConfig{WorkspacePath: cfg.WorkspacePath, Docker: cfg.Docker}
	for _, m := range cfg.MoodlePlugins {
		m.MegaIgnore, m.MegaIgnoreRule = "", ""
		e.MoodlePlugins = append(e.MoodlePlugins, m)
	}
	return e
}

// ExportConfig writes every exportable item (see Exportable) to path, as
// YAML. MariaDB credentials are included in plaintext.
func ExportConfig(path string) error {
	return exportTo(path, nil)
}

// ExportSelected writes the items sel names to path, as ExportConfig does.
func ExportSelected(path string, sel TransferSelection) error {
	return exportTo(path, &sel)
}

// exportTo writes the exportable items, filtered by sel when it is not
// nil, to path.
func exportTo(path string, sel *TransferSelection) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("carregar config: %w", err)
	}
	exported := Exportable(cfg)
	if sel != nil {
		if exported, err = exported.Filter(*sel); err != nil {
			return err
		}
	}
	if exported.empty() {
		return ErrNothingToExport
	}
	data, err := yaml.Marshal(exported)
	if err != nil {
		return fmt.Errorf("gerar yaml: %w", err)
	}
	// The file contains MariaDB credentials, so it is written atomically with
	// 0600 permissions, even when overwriting an existing file.
	if err := fsutil.WriteFileAtomic(path, data, 0o600); err != nil {
		return fmt.Errorf("escrever %s: %w", path, err)
	}
	return nil
}

// LoadExportedConfig reads and parses path, so a caller can show a summary
// (or a clear "file not found"/parse error) before anything destructive.
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
// WorkspacePath configured and it disagrees with exported.WorkspacePath;
// the mismatch is refused rather than silently overridden.
var ErrWorkspaceMismatch = errors.New("workspace path não corresponde ao da exportação")

// pluginComponentPattern is a Moodle component: <type>_<name>.
var pluginComponentPattern = regexp.MustCompile(`^[a-z][a-z0-9]*_[a-z][a-z0-9_]*$`)

// validateExported checks everything an import would apply: every app
// (ValidateApp, plus unique folders and URLs), the MariaDB user, every
// plugin mount, and an absolute workspace path. Nothing in the file is
// trusted to be well-formed.
func validateExported(e ExportedConfig) error {
	if e.WorkspacePath != "" && !filepath.IsAbs(e.WorkspacePath) {
		return fmt.Errorf("workspace_path deve ser um caminho absoluto: %q", e.WorkspacePath)
	}
	if u := e.Docker.MariaDB.DBUser; u != "" {
		if !ValidDBIdentifier.MatchString(u) {
			return fmt.Errorf("usuário do banco inválido: %q", u)
		}
		if err := validateDBPassword(e.Docker.MariaDB.DBPass); err != nil {
			return err
		}
	}
	for _, p := range []string{e.Docker.MariaDB.DBRootPass, e.Docker.MariaDB.DataPass} {
		if strings.ContainsAny(p, "\r\n") {
			return fmt.Errorf("senha do banco inválida: não pode conter quebra de linha")
		}
	}
	if u := e.Docker.MariaDB.DataUser; u != "" && !ValidDBIdentifier.MatchString(u) {
		return fmt.Errorf("usuário do banco inválido: %q", u)
	}
	folders := map[string]bool{}
	for _, app := range e.Docker.Apps {
		if err := ValidateApp(app); err != nil {
			return fmt.Errorf("aplicativo %q: %w", app.Folder, err)
		}
		if strings.ContainsAny(app.DevCommand+app.WorkerCommand, "\r\n") {
			return fmt.Errorf("aplicativo %q: os comandos não podem conter quebra de linha", app.Folder)
		}
		if folders[app.Folder] {
			return fmt.Errorf("aplicativo %q aparece mais de uma vez", app.Folder)
		}
		folders[app.Folder] = true
		if err := checkURLUnique(app, e.Docker.Apps); err != nil {
			return err
		}
	}
	keys := map[string]bool{}
	for _, m := range e.MoodlePlugins {
		key := PluginKey(m)
		switch {
		case !ValidAppFolder.MatchString(m.App):
			return fmt.Errorf("plugin montado %q: Moodle inválido", key)
		case !pluginComponentPattern.MatchString(m.Component):
			return fmt.Errorf("plugin montado %q: componente inválido %q", key, m.Component)
		case !filepath.IsLocal(m.RelPath):
			return fmt.Errorf("plugin montado %q: destino fora do Moodle", key)
		case !filepath.IsAbs(m.DevPath):
			return fmt.Errorf("plugin montado %q: a pasta de desenvolvimento deve ser um caminho absoluto", key)
		case keys[key]:
			return fmt.Errorf("plugin montado %q aparece mais de uma vez", key)
		}
		keys[key] = true
	}
	return nil
}

// resolveImportWorkspace decides what cfg.WorkspacePath should become after
// importing: adopting exportedWorkspacePath when the local config has none
// yet, or refusing with ErrWorkspaceMismatch when the two disagree.
func resolveImportWorkspace(localWorkspacePath, exportedWorkspacePath string) (string, error) {
	if localWorkspacePath == "" {
		return exportedWorkspacePath, nil
	}
	if exportedWorkspacePath != "" && exportedWorkspacePath != localWorkspacePath {
		return "", fmt.Errorf("%w: exportado de %q, workspace local é %q", ErrWorkspaceMismatch, exportedWorkspacePath, localWorkspacePath)
	}
	return localWorkspacePath, nil
}

// mergeApps returns local with every app of incoming added, an app with the
// same folder being replaced.
func mergeApps(local, incoming []config.AppContainer) []config.AppContainer {
	merged := append([]config.AppContainer(nil), local...)
	for _, app := range incoming {
		replaced := false
		for i := range merged {
			if merged[i].Folder == app.Folder {
				merged[i] = app
				replaced = true
			}
		}
		if !replaced {
			merged = append(merged, app)
		}
	}
	return merged
}

// PluginImportState tells whether a plugin mount of an export file can be
// mounted on this machine. Mounted means the same mount already exists;
// otherwise Reason says why it can't be mounted (empty when it can).
type PluginImportState struct {
	Mount   config.MoodlePluginMount
	Mounted bool
	Reason  string
}

// PluginImportStates checks every plugin mount of e against this machine,
// as if every app of e were imported.
func PluginImportStates(e ExportedConfig) ([]PluginImportState, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("carregar config: %w", err)
	}
	workspace, err := resolveImportWorkspace(cfg.WorkspacePath, e.WorkspacePath)
	if err != nil {
		return nil, err
	}
	check := *cfg
	check.WorkspacePath = workspace
	check.Docker.Apps = mergeApps(cfg.Docker.Apps, e.Docker.Apps)
	states := make([]PluginImportState, 0, len(e.MoodlePlugins))
	for _, m := range e.MoodlePlugins {
		state := PluginImportState{Mount: m}
		state.Mounted, state.Reason = pluginImportCheck(&check, m)
		states = append(states, state)
	}
	return states, nil
}

// pluginImportCheck tells whether m can be mounted with cfg: mounted when
// the same mount is already recorded, otherwise a reason it can't be
// (empty when it can).
func pluginImportCheck(cfg *config.Config, m config.MoodlePluginMount) (mounted bool, reason string) {
	if local, ok := findPluginByKey(cfg.MoodlePlugins, PluginKey(m)); ok {
		if local.DevPath == m.DevPath {
			return true, ""
		}
		return false, "outro plugin já está montado nesse destino (" + local.DevPath + ")"
	}
	if app, ok := FindAppByFolder(cfg.Docker.Apps, m.App); !ok || app.Type != config.AppTypeMoodle {
		return false, "o Moodle " + m.App + " não existe nesta máquina nem no arquivo"
	}
	if info, err := os.Stat(m.DevPath); err != nil || !info.IsDir() {
		return false, "a pasta de desenvolvimento " + m.DevPath + " não existe nesta máquina"
	}
	ptype, name, _ := strings.Cut(m.Component, "_")
	if _, err := PlanMoodlePlugin(cfg, m.DevPath, m.App, ptype, name); err != nil {
		return false, err.Error()
	}
	return false, ""
}

// ImportConfig applies the items of exported that sel names onto the local
// config.yaml (locking cfg.WorkspacePath via resolveImportWorkspace),
// merging them: an app with the same folder is replaced, everything else
// local stays. It then mounts the selected plugin mounts (one password
// prompt; a mount that can't be made on this machine is skipped with a
// warning) and recreates the selected containers (Nginx, then MariaDB, then
// every Container Aplicativo, in that dependency order) by reusing each
// type's own Recreate* function. Running containers of a newly mounted
// plugin that weren't recreated are restarted so they see it.
//
// The whole file is validated before anything is saved or touched
// (validateExported), and so are the selection's dependencies: an app
// with database access needs MariaDB, here or selected. After that it
// stops at the first container error, leaving the config saved (so a
// second import or a manual recreate can pick up where it left off); a
// partially applied import is not rolled back. Skipped plugin mounts make
// it return executor.ErrCompletedWithWarnings.
func ImportConfig(ctx context.Context, exe *executor.Executor, stdin io.Reader, stdout io.Writer, exported ExportedConfig, sel TransferSelection) error {
	if err := validateExported(exported); err != nil {
		return fmt.Errorf("arquivo de configurações inválido: %w", err)
	}
	part, err := exported.Filter(sel)
	if err != nil {
		return err
	}
	if part.empty() {
		return errors.New("nenhum item selecionado para importar")
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

	merged := mergeApps(cfg.Docker.Apps, part.Docker.Apps)
	hasDB := cfg.Docker.MariaDB.DBUser != "" || part.Docker.MariaDB.DBUser != ""
	for _, app := range part.Docker.Apps {
		if err := checkURLUnique(app, merged); err != nil {
			return err
		}
		if app.DBAccess && !hasDB {
			return fmt.Errorf("o aplicativo %q usa o banco: selecione também o MariaDB", app.Folder)
		}
	}

	checkCfg := *cfg
	checkCfg.WorkspacePath = workspacePath
	checkWorkspace, err := resolveWorkspace(&checkCfg)
	if err != nil {
		return err
	}
	// localData: the MariaDB data directory under the imported workspace
	// already exists, so the local secrets are the ones it was created with.
	localData := mariadbDataInitialized(MariaDBDataDir(checkWorkspace))

	// The imported database credentials are checked against the local data
	// directory before anything is saved or removed: a file from another
	// machine must not strand the database already on disk.
	if u := part.Docker.MariaDB.DBUser; u != "" {
		if err := checkMariaDBCredentials(stdout, &checkCfg, checkWorkspace, u, part.Docker.MariaDB.DBPass); err != nil {
			return err
		}
	}

	if err := config.Update(func(cfg *config.Config) error {
		cfg.WorkspacePath = workspacePath
		if part.Docker.NginxCreated {
			cfg.Docker.NginxCreated = true
		}
		if part.Docker.MariaDB.DBUser != "" {
			// With an existing local data directory, the local root and data
			// secrets win (they are the ones it was created with); otherwise
			// the file's win. Missing values are never blanked.
			m, local := part.Docker.MariaDB, cfg.Docker.MariaDB
			if localData {
				m.DBRootPass = cmp.Or(local.DBRootPass, m.DBRootPass)
				m.DataUser = cmp.Or(local.DataUser, m.DataUser)
				m.DataPass = cmp.Or(local.DataPass, m.DataPass)
			} else {
				m.DBRootPass = cmp.Or(m.DBRootPass, local.DBRootPass)
				m.DataUser = cmp.Or(m.DataUser, local.DataUser)
				m.DataPass = cmp.Or(m.DataPass, local.DataPass)
			}
			cfg.Docker.MariaDB = m
		}
		cfg.Docker.Apps = mergeApps(cfg.Docker.Apps, part.Docker.Apps)
		return nil
	}); err != nil {
		return fmt.Errorf("salvar config: %w", err)
	}

	// Plugin mounts come before the containers, so the containers recreated
	// below already see them.
	pluginApps, skipped := importPlugins(ctx, exe, stdout, part.MoodlePlugins)

	if part.Docker.NginxCreated {
		ui.Info(stdout, "Recriando Contêiner Nginx...")
		if _, err := recreateNginx(ctx, exe, stdin, stdout, false); err != nil {
			return fmt.Errorf("recriar nginx: %w", err)
		}
	}

	if part.Docker.MariaDB.DBUser != "" {
		ui.Info(stdout, "Recriando Contêiner MariaDB...")
		if err := RecreateMariaDB(ctx, exe, stdout); err != nil {
			return fmt.Errorf("recriar mariadb: %w", err)
		}
	}

	// recreateAppContainer (not RecreateApp) so N apps trigger only one nginx
	// reload below, using the final apps list; each app is already reflected
	// in config.yaml at this point.
	recreated := map[string]bool{}
	apps := merged
	for _, app := range part.Docker.Apps {
		ui.Info(stdout, "Recriando Contêiner Aplicativo "+app.Folder+"...")
		if apps, err = recreateAppContainer(ctx, exe, stdout, app.Folder); err != nil {
			return fmt.Errorf("recriar app %s: %w", app.Folder, err)
		}
		recreated[app.Folder] = true
	}
	if part.Docker.NginxCreated || len(part.Docker.Apps) > 0 {
		if err := ReloadNginxConfig(ctx, exe, stdout, apps); err != nil {
			return fmt.Errorf("recarregar nginx: %w", err)
		}
	}

	// Containers that were running before the import and weren't recreated
	// only see a new plugin mount after a restart.
	if len(pluginApps) > 0 {
		for app := range pluginApps {
			if !recreated[app] && ContainerStatus(ctx, exe, app) == "running" {
				ui.Info(stdout, "Reiniciando o contêiner "+app+" para aplicar os plugins...")
				if err := RestartContainer(ctx, exe, stdout, app); err != nil {
					return err
				}
			}
		}
		if !part.Docker.NginxCreated && nginxContainerRunning(ctx, exe) {
			ui.Info(stdout, "Reiniciando o contêiner Nginx...")
			if err := RestartContainer(ctx, exe, stdout, NginxContainerName); err != nil {
				return err
			}
		}
	}

	if skipped > 0 {
		ui.Warning(stdout, fmt.Sprintf("%d plugin(s) montado(s) do arquivo não foram montados nesta máquina (veja os avisos acima).", skipped))
		return executor.ErrCompletedWithWarnings
	}
	ui.Success(stdout, "Configurações importadas.")
	return nil
}

// importPlugins mounts the plugin mounts of an import with the saved
// config, all in one privileged batch. Mounts already in place are left
// alone; mounts that can't be made on this machine are skipped with a
// warning. It returns the apps that got a new mount and how many mounts
// were skipped. The caller holds stackMu.
func importPlugins(ctx context.Context, exe *executor.Executor, stdout io.Writer, mounts []config.MoodlePluginMount) (apps map[string]bool, skipped int) {
	if len(mounts) == 0 {
		return nil, 0
	}
	cfg, err := config.Load()
	if err != nil {
		ui.Warning(stdout, "Plugins montados não importados: carregar config: "+err.Error())
		return nil, len(mounts)
	}
	var prepared []preparedMount
	for _, m := range mounts {
		key := PluginKey(m)
		mounted, reason := pluginImportCheck(cfg, m)
		if mounted {
			ui.Info(stdout, "Plugin "+m.Component+" já está montado em "+key+".")
			continue
		}
		if reason != "" {
			ui.Warning(stdout, "Plugin "+m.Component+" ("+key+") não montado: "+reason)
			skipped++
			continue
		}
		ptype, name, _ := strings.Cut(m.Component, "_")
		plan, err := PlanMoodlePlugin(cfg, m.DevPath, m.App, ptype, name)
		if err == nil {
			var p preparedMount
			if p, err = prepareMount(stdout, plan); err == nil {
				prepared = append(prepared, p)
				// Later mounts of this import see this destination as taken.
				cfg.MoodlePlugins = append(cfg.MoodlePlugins, p.record)
				continue
			}
		}
		ui.Warning(stdout, "Plugin "+m.Component+" ("+key+") não montado: "+err.Error())
		skipped++
	}
	if len(prepared) == 0 {
		return nil, skipped
	}
	ui.Info(stdout, fmt.Sprintf("Montando %d plugin(s)...", len(prepared)))
	kept, err := installPrepared(ctx, exe, stdout, prepared)
	if err != nil {
		ui.Warning(stdout, "Falha ao montar os plugins: "+err.Error())
		skipped += len(prepared) - len(kept)
	}
	apps = map[string]bool{}
	for _, p := range kept {
		apps[p.plan.App] = true
		ui.Success(stdout, p.plan.Component+" montado em "+p.plan.Dest+".")
	}
	return apps, skipped
}

// Validate checks e the way ImportConfig does before applying anything.
func (e ExportedConfig) Validate() error {
	return validateExported(e)
}
