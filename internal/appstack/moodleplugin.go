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

// Moodle plugin mounts: a plugin development folder is bind-mounted on the
// host over an empty folder inside a Moodle app's code (for example
// html/<app>/local/<name>). The mount is a systemd mount unit ordered
// before docker.service, so it is back after a reboot before any container
// starts. Containers see host mounts that already exist under their own
// bind mounts when they start, so a running Moodle container (and Nginx)
// picks a new mount up on its next restart. Inside the container the
// plugin is a real folder, so PHP's __DIR__ stays inside the Moodle tree.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"

	"github.com/oito2/perci/internal/config"
	"github.com/oito2/perci/internal/executor"
	"github.com/oito2/perci/internal/ui"
)

// systemdUnitDir is where the mount units are installed.
var systemdUnitDir = "/etc/systemd/system"

// Status values of a Moodle plugin mount, see MoodlePluginStatuses.
const (
	PluginApplied  = "applied"  // the app container (and Nginx, when running) see the plugin
	PluginPending  = "pending"  // mounted on the host, waiting for a container restart
	PluginStopped  = "stopped"  // mounted on the host, the app container is not running
	PluginInactive = "inactive" // the host mount is not active
	PluginNoApp    = "missing"  // the app container does not exist
)

// pluginNamePattern is Moodle's rule for a plugin name (the part after
// the type in its component).
var pluginNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// componentPattern finds $plugin->component in a version.php.
var componentPattern = regexp.MustCompile(`\$plugin->component\s*=\s*['"]([a-z][a-z0-9]*)_([a-z][a-z0-9_]*)['"]`)

// MoodleApps returns the registered Moodle containers.
func MoodleApps(apps []config.AppContainer) []config.AppContainer {
	var out []config.AppContainer
	for _, a := range apps {
		if a.Type == config.AppTypeMoodle {
			out = append(out, a)
		}
	}
	return out
}

