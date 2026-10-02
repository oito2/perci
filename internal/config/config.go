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

	// SidebarLogo and AppIcon pick the mascot color variant ("blue"/"pink",
	// see SidebarLogos()/AppIcons()) shown as the sidebar logo and as the
	// window/taskbar icon, respectively. They are independent fields.
	SidebarLogo string `yaml:"sidebar_logo,omitempty"`
	AppIcon     string `yaml:"app_icon,omitempty"`

	// RepoFolders remembers the working folders already used on the
	// repositories screen. Only paths are stored; the displayed name is the
	// path's last segment. Entries whose path no longer exists are removed by
	// PruneMissingRepoFolders.
	RepoFolders []string `yaml:"repo_folders,omitempty"`

	// AgentSkills tracks the skills.sh skills installed through the GUI.
	// config.yaml is the source of truth for what is installed; nothing is
	// derived from the skills CLI output.
	AgentSkills []AgentSkillInstall `yaml:"agent_skills,omitempty"`

	// MCPServers tracks the MCP servers registered through the GUI.
	// config.yaml is the source of truth for what is registered.
	MCPServers []MCPServerInstall `yaml:"mcp_servers,omitempty"`

	// TrayEnabled turns on the system tray icon. Off by default; the zero
	// value is the default, so there is no OrDefault helper.
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

// MCPServerInstall records one (mcp server, scope) pair, with the same
// convention as AgentSkillInstall. Param is the extra value (a folder or a
// file path) some servers need at registration time, persisted so an
// update can re-register with the same value.
type MCPServerInstall struct {
	Slug   string `yaml:"slug"`
	Global bool   `yaml:"global"`
	Folder string `yaml:"folder,omitempty"`
	Param  string `yaml:"param,omitempty"`
}

// GUIThemes is the fixed set of daisyUI themes the GUI offers.
func GUIThemes() []string {
	return []string{"light", "dark", "cupcake", "synthwave", "retro", "valentine", "halloween", "garden"}
}

// DefaultGUITheme is used whenever GUITheme is unset or not one of
// GUIThemes() (e.g. an old config.yaml, or a name from a future release
// this binary doesn't know yet).
const DefaultGUITheme = "dark"

// firstMatchOrDefault returns value when it is one of allowed, def
// otherwise.
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
// its sidebar.
func SidebarLogos() []string {
	return []string{"blue", "pink"}
}

// DefaultSidebarLogo is used when SidebarLogo is unset or not one of
// SidebarLogos().
const DefaultSidebarLogo = "blue"

// SidebarLogoOrDefault returns cfg.SidebarLogo when it is one of
// SidebarLogos(), DefaultSidebarLogo otherwise.
func (c *Config) SidebarLogoOrDefault() string {
	return firstMatchOrDefault(c.SidebarLogo, SidebarLogos(), DefaultSidebarLogo)
}

// AppIcons is the fixed set of window/taskbar icon variants the GUI
// offers. The window icon is set at creation, so a saved value takes
// effect only the next time the app starts.
func AppIcons() []string {
	return []string{"blue", "pink"}
}

// DefaultAppIcon is used when AppIcon is unset or not one of AppIcons().
const DefaultAppIcon = "blue"

// AppIconOrDefault returns cfg.AppIcon when it is one of AppIcons(),
// DefaultAppIcon otherwise.
func (c *Config) AppIconOrDefault() string {
	return firstMatchOrDefault(c.AppIcon, AppIcons(), DefaultAppIcon)
}

// App type values for AppContainer.Type. "generic" behaves exactly like
// "php" in the appstack package; it differs only as a catalog label.
// AppTypeNode is a standalone Node/JS container that runs whatever
// DevCommand says, regardless of framework. AppTypePHPNode is a single
// multi-process container pairing a PHP api/ with a Node app/. Only these
// two types use the NodeVersion/DevCommand/DevPort fields.
const (
	AppTypeMoodle  = "moodle"
	AppTypePHP     = "php"
	AppTypeGeneric = "generic"
	AppTypeNode    = "node"
	AppTypePHPNode = "php_node"
)

