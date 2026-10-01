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

package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"

	"github.com/oito2/perci/internal/fsutil"
)

const (
	configDir  = ".perci"
	configFile = "config.yaml"
)

// Config holds all user-configurable settings for perci.gnl.
type Config struct {
	WorkspacePath string       `yaml:"workspace_path"`
	FlatpakScope  string       `yaml:"flatpak_scope,omitempty"`
	Docker        DockerConfig `yaml:"docker,omitempty"`

	// GUITheme is the daisyUI theme name active in the GUI (cmd/prci-gui),
	// one of GUIThemes(). Empty means "not set yet" — GUIThemeOrDefault
	// fills in the default.
	GUITheme string `yaml:"gui_theme,omitempty"`

	// SidebarLogo and AppIcon (Home :: Settings, decided with the user on
	// 2026-09-14) pick which mascot color variant ("blue"/"pink",
	// see SidebarLogos()/AppIcons() below) the GUI shows in its sidebar and
	// as its window/taskbar icon, respectively — kept as two separate
	// fields (not one shared "app color") since the user may want a
	// different combination between the two.
	SidebarLogo string `yaml:"sidebar_logo,omitempty"`
	AppIcon     string `yaml:"app_icon,omitempty"`

	// RepoFolders (Dev Tools :: Repositórios, decided with the user on
	// 2026-09-14) remembers, across sessions, the working folders already
	// used on that screen — shown as quick-selection cards instead of
	// requiring the folder to be chosen again every time. The name shown on
	// each card is always the folder's own name (the path's last segment,
	// derived on the fly by DevToolsService.GetRepoFolders) — never stored
	// separately. Entries whose path has disappeared from disk are removed
	// by PruneMissingRepoFolders, called every time this list is read.
	RepoFolders []string `yaml:"repo_folders,omitempty"`

	// AgentSkills tracks skills.sh skills installed via "Dev Tools :: IA:
	// SKILLs" (GUI-only feature, added 2026-09-11) — config.yaml is the
	// source of truth for what's installed, same principle as Docker.Apps
	// above: `npx skills list` has no documented machine-readable output to
	// derive this from instead, and this doesn't try to parse its
	// human-oriented text.
	AgentSkills []AgentSkillInstall `yaml:"agent_skills,omitempty"`

	// MCPServers tracks MCP servers registered via "Dev Tools :: IA: MCPs"
	// (GUI-only feature, added 2026-09-11) — same principle as AgentSkills:
	// each agent (Claude Code/Codex/Antigravity) has its own way of
	// registering a server, with no unified "is it installed?" query to
	// derive this back from, so config.yaml is the source of truth.
	MCPServers []MCPServerInstall `yaml:"mcp_servers,omitempty"`

	// TrayEnabled turns on the system tray icon (Home :: Settings, decided
	// with the user on 2026-09-15) — opt-in, off by default. No
	// *OrDefault() helper: unlike the enum-like fields above (GUITheme/
	// SidebarLogo/AppIcon), the zero-value bool (false) already IS the
	// desired default behavior, so there's no "never configured" vs.
	// "turned off" ambiguity to resolve.
	TrayEnabled bool `yaml:"tray_enabled,omitempty"`
}

// AgentSkillInstall records one (skill, scope) pair. Global installs are
// one global set (Folder empty); Local installs are scoped per project
// folder — the same skill can be installed in some project folders and
// not others, so Folder is part of the key alongside Slug.
type AgentSkillInstall struct {
	Slug   string `yaml:"slug"`
	Global bool   `yaml:"global"`
	Folder string `yaml:"folder,omitempty"`
}

// MCPServerInstall records one (mcp server, scope) pair — same convention
// as AgentSkillInstall. Param is the extra value (a folder or a file path)
// some servers need at registration time (ex. Filesystem's allowed
// directory, SQLite's --db-path) — persisted so "Atualizar" (remove +
// re-register — no native update primitive for a plain MCP registration)
// can reapply the exact same value without asking again.
type MCPServerInstall struct {
	Slug   string `yaml:"slug"`
	Global bool   `yaml:"global"`
	Folder string `yaml:"folder,omitempty"`
	Param  string `yaml:"param,omitempty"`
}

// GUIThemes is the fixed set of daisyUI themes the GUI offers, decided
// with the user on 2026-09-10 — a curated subset of daisyUI's 35
// built-in themes, not the full catalog.
func GUIThemes() []string {
	return []string{"light", "dark", "cupcake", "synthwave", "retro", "valentine", "halloween", "garden"}
}