// MoodlePluginTypes reads the plugin types a Moodle accepts from its
// lib/components.json: type → folder relative to the app's html folder
// (for example "local" → "local" on Moodle 4.x, "public/local" on 5.1+).
func MoodlePluginTypes(workspace, folder string) (map[string]string, error) {
	path := filepath.Join(AppHTMLDir(workspace, folder), "lib", "components.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("não foi possível ler os tipos de plugin do Moodle %s (%s): %w", folder, path, err)
	}
	var parsed struct {
		PluginTypes map[string]string `json:"plugintypes"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("components.json inválido em %s: %w", path, err)
	}
	if len(parsed.PluginTypes) == 0 {
		return nil, fmt.Errorf("nenhum tipo de plugin em %s", path)
	}
	return parsed.PluginTypes, nil
}

// DevPluginComponent returns the type and name declared by
// $plugin->component in dev/version.php, or empty strings when the file
// does not exist or declares no component.
func DevPluginComponent(dev string) (ptype, name string) {
	data, err := os.ReadFile(filepath.Join(dev, "version.php"))
	if err != nil {
		return "", ""
	}
	m := componentPattern.FindSubmatch(data)
	if m == nil {
		return "", ""
	}
	return string(m[1]), string(m[2])
}

// MoodlePluginPlan is a validated mount: Dest is the folder inside the
// app's html folder (RelPath relative to it) where DevPath will appear.
type MoodlePluginPlan struct {
	App       string
	Component string
	DevPath   string
	Dest      string
	RelPath   string
	HTML      string // the app's html folder, links resolved: Dest must stay inside it
}

// PlanMoodlePlugin validates mounting dev as the plugin ptype_name into
// the Moodle app folder. dev must be an existing folder outside the app's
// code; the destination must not exist yet or be an empty folder, and no
// other record may use it. When dev/version.php declares a component, it
// must be ptype_name.
func PlanMoodlePlugin(cfg *config.Config, dev, folder, ptype, name string) (MoodlePluginPlan, error) {
	workspace, err := resolveWorkspace(cfg)
	if err != nil {
		return MoodlePluginPlan{}, err
	}
	app, found := FindAppByFolder(cfg.Docker.Apps, folder)
	if !found || app.Type != config.AppTypeMoodle {
		return MoodlePluginPlan{}, fmt.Errorf("%q não é um contêiner Moodle do Perci", folder)
	}
	if !pluginNamePattern.MatchString(name) {
		return MoodlePluginPlan{}, fmt.Errorf("nome do plugin inválido: use letras minúsculas, números e _, começando por letra")
	}
	types, err := MoodlePluginTypes(workspace, folder)
	if err != nil {
		return MoodlePluginPlan{}, err
	}
	typeDir, ok := types[ptype]
	if !ok {
		return MoodlePluginPlan{}, fmt.Errorf("tipo de plugin %q não existe no Moodle %s", ptype, folder)
	}
	if declType, declName := DevPluginComponent(dev); declType != "" && (declType != ptype || declName != name) {
		return MoodlePluginPlan{}, fmt.Errorf("o version.php da pasta de desenvolvimento declara %s_%s", declType, declName)
	}

	devPath, err := resolveExistingDir(dev)
	if err != nil {
		return MoodlePluginPlan{}, err
	}
	html, err := resolveExistingDir(AppHTMLDir(workspace, folder))
	if err != nil {
		return MoodlePluginPlan{}, err
	}
	relPath := filepath.Join(typeDir, name)
	if typeDir == "" || filepath.IsAbs(typeDir) || !filepath.IsLocal(relPath) {
		return MoodlePluginPlan{}, fmt.Errorf("o tipo de plugin %q aponta para fora do Moodle", ptype)
	}
	dest := filepath.Join(html, relPath)
	if !resolvedWithin(html, dest) {
		return MoodlePluginPlan{}, fmt.Errorf("o destino %s fica fora do Moodle %s", dest, folder)
	}
	if within(html, devPath) || within(devPath, html) {
		return MoodlePluginPlan{}, fmt.Errorf("a pasta de desenvolvimento não pode ficar dentro do Moodle %s nem conter o Moodle", folder)
	}
	for _, p := range []string{devPath, dest} {
		if strings.ContainsFunc(p, func(r rune) bool { return r <= ' ' || r == 0x7f || r == '\\' }) {
			return MoodlePluginPlan{}, fmt.Errorf("caminho com espaço, barra invertida ou caractere de controle não é suportado: %q", p)
		}
	}
	for _, m := range cfg.MoodlePlugins {
		if m.App == folder && filepath.Clean(m.RelPath) == relPath {
			return MoodlePluginPlan{}, fmt.Errorf("%s_%s já está montado no Moodle %s", ptype, name, folder)
		}
	}
	if err := requireEmptyOrMissing(dest); err != nil {
		return MoodlePluginPlan{}, err
	}
	return MoodlePluginPlan{
		App: folder, Component: ptype + "_" + name, DevPath: devPath, Dest: dest, RelPath: relPath, HTML: html,
	}, nil
}

// requireEmptyOrMissing refuses a destination that already has content
// (or isn't a plain folder).
func requireEmptyOrMissing(dest string) error {
	info, err := os.Lstat(dest)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("já existe um arquivo ou link em %s", dest)
	}
	entries, err := os.ReadDir(dest)
	if err != nil {
		return err
	}
	if len(entries) > 0 {
		return fmt.Errorf("já existe conteúdo em %s — mova ou apague essa pasta antes de montar o plugin", dest)
	}
	return nil
}

// AddMoodlePlugin creates the empty destination folder, records the mount,
// then installs and starts the mount unit (one password prompt). When the
// unit cannot be installed or started, the privileged batch removes it
// again and the record and the new folder are dropped. Containers pick the
// mount up on their next restart. A destination inside a MEGA-synced folder
// is first excluded in that sync's .megaignore; the mount is refused when
// the exclusion cannot be written.
func AddMoodlePlugin(ctx context.Context, exe *executor.Executor, stdout io.Writer, dev, folder, ptype, name string) error {
	stackMu.Lock()
	defer stackMu.Unlock()
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("carregar config: %w", err)
	}
	plan, err := PlanMoodlePlugin(cfg, dev, folder, ptype, name)
	if err != nil {
		return err
	}

	// The MEGA exclusion is written before the destination folder exists,
	// so MEGAsync never scans the folder that is about to be mounted.
	megaFile, megaRule, err := excludeFromMega(stdout, plan.Dest)
	if err != nil {
		return err
	}
	_, statErr := os.Lstat(plan.Dest)
	created := errors.Is(statErr, fs.ErrNotExist)
	dropDest := func() {
		if created {
			_ = os.Remove(plan.Dest)
		}
		if megaRule != "" {
			_ = removeMegaIgnoreRule(megaFile, megaRule)
		}
	}
	if err := os.MkdirAll(plan.Dest, 0o755); err != nil {
		dropDest()
		return fmt.Errorf("criar %s: %w", plan.Dest, err)
	}
	// Checked again right before the privileged step: the folders created
	// above must not have led outside the Moodle through a link.
	if !resolvedWithin(plan.HTML, plan.Dest) {
		dropDest()
		return fmt.Errorf("o destino %s fica fora do Moodle %s", plan.Dest, folder)
	}

	record := config.MoodlePluginMount{
		App: plan.App, Component: plan.Component, DevPath: plan.DevPath, RelPath: plan.RelPath,
		MegaIgnore: megaFile, MegaIgnoreRule: megaRule,
	}
	if err := config.Update(func(c *config.Config) error {
		c.MoodlePlugins = append(c.MoodlePlugins, record)
		return nil
	}); err != nil {
		dropDest()
		return err
	}
	if err := installMountUnit(ctx, exe, stdout, plan); err != nil {
		// A mount the rollback could not stop keeps its unit, so the record
		// is kept too: Remover can still undo it.
		if mountActive(plan.DevPath, plan.Dest) {
			return fmt.Errorf("%w — a montagem continua ativa e o registro foi mantido; use Remover para desfazê-la", err)
		}
		_ = config.Update(func(c *config.Config) error {
			c.MoodlePlugins = withoutMoodlePlugin(c.MoodlePlugins, plan.App, plan.RelPath)
			return nil
		})
		dropDest()
		return err
	}
	ui.Success(stdout, fmt.Sprintf("%s montado em %s. Para aplicar, use \"Reiniciar contêiner\" na aba Plugins montados.", plan.Component, plan.Dest))
	return nil
}

// mountActive reports whether dev is mounted over dest. Replaced in tests.
var mountActive = sameDir

// withoutMoodlePlugin returns mounts minus the record for (app, relPath).
func withoutMoodlePlugin(mounts []config.MoodlePluginMount, app, relPath string) []config.MoodlePluginMount {
	kept := make([]config.MoodlePluginMount, 0, len(mounts))
	for _, r := range mounts {
		if r.App != app || filepath.Clean(r.RelPath) != filepath.Clean(relPath) {
			kept = append(kept, r)
		}
	}
	return kept
}

// guardedRemoveScript removes the unit file $2 unless the folder $3 is
// still a mount point: a unit that failed to stop is kept, so the mount is
// not left active with nothing recording how to stop it.
const guardedRemoveScript = `if mountpoint -q -- "$3"; then printf '%s\n' "$3 continua montado — a unidade foi mantida." >&2; exit 1; fi; rm -f -- "$2"`

// enableOrRollbackScript enables and starts the unit $1; when that fails it
// stops and removes it again (unit file $2, mount folder $3), so a failed
// mount leaves nothing behind that would try to mount at boot.
const enableOrRollbackScript = `systemctl enable --now -- "$1" && exit 0; systemctl disable --now -- "$1"; ` +
	guardedRemoveScript + `; systemctl daemon-reload; exit 1`

// unmountSteps stops and disables unit and removes its file (unless dest is
// still mounted), then reloads systemd. Used by RemoveMoodlePlugin.
func unmountSteps(unit, dest string) []executor.PrivilegedStep {
	unitFile := filepath.Join(systemdUnitDir, unit)
	return []executor.PrivilegedStep{
		{Announce: "Desmontando " + dest + "...", Soft: true, WarnMessage: "A unidade " + unit + " já estava parada ou não existia.", Name: "systemctl", Args: []string{"disable", "--now", "--", unit}},
		{Name: "sh", Args: []string{"-c", guardedRemoveScript, "sh", unit, unitFile, dest}},
		{Name: "systemctl", Args: []string{"daemon-reload"}},
	}
}

// installMountUnit writes the mount unit through a root-owned staged copy
// and enables it, in one privileged batch. If enabling fails, the batch
// removes the unit again.
func installMountUnit(ctx context.Context, exe *executor.Executor, stdout io.Writer, plan MoodlePluginPlan) error {
	content := mountUnitContent(plan)
	tmp, err := os.CreateTemp("", "perci-mount-*.mount")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := tmp.WriteString(content); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	sum := sha256.Sum256([]byte(content))

	unit := mountUnitName(plan.Dest)
	unitFile := filepath.Join(systemdUnitDir, unit)
	steps, err := executor.PrivilegedInstallSteps(tmp.Name(), unitFile, hex.EncodeToString(sum[:]), "0644")
	if err != nil {
		return err
	}
	steps[0].Announce = "Instalando a unidade de montagem " + unit + "..."
	steps = append(steps,
		executor.PrivilegedStep{Name: "systemctl", Args: []string{"daemon-reload"}},
		executor.PrivilegedStep{Announce: "Montando " + plan.Dest + "...", Name: "sh", Args: []string{"-c", enableOrRollbackScript, "sh", unit, unitFile, plan.Dest}},
	)
	ui.Info(stdout, "Solicitando autenticação de administrador para montar o plugin...")
	if err := exe.RunSudoSequence(ctx, executor.Options{Stdout: stdout, Stderr: stdout}, steps); err != nil {
		return fmt.Errorf("montar o plugin: %w", err)
	}
	return nil
}

// mountUnitHeader is the first line of every mount unit perci installs.
const mountUnitHeader = "# Generated by perci.gnl: Dev Tools > Plugins Moodle."

// unitPath escapes a path for use as a systemd unit setting value, where
// '%' starts a specifier.
func unitPath(p string) string { return strings.ReplaceAll(p, "%", "%%") }

// mountUnitContent is the systemd mount unit binding plan.DevPath over
// plan.Dest, ordered before docker.service.
func mountUnitContent(plan MoodlePluginPlan) string {
	return mountUnitHeader + "\n" +
		"[Unit]\n" +
		"Description=perci: " + plan.Component + " no Moodle " + plan.App + "\n" +
		"RequiresMountsFor=" + unitPath(plan.DevPath) + "\n" +
		"Before=docker.service\n" +
		"\n" +
		"[Mount]\n" +
		"What=" + unitPath(plan.DevPath) + "\n" +
		"Where=" + unitPath(plan.Dest) + "\n" +
		"Type=none\n" +
		"Options=bind\n" +
		"\n" +
		"[Install]\n" +
		"WantedBy=multi-user.target\n"
}

// mountUnitName is the systemd unit name for a mount at path, escaped the
// way systemd-escape --path --suffix=mount does.
func mountUnitName(path string) string {
	path = strings.Trim(filepath.Clean(path), "/")
	if path == "" {
		return "-.mount"
	}
	var b strings.Builder
	for i := 0; i < len(path); i++ {
		c := path[i]
		switch {
		case c == '/':
			b.WriteByte('-')
		case c == '.' && i == 0:
			fmt.Fprintf(&b, `\x%02x`, c)
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == ':', c == '_', c == '.':
			b.WriteByte(c)
		default:
			fmt.Fprintf(&b, `\x%02x`, c)
		}
	}
	return b.String() + ".mount"
}

// findMoodlePlugin returns the record for (folder, relPath).
func findMoodlePlugin(mounts []config.MoodlePluginMount, folder, relPath string) (config.MoodlePluginMount, bool) {
	for _, m := range mounts {
		if m.App == folder && filepath.Clean(m.RelPath) == filepath.Clean(relPath) {
			return m, true
		}
	}
	return config.MoodlePluginMount{}, false
}

// pluginUnit is an installed mount unit of a plugin: its name and the
// folder it mounts over.
type pluginUnit struct {
	name  string
	where string
}

// pluginUnits lists the mount units that mount m: the perci-generated units
// in systemdUnitDir whose What= is m.DevPath and whose Where= is dest (or
// ends with <app>/<relPath>, should the workspace path have changed since),
// plus the unit named after dest itself. Looking at the installed units
// keeps the removal working when the unit's name can no longer be derived
// from the current paths.
func pluginUnits(m config.MoodlePluginMount, dest string) []pluginUnit {
	units := []pluginUnit{{name: mountUnitName(dest), where: dest}}
	entries, err := os.ReadDir(systemdUnitDir)
	if err != nil {
		return units
	}
	suffix := "/" + m.App + "/" + filepath.ToSlash(filepath.Clean(m.RelPath))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".mount") || e.Name() == units[0].name {
			continue
		}
		data, err := os.ReadFile(filepath.Join(systemdUnitDir, e.Name()))
		if err != nil || !strings.HasPrefix(string(data), mountUnitHeader) {
			continue
		}
		var what, where string
		for _, line := range strings.Split(string(data), "\n") {
			if v, ok := strings.CutPrefix(line, "What="); ok {
				what = strings.ReplaceAll(v, "%%", "%")
			}
			if v, ok := strings.CutPrefix(line, "Where="); ok {
				where = strings.ReplaceAll(v, "%%", "%")
			}
		}
		if what == m.DevPath && where != "" && (where == dest || strings.HasSuffix(where, suffix)) {
			units = append(units, pluginUnit{name: e.Name(), where: where})
		}
	}
	return units
}

// RemoveMoodlePlugin stops and removes the mount unit of a recorded mount
// (one password prompt), removes the destination folder only when it is
// left empty and unmounted, drops the record and restarts the containers
// so they stop seeing the plugin. A .megaignore rule perci added for the
// mount is removed after the folder is unmounted. A unit whose mount could
// not be stopped is kept, and the record with it. The development folder is
// never touched.
func RemoveMoodlePlugin(ctx context.Context, exe *executor.Executor, stdout io.Writer, folder, relPath string) error {
	stackMu.Lock()
	defer stackMu.Unlock()
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("carregar config: %w", err)
	}
	m, found := findMoodlePlugin(cfg.MoodlePlugins, folder, relPath)
	if !found {
		return fmt.Errorf("nenhum plugin montado em %s/%s", folder, relPath)
	}
	workspace, err := resolveWorkspace(cfg)
	if err != nil {
		return err
	}
	html := AppHTMLDir(workspace, folder)
	if resolved, err := filepath.EvalSymlinks(html); err == nil {
		html = resolved
	}
	// The recorded path comes from config.yaml: it is checked before it
	// reaches the privileged step.
	if !filepath.IsLocal(m.RelPath) {
		return fmt.Errorf("o registro do plugin aponta para fora do Moodle: %s", m.RelPath)
	}
	dest := filepath.Join(html, m.RelPath)
	if !resolvedWithin(html, dest) {
		return fmt.Errorf("o destino %s fica fora do Moodle %s", dest, folder)
	}

	ui.Info(stdout, "Solicitando autenticação de administrador para desmontar o plugin...")
	var steps []executor.PrivilegedStep
	for _, u := range pluginUnits(m, dest) {
		steps = append(steps, unmountSteps(u.name, u.where)...)
	}
	if err := exe.RunSudoSequence(ctx, executor.Options{Stdout: stdout, Stderr: stdout}, steps); err != nil {
		return fmt.Errorf("desmontar o plugin: %w", err)
	}

	if !exe.DryRun {
		if sameDir(m.DevPath, dest) {
			return fmt.Errorf("%s continua montado; nada foi apagado", dest)
		}
		if err := os.Remove(dest); err != nil && !errors.Is(err, fs.ErrNotExist) {
			ui.Warning(stdout, "A pasta "+dest+" não foi apagada (não está vazia): "+err.Error())
		}
	}
	if m.MegaIgnoreRule != "" {
		if err := removeMegaIgnoreRule(m.MegaIgnore, m.MegaIgnoreRule); err != nil {
			ui.Warning(stdout, "A regra "+m.MegaIgnoreRule+" não foi removida de "+m.MegaIgnore+": "+err.Error())
		}
	}
	if err := config.Update(func(c *config.Config) error {
		c.MoodlePlugins = withoutMoodlePlugin(c.MoodlePlugins, m.App, m.RelPath)
		return nil
	}); err != nil {
		return err
	}
	ui.Success(stdout, m.Component+" desmontado do Moodle "+folder+".")
	return restartForPlugins(ctx, exe, stdout, folder)
}

// ApplyMoodlePlugins restarts the Moodle app container (when running) and
// Nginx (when running), so both see the current plugin mounts.
func ApplyMoodlePlugins(ctx context.Context, exe *executor.Executor, stdout io.Writer, folder string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("carregar config: %w", err)
	}
	if app, found := FindAppByFolder(cfg.Docker.Apps, folder); !found || app.Type != config.AppTypeMoodle {
		return fmt.Errorf("%q não é um contêiner Moodle do Perci", folder)
	}
	return restartForPlugins(ctx, exe, stdout, folder)
}

func restartForPlugins(ctx context.Context, exe *executor.Executor, stdout io.Writer, folder string) error {
	if ContainerStatus(ctx, exe, folder) == "running" {
		ui.Info(stdout, "Reiniciando o contêiner "+folder+"...")
		if err := RestartContainer(ctx, exe, stdout, folder); err != nil {
			return err
		}
	} else {
		ui.Info(stdout, "O contêiner "+folder+" não está rodando: os plugins serão aplicados quando ele iniciar.")
	}
	if nginxContainerRunning(ctx, exe) {
		ui.Info(stdout, "Reiniciando o contêiner Nginx...")
		if err := RestartContainer(ctx, exe, stdout, NginxContainerName); err != nil {
			return err
		}
	}
	ui.Success(stdout, "Contêineres reiniciados.")
	return nil
}

// MoodlePluginState is a recorded mount with its current status and, when
// dev/version.php declares a different component, that component.
type MoodlePluginState struct {
	config.MoodlePluginMount
	Status     string
	DeclaredAs string
}

// MoodlePluginStatuses checks every recorded mount, sorted by component
// and then by app. The checks compare the device and inode of the
// development folder with those of the destination on the host, inside
// the app container and inside Nginx: a bind mount shows the same pair on
// every side, even for an empty folder. Each container is inspected once
// and asked once for all the folders it must hold.
func MoodlePluginStatuses(ctx context.Context, exe *executor.Executor, cfg *config.Config) []MoodlePluginState {
	if len(cfg.MoodlePlugins) == 0 {
		return nil
	}
	workspace, wsErr := resolveWorkspace(cfg)
	nginxUp := nginxContainerRunning(ctx, exe)

	// Per app: the container status and, for a running one, the ids of the
	// plugin folders as the container sees them.
	appStatus := map[string]string{}
	appPaths := map[string][]string{}
	nginxPaths := []string{}
	for _, m := range cfg.MoodlePlugins {
		if _, seen := appStatus[m.App]; !seen {
			appStatus[m.App] = ContainerStatus(ctx, exe, m.App)
		}
		if appStatus[m.App] == "running" {
			appPaths[m.App] = append(appPaths[m.App], "/var/www/html/"+m.RelPath)
		}
		nginxPaths = append(nginxPaths, appHTMLMount+"/"+m.App+"/"+m.RelPath)
	}
	appIDs := map[string]map[string]string{}
	for app, paths := range appPaths {
		appIDs[app] = containerDirIDs(ctx, exe, app, paths)
	}
	var nginxIDs map[string]string
	if nginxUp {
		nginxIDs = containerDirIDs(ctx, exe, NginxContainerName, nginxPaths)
	}

	states := make([]MoodlePluginState, 0, len(cfg.MoodlePlugins))
	for _, m := range cfg.MoodlePlugins {
		st := MoodlePluginState{MoodlePluginMount: m}
		if t, n := DevPluginComponent(m.DevPath); t != "" && t+"_"+n != m.Component {
			st.DeclaredAs = t + "_" + n
		}
		st.Status = pluginStatus(workspace, wsErr, m, appStatus[m.App], appIDs[m.App], nginxUp, nginxIDs)
		states = append(states, st)
	}
	sort.SliceStable(states, func(i, j int) bool {
		if states[i].Component != states[j].Component {
			return states[i].Component < states[j].Component
		}
		return states[i].App < states[j].App
	})
	return states
}

// pluginStatus classifies one mount from the host's view of it, the status
// of its app container, and the folder ids the app container and Nginx
// reported.
func pluginStatus(workspace string, wsErr error, m config.MoodlePluginMount, appStatus string, appIDs map[string]string, nginxUp bool, nginxIDs map[string]string) string {
	devID, ok := dirID(m.DevPath)
	if wsErr != nil || !ok {
		return PluginInactive
	}
	hostID, ok := dirID(filepath.Join(AppHTMLDir(workspace, m.App), m.RelPath))
	if !ok || hostID != devID {
		return PluginInactive
	}
	switch appStatus {
	case "":
		return PluginNoApp
	case "running":
	default:
		return PluginStopped
	}
	if appIDs["/var/www/html/"+m.RelPath] != devID {
		return PluginPending
	}
	if nginxUp && nginxIDs[appHTMLMount+"/"+m.App+"/"+m.RelPath] != devID {
		return PluginPending
	}
	return PluginApplied
}

// dirID returns "device:inode" of an existing folder.
func dirID(path string) (string, bool) {
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return "", false
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return "", false
	}
	return fmt.Sprintf("%d:%d", st.Dev, st.Ino), true
}

// containerDirIDs returns "device:inode" for each of paths inside a running
// container, from one `docker exec ... stat`. A path it cannot read is
// absent from the result.
func containerDirIDs(ctx context.Context, exe *executor.Executor, container string, paths []string) map[string]string {
	ids := map[string]string{}
	if len(paths) == 0 {
		return ids
	}
	// stat exits non-zero when any path is missing but still prints the
	// others; the script swallows that status so the output is kept.
	args := append([]string{"exec", container, "sh", "-c", `stat -c '%n %d:%i' -- "$@" 2>/dev/null || true`, "sh"}, paths...)
	out, _ := exe.Output(ctx, executor.Options{}, "docker", args...)
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if i := strings.LastIndex(line, " "); i > 0 {
			ids[line[:i]] = line[i+1:]
		}
	}
	return ids
}

// sameDir reports whether a and b are the same existing folder.
func sameDir(a, b string) bool {
	ida, ok := dirID(a)
	if !ok {
		return false
	}
	idb, ok := dirID(b)
	return ok && ida == idb
}

// resolveExistingDir returns dir's absolute path with links resolved, or
// an error when it is not an existing folder.
func resolveExistingDir(dir string) (string, error) {
	if !filepath.IsAbs(dir) {
		return "", fmt.Errorf("caminho inválido: %q", dir)
	}
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return "", fmt.Errorf("pasta não encontrada: %s", dir)
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("não é uma pasta: %s", dir)
	}
	return resolved, nil
}

// resolvedWithin resolves the deepest existing ancestor of path, following
// links, and reports whether the final location stays under root. An
// ancestor that exists but cannot be resolved (a dangling link) counts as
// outside.
func resolvedWithin(root, path string) bool {
	if resolvedRoot, err := filepath.EvalSymlinks(root); err == nil {
		root = resolvedRoot
	}
	for p := path; ; p = filepath.Dir(p) {
		if _, err := os.Lstat(p); err == nil {
			real, err := filepath.EvalSymlinks(p)
			if err != nil {
				return false
			}
			rest, err := filepath.Rel(p, path)
			if err != nil {
				return false
			}
			return within(root, filepath.Join(real, rest))
		}
		if filepath.Dir(p) == p {
			return false
		}
	}
}

// within reports whether path is root or below it.
func within(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
