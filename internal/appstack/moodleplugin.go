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
	dest := filepath.Join(html, relPath)
	if within(html, devPath) || within(devPath, html) {
		return MoodlePluginPlan{}, fmt.Errorf("a pasta de desenvolvimento não pode ficar dentro do Moodle %s nem conter o Moodle", folder)
	}
	for _, p := range []string{devPath, dest} {
		if strings.ContainsFunc(p, func(r rune) bool { return r <= ' ' || r == 0x7f }) {
			return MoodlePluginPlan{}, fmt.Errorf("caminho com espaço ou caractere de controle não é suportado: %q", p)
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
		App: folder, Component: ptype + "_" + name, DevPath: devPath, Dest: dest, RelPath: relPath,
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

// AddMoodlePlugin creates the empty destination folder, installs and
// starts the mount unit (one password prompt) and records the mount.
// Containers pick it up on their next restart.
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

	_, statErr := os.Lstat(plan.Dest)
	created := errors.Is(statErr, fs.ErrNotExist)
	if err := os.MkdirAll(plan.Dest, 0o755); err != nil {
		return fmt.Errorf("criar %s: %w", plan.Dest, err)
	}
	if err := installMountUnit(ctx, exe, stdout, plan); err != nil {
		if created {
			_ = os.Remove(plan.Dest)
		}
		return err
	}
	if err := config.Update(func(c *config.Config) error {
		c.MoodlePlugins = append(c.MoodlePlugins, config.MoodlePluginMount{
			App: plan.App, Component: plan.Component, DevPath: plan.DevPath, RelPath: plan.RelPath,
		})
		return nil
	}); err != nil {
		return err
	}
	ui.Success(stdout, fmt.Sprintf("%s montado em %s. Reinicie o contêiner para aplicar.", plan.Component, plan.Dest))
	return nil
}

// installMountUnit writes the mount unit through a root-owned staged copy
// and enables it, in one privileged batch.
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
	steps, err := executor.PrivilegedInstallSteps(tmp.Name(), filepath.Join(systemdUnitDir, unit), hex.EncodeToString(sum[:]), "0644")
	if err != nil {
		return err
	}
	steps[0].Announce = "Instalando a unidade de montagem " + unit + "..."
	steps = append(steps,
		executor.PrivilegedStep{Name: "systemctl", Args: []string{"daemon-reload"}},
		executor.PrivilegedStep{Announce: "Montando " + plan.Dest + "...", Name: "systemctl", Args: []string{"enable", "--now", "--", unit}},
	)
	ui.Info(stdout, "Solicitando autenticação de administrador para montar o plugin...")
	if err := exe.RunSudoSequence(ctx, executor.Options{Stdout: stdout, Stderr: stdout}, steps); err != nil {
		return fmt.Errorf("montar o plugin: %w", err)
	}
	return nil
}

// mountUnitContent is the systemd mount unit binding plan.DevPath over
// plan.Dest, ordered before docker.service.
func mountUnitContent(plan MoodlePluginPlan) string {
	return "# Generated by perci.gnl: Dev Tools > Plugins Moodle.\n" +
		"[Unit]\n" +
		"Description=perci: " + plan.Component + " no Moodle " + plan.App + "\n" +
		"RequiresMountsFor=" + plan.DevPath + "\n" +
		"Before=docker.service\n" +
		"\n" +
		"[Mount]\n" +
		"What=" + plan.DevPath + "\n" +
		"Where=" + plan.Dest + "\n" +
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

// RemoveMoodlePlugin stops and removes the mount unit of a recorded mount
// (one password prompt), removes the destination folder only when it is
// left empty and unmounted, drops the record and restarts the containers
// so they stop seeing the plugin. The development folder is never touched.
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
	dest := filepath.Join(AppHTMLDir(workspace, folder), m.RelPath)
	if html, err := filepath.EvalSymlinks(AppHTMLDir(workspace, folder)); err == nil {
		dest = filepath.Join(html, m.RelPath)
	}
	unit := mountUnitName(dest)

	ui.Info(stdout, "Solicitando autenticação de administrador para desmontar o plugin...")
	steps := []executor.PrivilegedStep{
		{Announce: "Desmontando " + dest + "...", Soft: true, WarnMessage: "A unidade " + unit + " já estava parada ou não existia.", Name: "systemctl", Args: []string{"disable", "--now", "--", unit}},
		{Name: "rm", Args: []string{"-f", "--", filepath.Join(systemdUnitDir, unit)}},
		{Name: "systemctl", Args: []string{"daemon-reload"}},
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
	if err := config.Update(func(c *config.Config) error {
		kept := c.MoodlePlugins[:0]
		for _, r := range c.MoodlePlugins {
			if r.App != m.App || filepath.Clean(r.RelPath) != filepath.Clean(m.RelPath) {
				kept = append(kept, r)
			}
		}
		c.MoodlePlugins = kept
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
// every side, even for an empty folder.
func MoodlePluginStatuses(ctx context.Context, exe *executor.Executor, cfg *config.Config) []MoodlePluginState {
	if len(cfg.MoodlePlugins) == 0 {
		return nil
	}
	workspace, wsErr := resolveWorkspace(cfg)
	nginxUp := nginxContainerRunning(ctx, exe)
	states := make([]MoodlePluginState, 0, len(cfg.MoodlePlugins))
	for _, m := range cfg.MoodlePlugins {
		st := MoodlePluginState{MoodlePluginMount: m}
		if t, n := DevPluginComponent(m.DevPath); t != "" && t+"_"+n != m.Component {
			st.DeclaredAs = t + "_" + n
		}
		st.Status = pluginStatus(ctx, exe, workspace, wsErr, m, nginxUp)
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

func pluginStatus(ctx context.Context, exe *executor.Executor, workspace string, wsErr error, m config.MoodlePluginMount, nginxUp bool) string {
	devID, ok := dirID(m.DevPath)
	if wsErr != nil || !ok {
		return PluginInactive
	}
	hostID, ok := dirID(filepath.Join(AppHTMLDir(workspace, m.App), m.RelPath))
	if !ok || hostID != devID {
		return PluginInactive
	}
	switch ContainerStatus(ctx, exe, m.App) {
	case "":
		return PluginNoApp
	case "running":
	default:
		return PluginStopped
	}
	if containerDirID(ctx, exe, m.App, "/var/www/html/"+m.RelPath) != devID {
		return PluginPending
	}
	if nginxUp && containerDirID(ctx, exe, NginxContainerName, appHTMLMount+"/"+m.App+"/"+m.RelPath) != devID {
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

// containerDirID returns "device:inode" of path inside a running
// container, or "" when it can't be read.
func containerDirID(ctx context.Context, exe *executor.Executor, container, path string) string {
	out, err := exe.Output(ctx, executor.Options{}, "docker", "exec", container, "stat", "-c", "%d:%i", "--", path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
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

// within reports whether path is root or below it.
func within(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