// DefaultGUITheme is used whenever GUITheme is unset or not one of
// GUIThemes() (e.g. an old config.yaml, or a name from a future release
// this binary doesn't know yet).
const DefaultGUITheme = "dark"

// firstMatchOrDefault returns value when it's one of allowed, def otherwise
// — shared by GUIThemeOrDefault/SidebarLogoOrDefault/AppIconOrDefault
// below, instead of each duplicating this same "is it in the fixed list?"
// loop.
func firstMatchOrDefault(value string, allowed []string, def string) string {
	for _, v := range allowed {
		if value == v {
			return value
		}
	}
	return def
}

// GUIThemeOrDefault returns cfg.GUITheme when it is one of GUIThemes(),
// DefaultGUITheme otherwise.
func (c *Config) GUIThemeOrDefault() string {
	return firstMatchOrDefault(c.GUITheme, GUIThemes(), DefaultGUITheme)
}

// SidebarLogos is the fixed set of mascot logo variants the GUI offers for
// its sidebar (Home :: Settings, decided with the user on 2026-09-14) —
// the PNGs backing each name live in cmd/prci-gui/frontend/dist/assets/ as
// perci-<name>.png.
func SidebarLogos() []string {
	return []string{"blue", "pink"}
}

// DefaultSidebarLogo matches the mascot color the GUI always showed before
// this option existed (frontend/dist/assets/iamperci.png, retired in favor
// of perci-blue.png — a transparent, higher-resolution version of the same
// character), so nobody's sidebar changes look until they pick "pink".
const DefaultSidebarLogo = "blue"

// SidebarLogoOrDefault returns cfg.SidebarLogo when it is one of
// SidebarLogos(), DefaultSidebarLogo otherwise.
func (c *Config) SidebarLogoOrDefault() string {
	return firstMatchOrDefault(c.SidebarLogo, SidebarLogos(), DefaultSidebarLogo)
}

// AppIcons is the fixed set of window/taskbar icon variants the GUI offers
// (Home :: Settings, decided with the user on 2026-09-14) — the .png
// files backing each name live in cmd/prci-gui/frontend/dist/assets/ as
// perci-<name>.png (same PNGs the sidebar-logo picker uses), embedded into
// the binary itself (cmd/prci-gui/main.go) since Wails has no runtime API
// to change a window's icon after creation — a value saved here only
// takes effect the next time Perci is started, never live.
func AppIcons() []string {
	return []string{"blue", "pink"}
}

// DefaultAppIcon matches the SidebarLogo default above, for the same "no
// existing install's look changes on upgrade" reasoning — the GUI ran with
// no custom window icon at all before this option existed, so "blue" (the
// default sidebar color too) is as neutral a first choice as any.
const DefaultAppIcon = "blue"

// AppIconOrDefault returns cfg.AppIcon when it is one of AppIcons(),
// DefaultAppIcon otherwise.
func (c *Config) AppIconOrDefault() string {
	return firstMatchOrDefault(c.AppIcon, AppIcons(), DefaultAppIcon)
}

// App type values for AppContainer.Type. "generic" ("Servidor Genérico")
// is deliberately identical to "php" ("Aplicação PHP") in
// every way the appstack package behaves — it exists only as a separate
// catalog label for the user.
//
// AppTypeNode and AppTypePHPNode are unrelated to the PHP-only types
// above: AppTypeNode is a standalone Node/JS container (Vue,
// React, Svelte, a plain Node API, ... — perci is agnostic to the framework,
// it only runs whatever DevCommand says), and AppTypePHPNode is a single
// multi-process container pairing a PHP api/ with a Node app/ — see
// AppContainer's NodeVersion/DevCommand/DevPort fields, which only these two
// types use.
const (
	AppTypeMoodle  = "moodle"
	AppTypePHP     = "php"
	AppTypeGeneric = "generic"
	AppTypeNode    = "node"
	AppTypePHPNode = "php_node"
)

// DockerConfig holds settings for the per-project "Container Aplicativo"
// Docker stack (internal/appstack). It replaced the legacy shared stack.
type DockerConfig struct {
	NginxCreated bool           `yaml:"nginx_created,omitempty"`
	MariaDB      MariaDBConfig  `yaml:"mariadb,omitempty"`
	Apps         []AppContainer `yaml:"apps,omitempty"`
}