// DockerConfig holds settings for the per-project "Container Aplicativo"
// Docker stack (internal/appstack).
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
// first initialized with. They (and DBRootPass) are kept after the
// container is removed from the stack, so re-creating it over the same
// data with credentials the database doesn't know is refused.
type MariaDBConfig struct {
	DBUser     string `yaml:"db_user,omitempty"`
	DBPass     string `yaml:"db_pass,omitempty"`
	DBRootPass string `yaml:"db_root_pass,omitempty"`
	DataUser   string `yaml:"data_user,omitempty"`
	DataPass   string `yaml:"data_pass,omitempty"`
}

// AppContainer describes one "Container Aplicativo": a single project's
// own container, folder, PHP version and *.localhost URL. config.yaml is
// the source of truth for the list of containers.
type AppContainer struct {
	Name       string `yaml:"name"`        // display label ("Nome do aplicativo")
	Folder     string `yaml:"folder"`      // folder name = container name (technical); validated by appstack.ValidAppFolder
	Type       string `yaml:"type"`        // AppTypeMoodle | AppTypePHP | AppTypeGeneric | AppTypeNode | AppTypePHPNode
	URL        string `yaml:"url"`         // "<label>.localhost"; validated by appstack.ValidAppURL
	PHPVersion string `yaml:"php_version"` // "7.4".."8.4"; unused (empty) by AppTypeNode
	DBAccess   bool   `yaml:"db_access"`

	// MoodleVersion is one of appstack.MoodleVersions() ("3.x", "4.1",
	// "4.2-4.3", "4.4-4.5", "5.0", "5.1+") or "4.x" for containers saved
	// before the 4.x split. Used only by AppTypeMoodle, empty for every other
	// type. It is persisted separately from PHPVersion because the PHP ranges
	// overlap while the Nginx routing recipes differ. Empty is treated as
	// "5.1+".
	MoodleVersion string `yaml:"moodle_version,omitempty"`

	// NodeVersion, DevCommand and DevPort are used only by AppTypeNode and
	// AppTypePHPNode — empty/zero for every other type, kept out of
	// config.yaml by omitempty.
	NodeVersion string `yaml:"node_version,omitempty"` // e.g. "22", "24", "26"; validated by appstack.ValidNodeVersion
	DevCommand  string `yaml:"dev_command,omitempty"`  // e.g. "npm run dev" — free text, run by supervisord (php_node) or as the container's own CMD (node)
	DevPort     int    `yaml:"dev_port,omitempty"`     // e.g. 5173 — the dev server's port, proxy_pass'd by Nginx; validated by appstack.ValidDevPort

	// PHPMemoryLimit overrides the PHP memory_limit of the shared image
	// (appstack.DefaultPHPMemoryLimit, "512M") for this app. It is written to
	// a per-app php.ini snippet bind-mounted into the container's
	// /usr/local/etc/php/conf.d/, so it survives container recreation. Empty
	// means use the image default. Used by every PHP-family type; unused
	// (empty) by AppTypeNode. Validated by appstack.ValidPHPMemoryLimit.
	PHPMemoryLimit string `yaml:"php_memory_limit,omitempty"`

	// WorkerCommand is an optional extra background process (a queue/job
	// consumer) that runs alongside php-fpm and the Node dev server inside an
	// AppTypePHPNode container, managed by supervisord. Empty means no extra
	// process. Used only by AppTypePHPNode. Free text like DevCommand: never
	// regex-validated, passed only through the container's environment/shell,
	// never interpolated into a host-side command.
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
// 0600 and ~/.perci is kept 0700, also tightened when it already exists
// with 0755. The data is synced to disk before the rename
// (fsutil.WriteFileAtomic), so a power loss cannot leave an empty
// config.yaml.
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

// mu serializes every Load-mutate-save cycle against config.yaml, so
// concurrent callers (each bindable GUI method runs on its own goroutine)
// cannot lose each other's updates.
var mu sync.Mutex

// Update loads the config, applies mutate to it, and saves the result,
// all under mu. Use it for any read-modify-write of the config.
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
// longer exists or isn't a directory. Idempotent, with one os.Stat per
// entry.
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
		// Config unreadable (corrupt file, permission denied, ...): falls
		// back to --system, same as an unset scope.
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

// Workspace is the workspace folder: WorkspacePath, or ~/workspace when
// it isn't set. Load never fills WorkspacePath in, so everything that
// needs the actual folder resolves it here.
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