// MariaDBConfig holds the shared MariaDB container's credentials for the
// per-project Docker stack. Every AppContainer with DBAccess=true connects
// using these same credentials (network-only access, no per-project
// database or user).
//
// DataUser/DataPass record the credentials MariaDB's data directory was
// first initialized with — the image only applies MYSQL_USER/
// MYSQL_PASSWORD/MYSQL_ROOT_PASSWORD on an empty data directory, so these
// (and DBRootPass) are kept even after the container is removed from the
// stack, to refuse re-creating it over the same data with credentials the
// database doesn't know.
type MariaDBConfig struct {
	DBUser     string `yaml:"db_user,omitempty"`
	DBPass     string `yaml:"db_pass,omitempty"`
	DBRootPass string `yaml:"db_root_pass,omitempty"`
	DataUser   string `yaml:"data_user,omitempty"`
	DataPass   string `yaml:"data_pass,omitempty"`
}

// AppContainer describes one "Container Aplicativo": a single project's own
// container, folder, PHP version and *.localhost URL. config.yaml is the
// source of truth for the list of containers — it is not derived from
// `docker ps`/labels.
type AppContainer struct {
	Name       string `yaml:"name"`        // display label ("Nome do aplicativo")
	Folder     string `yaml:"folder"`      // folder name = container name (technical); validated by appstack.ValidAppFolder
	Type       string `yaml:"type"`        // AppTypeMoodle | AppTypePHP | AppTypeGeneric | AppTypeNode | AppTypePHPNode
	URL        string `yaml:"url"`         // "<label>.localhost"; validated by appstack.ValidAppURL
	PHPVersion string `yaml:"php_version"` // "7.4".."8.4"; unused (empty) by AppTypeNode
	DBAccess   bool   `yaml:"db_access"`

	// MoodleVersion is one of appstack.MoodleVersions() ("3.x", "4.x",
	// "5.0", "5.1+") — used only by AppTypeMoodle, empty for every other
	// type. It has to be its own persisted field rather than derived from
	// PHPVersion because "5.0" and "5.1+" share the same PHP range (8.2,
	// 8.3, 8.4 — Moodle 5.0 raised its own floor to match 5.1+'s,
	// moodledev.io/general/releases/5.0, confirmed 2026-08-24) yet need
	// different Nginx routing recipes (appstack/nginx.go) — PHPVersion
	// alone can't tell them apart. Empty is treated as "5.1+"
	// (appstack.buildAppServerBlock) so pre-existing Moodle entries from
	// before this field existed keep rendering the same Nginx recipe they
	// always did.
	MoodleVersion string `yaml:"moodle_version,omitempty"`

	// NodeVersion, DevCommand and DevPort are used only by AppTypeNode and
	// AppTypePHPNode — empty/zero for every other type, kept out of
	// config.yaml by omitempty.
	NodeVersion string `yaml:"node_version,omitempty"` // e.g. "22", "24", "26"; validated by appstack.ValidNodeVersion
	DevCommand  string `yaml:"dev_command,omitempty"`  // e.g. "npm run dev" — free text, run by supervisord (php_node) or as the container's own CMD (node); see appstack's doc comments for why this isn't regex-validated
	DevPort     int    `yaml:"dev_port,omitempty"`     // e.g. 5173 — the dev server's port, proxy_pass'd by Nginx; validated by appstack.ValidDevPort

	// PHPMemoryLimit overrides the PHP memory_limit baked into the shared
	// perci-php<version>/perci-php<version>-node<version> image
	// (appstack.DefaultPHPMemoryLimit, "512M") for this app specifically.
	// It's written to a per-app php.ini snippet bind-mounted into the
	// container's own /usr/local/etc/php/conf.d/ (appstack.
	// WritePHPMemoryLimitConf) — never edited in the shared image or in a
	// running container's writable layer — so it survives Recriar/remove+
	// recreate, unlike a manual edit inside the container. Empty means "use
	// the image's own default". Used by every PHP-family type (Moodle, PHP,
	// Generic, PHPNode); unused (empty) by AppTypeNode. Validated by
	// appstack.ValidPHPMemoryLimit.
	PHPMemoryLimit string `yaml:"php_memory_limit,omitempty"`

	// WorkerCommand is an optional extra background process — a queue/job
	// consumer (e.g. Symfony Messenger, a Laravel queue worker) — that runs
	// alongside php-fpm and the Node dev server inside an AppTypePHPNode
	// container, supervisord-managed (appstack's [program:worker]). Empty
	// means no extra process runs. Used only by AppTypePHPNode. Same
	// free-text philosophy as DevCommand (see its own comment above) —
	// never regex-validated, only ever passed through the container's own
	// environment/shell, never interpolated into a host-side command.
	WorkerCommand string `yaml:"worker_command,omitempty"`
}

// Load reads ~/.perci/config.yaml and returns its content — an empty
// Config, without error, when the file doesn't exist yet. No field is
// defaulted here (see Workspace).
func Load() (*Config, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return &cfg, nil
}

// save writes cfg to ~/.perci/config.yaml atomically, creating the directory if needed.
//
// The file holds credentials (the MariaDB root/user passwords), so it is
// 0600 and ~/.perci itself is kept 0700 — also tightened when it already
// exists with the 0755 older versions created. The data is synced to disk
// before the rename (fsutil.WriteFileAtomic): a power loss right after it must not leave an empty
// config.yaml, which would lose the generated db_root_pass for good.
func save(cfg *Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return fmt.Errorf("chmod config dir: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := fsutil.WriteFileAtomic(path, data, 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

// mu serializes every Load-mutate-save cycle against config.yaml. Each
// bindable GUI method runs on its own goroutine (cmd/prci-gui's service_*.go
// files), so two
// concurrent Set*/Create*/Delete* calls doing their own Load()+save() could
// race — the one that saves last silently wins, dropping the other's change
// (save itself is atomic at the file level, but that doesn't protect
// against this in-memory lost-update race). Update below is the fix: every
// caller that used to do Load()+mutate+Save() should go through it instead.
var mu sync.Mutex

// Update loads the config, applies mutate to it, and saves the result, all
// under the same lock — use this instead of a manual Load()+save() pair
// whenever the operation is "read the config, change one thing, write it
// back" (see mu's doc comment for why).
func Update(mutate func(cfg *Config) error) error {
	mu.Lock()
	defer mu.Unlock()

	cfg, err := Load()
	if err != nil {
		return err
	}
	if err := mutate(cfg); err != nil {
		return err
	}
	return save(cfg)
}

// PruneMissingRepoFolders drops every RepoFolders entry whose path no
// longer exists (or isn't a directory anymore): the list used to only grow,
// with no way for a deleted/moved folder's card to ever disappear from
// "Dev Tools :: Repositórios" on its own. Safe to call often (idempotent,
// cheap os.Stat per entry) — called once whenever that screen's folder
// list is fetched.
func PruneMissingRepoFolders() error {
	return Update(func(cfg *Config) error {
		kept := make([]string, 0, len(cfg.RepoFolders))
		for _, p := range cfg.RepoFolders {
			if info, err := os.Stat(p); err == nil && info.IsDir() {
				kept = append(kept, p)
			}
		}
		cfg.RepoFolders = kept
		return nil
	})
}

// FlatpakFlag returns the --system or --user scope flag based on the config.
// Falls back to --system when the config cannot be loaded or the scope is unset.
func FlatpakFlag() string {
	cfg, err := Load()
	if err != nil {
		// Config unreadable (corrupt file, permission denied, ...) — falls
		// back to --system same as an unset scope, but this is a fallback
		// on error, not the user's actual choice.
		return "--system"
	}
	if cfg.FlatpakScope == "user" {
		return "--user"
	}
	return "--system"
}

// ExpandPath resolves a leading ~ or ~/ against the user's home directory.
func ExpandPath(path string) (string, error) {
	if path != "~" && !strings.HasPrefix(path, "~/") {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	if path == "~" {
		return home, nil
	}
	return filepath.Join(home, path[2:]), nil
}

func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	return filepath.Join(home, configDir, configFile), nil
}

// Workspace is the workspace folder: WorkspacePath, or ~/workspace when it
// isn't set. Load never fills WorkspacePath in — "" means the user hasn't
// chosen one (the Configurações screen warns, and Importar configurações
// adopts the exported one) — so everything that needs the actual folder
// resolves it here.
func (c *Config) Workspace() (string, error) {
	if c.WorkspacePath != "" {
		return c.WorkspacePath, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("obter diretório home: %w", err)
	}
	return filepath.Join(home, "workspace"), nil
}
