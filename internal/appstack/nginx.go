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
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/oito2/perci/internal/config"
	"github.com/oito2/perci/internal/executor"
	"github.com/oito2/perci/internal/ui"
)

// appHTMLMount is where the whole {{workspace}}/localhost/html tree is
// bind-mounted, read-only, inside the Nginx container, not per-app, so
// adding a new Container Aplicativo only requires rewriting default.conf
// and reloading it.
const appHTMLMount = "/var/www/localhost/html"

// NetworkName is the Docker network every appstack container joins.
const NetworkName = "docker-php-network"

// NginxContainerName is the fixed name of the appstack's single Nginx
// container (its folder-name-as-container-name convention only applies to
// Container Aplicativo, not to Nginx/MariaDB themselves).
const NginxContainerName = "nginx"

// NginxImage is the Nginx image tag the appstack's Nginx container is
// created from.
const NginxImage = "nginx:1.26-alpine"

func isDockerInstalled(ctx context.Context, exe *executor.Executor) bool {
	return exe.CommandAvailable(ctx, "docker")
}

// NetworkExists reports whether NetworkName already exists.
func NetworkExists(ctx context.Context, exe *executor.Executor) bool {
	_, err := exe.Output(ctx, executor.Options{}, "docker", "network", "inspect", "--", NetworkName)
	return err == nil
}

// EnsureNetwork creates NetworkName if it doesn't exist yet. Safe to call
// every time a container is created — a no-op once the network is up.
func EnsureNetwork(ctx context.Context, exe *executor.Executor, stdout io.Writer) error {
	if NetworkExists(ctx, exe) {
		return nil
	}
	ui.Info(stdout, "Criando rede Docker "+NetworkName+"...")
	if err := exe.Run(ctx, executor.Options{Stdout: stdout, Stderr: stdout}, "docker", "network", "create", NetworkName); err != nil {
		return fmt.Errorf("criar rede %s: %w", NetworkName, err)
	}
	return nil
}

// NginxExists reports whether a container named NginxContainerName exists
// in any state (running or stopped). "docker run --name" refuses to start
// a new container while a stopped one still holds that name, so callers
// that want to recreate must check this — not just "is it running" — before
// calling CreateNginx.
func NginxExists(ctx context.Context, exe *executor.Executor) bool {
	return ContainerStatus(ctx, exe, NginxContainerName) != ""
}

// RemoveNginx force-removes the nginx container, running or stopped. The
// caller is responsible for confirming with the user first.
func RemoveNginx(ctx context.Context, exe *executor.Executor, stdout io.Writer) error {
	return removeContainer(ctx, exe, stdout, NginxContainerName, "contêiner nginx")
}

// wildcardCertFile / wildcardKeyFile are the fixed filenames of the
// wildcard certificate pair, so every caller can locate it without
// parsing mkcert's naming scheme.
const (
	wildcardCertFile = "wildcard.localhost.pem"
	wildcardKeyFile  = "wildcard.localhost-key.pem"
)

// WildcardCertReady reports whether the *.localhost wildcard certificate
// pair already exists at CertsDir(). When it does, EnsureWildcardCert (used
// by CreateNginx/RecreateNginx) reuses it as-is and never runs mkcert
// again, so no mkcert-managed privilege escalation happens. Callers can
// use it to warn before mkcert runs for the first time.
func WildcardCertReady() bool {
	dir, err := CertsDir()
	if err != nil {
		return false
	}
	if _, err := os.Stat(filepath.Join(dir, wildcardCertFile)); err != nil {
		return false
	}
	_, err = os.Stat(filepath.Join(dir, wildcardKeyFile))
	return err == nil
}

// CertsDir returns ~/.perci/appstack/certs, where the mkcert wildcard
// certificate for *.localhost is stored (under ~/.perci, not inside
// cfg.WorkspacePath).
func CertsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("obter diretório home: %w", err)
	}
	return filepath.Join(home, ".perci", "appstack", "certs"), nil
}

func isMkcertInstalled(ctx context.Context, exe *executor.Executor) bool {
	return exe.CommandAvailable(ctx, "mkcert")
}

// EnsureWildcardCert makes sure a *.localhost wildcard certificate pair
// exists at CertsDir(), generating one via mkcert if missing, and returns
// its (cert, key) paths. Idempotent: an existing pair is reused as-is, so
// recreating the Nginx container never mints a new certificate.
func EnsureWildcardCert(ctx context.Context, exe *executor.Executor, stdin io.Reader, stdout io.Writer) (certFile, keyFile string, err error) {
	if !isMkcertInstalled(ctx, exe) {
		return "", "", fmt.Errorf("mkcert não está instalado — instale em \"Dev Stuff :: Pré-requisitos\" antes de criar o Contêiner Nginx")
	}

	dir, err := CertsDir()
	if err != nil {
		return "", "", err
	}
	// 0o700: this directory holds the wildcard's private key.
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", "", fmt.Errorf("criar %s: %w", dir, err)
	}

	certFile = filepath.Join(dir, wildcardCertFile)
	keyFile = filepath.Join(dir, wildcardKeyFile)

	if _, statErr := os.Stat(certFile); statErr == nil {
		if _, statErr := os.Stat(keyFile); statErr == nil {
			return certFile, keyFile, nil
		}
	}

	// mkcert -install is idempotent — it no-ops with "The local CA is
	// already installed" once trusted — and manages its own privilege
	// escalation for the system trust store internally. It must run as the
	// invoking user, not wrapped in our own sudo: under sudo, $HOME (and so
	// mkcert's CAROOT) resolves to root's, not the user's, and mkcert would
	// still prompt for a password of its own on top of that.
	ui.Info(stdout, "Instalando autoridade certificadora local (mkcert -install)...")
	if err := exe.Run(ctx, executor.Options{Stdin: stdin, Stdout: stdout, Stderr: stdout}, "mkcert", "-install"); err != nil {
		return "", "", fmt.Errorf("mkcert -install: %w", err)
	}

	ui.Info(stdout, "Gerando certificado wildcard para *.localhost...")
	if err := exe.Run(ctx, executor.Options{Stdin: stdin, Stdout: stdout, Stderr: stdout},
		"mkcert", "-cert-file", certFile, "-key-file", keyFile, "*.localhost"); err != nil {
		return "", "", fmt.Errorf("gerar certificado: %w", err)
	}

	return certFile, keyFile, nil
}

// NginxConfDir returns ~/.perci/appstack/nginx, where default.conf is
// generated and bind-mounted into the nginx container.
func NginxConfDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("obter diretório home: %w", err)
	}
	return filepath.Join(home, ".perci", "appstack", "nginx"), nil
}

// nginxResolver points every server{} block's fastcgi_pass/proxy_pass at
// Docker's embedded DNS server instead of nginx's default of resolving
// upstream hostnames once, at config-load/reload time. Together with the
// set-a-variable-then-pass-it-to-fastcgi_pass/proxy_pass pattern in every
// routing recipe below (buildAppServerBlock's {{FOLDER}} substitution
// still fills in the hostname), an upstream container that is stopped or
// missing does not fail `nginx -t` with "host not found in upstream" for
// the whole file; it only fails requests to that app.
const nginxResolver = "resolver 127.0.0.11 valid=10s;\n\n"

// nginxConfBase is the fixed catch-all default_server, always present: it
// covers every *.localhost hostname that doesn't match one of the app
// blocks BuildNginxConf appends after it, with a 404 instead of falling
// through to whatever Docker's default nginx image would otherwise serve.
const nginxConfBase = `server {
    listen 80 default_server;
    listen 443 ssl default_server;
    server_name _;

    ssl_certificate     /etc/nginx/certs/` + wildcardCertFile + `;
    ssl_certificate_key /etc/nginx/certs/` + wildcardKeyFile + `;

    return 404;
}
`

// appDenyBlocks are appended to every server{} block whose type serves files
// straight off Nginx's own filesystem (root+try_files: Moodle/PHP/Generic;
// see buildAppServerBlock): they hide dotfiles and deny direct access to
// files that should never be served as static files (vendor/,
// node_modules/, composer.json, phpunit.xml, ...). They are not gated on
// AppTypeMoodle.
//
// They are not applied to AppTypeNode/AppTypePHPNode's proxied paths:
// nginx checks regex locations (these are `location ~ ...`) in file order
// and dispatches to the first match, ahead of any `location /` prefix
// block, so they would intercept requests meant for the dev server's own
// proxy_pass (e.g. Vite's /node_modules/.vite/deps/vue.js matches both
// blocks). AppTypeNode gets no deny block at all (nothing is served from
// Nginx's filesystem); AppTypePHPNode gets appComboDenyBlocks instead,
// anchored to /api/ so the PHP half stays protected without touching the
// proxied half.
const appDenyBlocks = `    location ~ /\.(?!well-known).* {
        return 404;
    }
    location ~ (/vendor/|/node_modules/|composer\.json|/readme|/README|readme\.txt|/upgrade\.txt|/UPGRADING\.md|db/install\.xml|/fixtures/|/behat/|phpunit\.xml|\.lock|environment\.xml) {
        deny all;
        return 404;
    }
`

// appComboDenyBlocks is AppTypePHPNode's deny block: the same protections
// as appDenyBlocks, but anchored to ^/api/ so they only match the PHP half
// of the combo (fastcgi, served via root+try_files off Nginx's own
// filesystem; see appComboRouting). The catch-all `/` half is a pure
// proxy_pass to the container's own Node dev server, which owns its own
// file-serving decisions.
const appComboDenyBlocks = `    location ~ ^/api/\.(?!well-known).* {
        return 404;
    }
    location ~ ^/api/.*(/vendor/|composer\.json|/readme|/README|readme\.txt|/upgrade\.txt|/UPGRADING\.md|db/install\.xml|/fixtures/|/behat/|phpunit\.xml|\.lock|environment\.xml) {
        deny all;
        return 404;
    }
`

// appMoodleRouting, appMoodleR50Routing, appMoodleClassicRouting and
// appDefaultRouting are the request-dispatch recipes a server{} block gets:
// AppTypeMoodle picks between the first three by MoodleVersion
// (moodleRoutingFor), every other PHP-family type always gets
// appDefaultRouting.
//
// appMoodleRouting is the recipe for the Moodle 5.1+ /public split-webroot
// layout. Every Container Aplicativo gets its own dedicated *.localhost
// vhost with root pointed straight at <folder>/public.
// SCRIPT_FILENAME/DOCUMENT_ROOT are hardcoded to the app container's own
// path instead of using `$realpath_root`: Nginx and php-fpm are separate
// containers, Nginx's root sees the whole localhost/html tree at
// appHTMLMount, while the app's own container mounts just its project root
// at the fixed /var/www/html, so `$realpath_root` resolves on Nginx's side
// and does not exist on php-fpm's. The same applies to every other fastcgi
// routing recipe below.
const appMoodleRouting = `    location / {
        try_files $uri $uri/ /r.php$is_args$args;
    }

    location ~ \.php(/|$) {
        fastcgi_split_path_info ^(.+\.php)(/.*)?$;
        set $path_info $fastcgi_path_info;
        try_files $fastcgi_script_name $fastcgi_script_name/ /r.php$is_args$args;
        set $backend {{FOLDER}}:9000;
        fastcgi_pass $backend;
        include fastcgi_params;
        fastcgi_param PATH_INFO $path_info;
        fastcgi_param SCRIPT_FILENAME /var/www/html/public$fastcgi_script_name;
        fastcgi_param DOCUMENT_ROOT /var/www/html/public;
    }
`

// appMoodleR50Routing is the recipe for Moodle 5.0: appMoodleRouting minus
// the /public root and minus the r.php fallback in both try_files chains
// (5.0 does not re-dispatch to r.php on a fastcgi miss). It is its own
// const rather than derived from appMoodleRouting by string manipulation.
// SCRIPT_FILENAME/DOCUMENT_ROOT are hardcoded to /var/www/html for the same
// reason as appMoodleRouting (no /public suffix here, matching root's own
// appHTMLMount+"/"+folder).
const appMoodleR50Routing = `    location / {
        try_files $uri $uri/ /r.php;
    }

    location ~ \.php(/|$) {
        fastcgi_split_path_info ^(.+\.php)(/.*)?$;
        set $path_info $fastcgi_path_info;
        try_files $fastcgi_script_name $fastcgi_script_name/;
        set $backend {{FOLDER}}:9000;
        fastcgi_pass $backend;
        include fastcgi_params;
        fastcgi_param PATH_INFO $path_info;
        fastcgi_param SCRIPT_FILENAME /var/www/html$fastcgi_script_name;
        fastcgi_param DOCUMENT_ROOT /var/www/html;
    }
`

// appMoodleClassicRouting is the recipe for Moodle 3.x/4.x, which has
// neither r.php nor the /public split, so every page is requested by its
// own .php filename directly (e.g. /login/index.php) with no front-controller
// rewrite: unlike appMoodleRouting/appMoodleR50Routing there is
// deliberately no "location /" block, since nginx's index/try_files
// defaults already resolve directory requests to index.php, and rewriting
// them would break Moodle's slash-argument URLs.
// SCRIPT_FILENAME/DOCUMENT_ROOT are hardcoded to /var/www/html for the same
// reason as appMoodleRouting.
const appMoodleClassicRouting = `    location ~ [^/]\.php(/|$) {
        fastcgi_split_path_info ^(.+\.php)(/.+)$;
        fastcgi_index index.php;
        set $backend {{FOLDER}}:9000;
        fastcgi_pass $backend;
        include fastcgi_params;
        fastcgi_param PATH_INFO $fastcgi_path_info;
        fastcgi_param SCRIPT_FILENAME /var/www/html$fastcgi_script_name;
        fastcgi_param DOCUMENT_ROOT /var/www/html;
    }
`

// appDefaultRouting hardcodes SCRIPT_FILENAME/DOCUMENT_ROOT to
// /var/www/html, the app container's project root, for the same reason as
// the Moodle recipes above.
const appDefaultRouting = `    location / {
        try_files $uri $uri/ /index.php?$query_string;
    }

    location ~ \.php$ {
        include fastcgi_params;
        set $backend {{FOLDER}}:9000;
        fastcgi_pass $backend;
        fastcgi_param SCRIPT_FILENAME /var/www/html$fastcgi_script_name;
        fastcgi_param DOCUMENT_ROOT /var/www/html;
    }
`

// appNodeRouting is AppTypeNode's whole routing recipe: every request is
// reverse-proxied to the container's own dev server (Vite,
// webpack-dev-server, ...) on DevPort, so there is no root/index to set for
// this type. proxy_http_version 1.1 plus the Upgrade/Connection headers
// let a WebSocket-based HMR connection survive being proxied.
const appNodeRouting = `    location / {
        set $backend {{FOLDER}}:{{DEVPORT}};
        proxy_pass http://$backend;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
    }
`

// appComboRouting is AppTypePHPNode's routing recipe: /api/* is preserved
// verbatim (no rewrite, so the app's own /api-prefixed routes match) and
// dispatched to the container's php-fpm on :9000, same fastcgi recipe as
// appDefaultRouting; everything else is reverse-proxied to the container's
// own dev server on DevPort, same recipe as appNodeRouting including the
// WebSocket upgrade headers HMR needs.
//
// root is set at server level (see buildAppServerBlock) to the whole
// project folder, not .../api: nginx computes a location's file path as
// root+$uri, and $uri already includes the "/api" segment, so
// root+"/api/index.php" resolves to the right file for try_files without
// any rewrite. The combo container's php-fpm only has the api/ half
// mounted, always at /var/www/html (comboAppRunArgs), so SCRIPT_FILENAME
// cannot reuse $document_root/$fastcgi_script_name: the "/api" prefix is
// stripped first (captured via the location regex) before prefixing
// /var/www/html.
//
// location ^~ /uploads/ serves user-uploaded files straight off Nginx's
// own filesystem (root is the whole project folder, so Nginx sees
// api/uploads/ directly, unlike php-fpm). Without it, /uploads/... would
// fall through to the catch-all `location /` and be proxied to the Node
// dev server, which has no such path. `^~` stops nginx from also checking
// the regex locations for a longer match.
const appComboRouting = `    location /api/ {
        try_files $uri $uri/ /api/index.php?$query_string;
    }

    location ~ ^/api/(.*\.php)$ {
        include fastcgi_params;
        set $backend {{FOLDER}}:9000;
        fastcgi_pass $backend;
        fastcgi_param SCRIPT_FILENAME /var/www/html/$1;
        fastcgi_param DOCUMENT_ROOT /var/www/html;
    }

    location ^~ /uploads/ {
        alias ` + appHTMLMount + `/{{FOLDER}}/api/uploads/;
    }

    location / {
        set $backend {{FOLDER}}:{{DEVPORT}};
        proxy_pass http://$backend;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
    }
`

// moodleRoutingFor picks the Nginx routing recipe for an AppTypeMoodle app
// by its MoodleVersion. An empty/unrecognized MoodleVersion falls back to
// appMoodleRouting (5.1+).
func moodleRoutingFor(moodleVersion string) string {
	switch moodleVersion {
	case MoodleVersion3x, MoodleVersion41, MoodleVersion42to43, MoodleVersion44to45, MoodleVersion4x:
		return appMoodleClassicRouting
	case MoodleVersion50:
		return appMoodleR50Routing
	default: // MoodleVersion51Plus, "", or anything unrecognized
		return appMoodleRouting
	}
}

// buildAppServerBlock renders one server{} block for app. Every field it
// interpolates unescaped into the config (Folder into fastcgi_pass/
// proxy_pass and root, URL into server_name, DevPort into proxy_pass for
// AppTypeNode/AppTypePHPNode) must already be valid per
// ValidAppFolder/ValidAppURL/ValidDevPort; an invalid entry is skipped with
// nothing emitted.
func buildAppServerBlock(app config.AppContainer) string {
	if !ValidAppFolder.MatchString(app.Folder) || !ValidAppURL.MatchString(app.URL) {
		return ""
	}
	needsDevPort := app.Type == config.AppTypeNode || app.Type == config.AppTypePHPNode
	if needsDevPort && !ValidDevPort(app.DevPort) {
		return ""
	}

	var b strings.Builder
	fmt.Fprintf(&b, "\nserver {\n    listen 80;\n    listen 443 ssl;\n    server_name %s;\n", app.URL)

	routing := appDefaultRouting
	denyBlock := appDenyBlocks
	switch app.Type {
	case config.AppTypeMoodle:
		routing = moodleRoutingFor(app.MoodleVersion)
		root := appHTMLMount + "/" + app.Folder
		if app.MoodleVersion == MoodleVersion51Plus || app.MoodleVersion == "" {
			root += "/public" // 5.1+ serves every web-accessible file from /public; "" counts as 5.1+
		}
		fmt.Fprintf(&b, "    root %s;\n    index index.php index.html;\n", root)
	case config.AppTypeNode:
		// No root/index — proxy_pass only, nothing served from disk, so no
		// deny block either (see appDenyBlocks' doc comment).
		routing = appNodeRouting
		denyBlock = ""
	case config.AppTypePHPNode:
		fmt.Fprintf(&b, "    root %s/%s;\n    index index.php index.html;\n", appHTMLMount, app.Folder)
		routing = appComboRouting
		denyBlock = appComboDenyBlocks
	default: // AppTypePHP, AppTypeGeneric
		fmt.Fprintf(&b, "    root %s/%s;\n    index index.php index.html;\n", appHTMLMount, app.Folder)
	}

	fmt.Fprintf(&b, "\n    ssl_certificate     /etc/nginx/certs/%s;\n    ssl_certificate_key /etc/nginx/certs/%s;\n\n", wildcardCertFile, wildcardKeyFile)

	routing = strings.ReplaceAll(routing, "{{FOLDER}}", app.Folder)
	routing = strings.ReplaceAll(routing, "{{DEVPORT}}", strconv.Itoa(app.DevPort))
	b.WriteString(denyBlock)
	b.WriteString(routing)
	b.WriteString("}\n")

	return b.String()
}

// BuildNginxConf renders the full default.conf content: the fixed
// catch-all block plus one server{} per app in apps.
func BuildNginxConf(apps []config.AppContainer) string {
	var b strings.Builder
	b.WriteString(nginxResolver)
	b.WriteString(nginxConfBase)
	for _, app := range apps {
		b.WriteString(buildAppServerBlock(app))
	}
	return b.String()
}

// writeNginxConf (re)writes default.conf in NginxConfDir() from apps and
// returns its path.
func writeNginxConf(apps []config.AppContainer) (string, error) {
	dir, err := NginxConfDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("criar %s: %w", dir, err)
	}
	path := filepath.Join(dir, "default.conf")
	if err := os.WriteFile(path, []byte(BuildNginxConf(apps)), 0o644); err != nil {
		return "", fmt.Errorf("escrever %s: %w", path, err)
	}
	return path, nil
}

// nginxContainerRunning reports whether the nginx container is up.
func nginxContainerRunning(ctx context.Context, exe *executor.Executor) bool {
	return ContainerStatus(ctx, exe, NginxContainerName) == "running"
}

// ReloadNginxConfig regenerates default.conf from apps and, if the nginx
// container is running, tests and reloads it without downtime — rolling
// the file back if the new config doesn't pass `nginx -t`. Called
// automatically after every Container Aplicativo create/edit/remove. If
// Nginx hasn't been created yet, the file is still written to disk so it's
// picked up whenever it is.
//
// This function only ever overwrites default.conf's content in place
// (os.WriteFile truncates the existing inode, it doesn't replace it), so
// the bind mount a running nginx container holds never goes stale.
func ReloadNginxConfig(ctx context.Context, exe *executor.Executor, stdout io.Writer, apps []config.AppContainer) error {
	path, err := NginxConfDir()
	if err != nil {
		return err
	}
	path = filepath.Join(path, "default.conf")

	oldContent, readErr := os.ReadFile(path) // readErr != nil is fine if it doesn't exist yet

	if _, err := writeNginxConf(apps); err != nil {
		return err
	}

	if !nginxContainerRunning(ctx, exe) {
		ui.Info(stdout, "nginx/default.conf atualizado em disco. Crie ou inicie o Contêiner Nginx para aplicar.")
		return nil
	}

	var out bytes.Buffer
	if err := exe.Run(ctx, executor.Options{Stdout: &out, Stderr: &out}, "docker", "exec", NginxContainerName, "nginx", "-t"); err != nil {
		if readErr == nil {
			if restoreErr := os.WriteFile(path, oldContent, 0o644); restoreErr != nil {
				return fmt.Errorf("configuração nginx inválida e falha ao reverter: %w", restoreErr)
			}
		} else {
			// No valid previous config to roll back to — remove the broken
			// one instead of leaving it on disk, where it would become the
			// (corrupt) baseline for the next reload's own rollback attempt.
			_ = os.Remove(path)
		}
		return fmt.Errorf("configuração nginx inválida (alterações revertidas): %s", out.String())
	}

	if err := exe.Run(ctx, executor.Options{Stdout: stdout, Stderr: stdout}, "docker", "exec", NginxContainerName, "nginx", "-s", "reload"); err != nil {
		return fmt.Errorf("recarregar nginx: %w", err)
	}
	ui.Success(stdout, "Roteamento do Nginx atualizado e aplicado sem downtime.")
	return nil
}

// nginxRunArgs builds the "docker run" argument list for the Nginx
// container, given the already-resolved paths for its config, certs, html
// tree and logs. Unit-testable without a Docker daemon.
func nginxRunArgs(confPath, certsDir, htmlDir, logDir string) []string {
	return []string{
		"run", "-d",
		"--name", NginxContainerName,
		"--network", NetworkName,
		"--restart", "unless-stopped",
		"-p", "127.0.0.1:80:80",
		"-p", "127.0.0.1:443:443",
		"-v", confPath + ":/etc/nginx/conf.d/default.conf",
		"-v", certsDir + ":/etc/nginx/certs:ro",
		"-v", htmlDir + ":" + appHTMLMount + ":ro",
		"-v", logDir + ":/var/log/nginx",
		"--",
		NginxImage,
	}
}

// CreateNginx creates the appstack's Nginx container: ensures NetworkName,
// ensures the mkcert wildcard certificate, (re)writes default.conf, and
// starts the container. Assumes no container named NginxContainerName
// exists yet; the caller checks NginxExists and calls RemoveNginx first
// when recreating. Returns the wildcard certificate's path.
func CreateNginx(ctx context.Context, exe *executor.Executor, stdin io.Reader, stdout io.Writer) (certFile string, err error) {
	if !isDockerInstalled(ctx, exe) {
		return "", fmt.Errorf("docker não instalado")
	}

	cfg, err := config.Load()
	if err != nil {
		return "", fmt.Errorf("carregar config: %w", err)
	}

	workspace, err := resolveWorkspace(cfg)
	if err != nil {
		return "", err
	}

	if err := EnsureNetwork(ctx, exe, stdout); err != nil {
		return "", err
	}

	certFile, _, err = EnsureWildcardCert(ctx, exe, stdin, stdout)
	if err != nil {
		return "", err
	}

	confPath, err := writeNginxConf(cfg.Docker.Apps)
	if err != nil {
		return "", err
	}

	htmlDir := filepath.Join(workspace, "localhost", "html")
	if err := os.MkdirAll(htmlDir, 0o755); err != nil {
		return "", fmt.Errorf("criar diretório html: %w", err)
	}

	logDir := filepath.Join(workspace, "localhost", "logs", "nginx")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return "", fmt.Errorf("criar diretório de logs: %w", err)
	}

	ui.Info(stdout, "Criando contêiner nginx...")
	args := nginxRunArgs(confPath, filepath.Dir(certFile), htmlDir, logDir)
	if err := exe.Run(ctx, executor.Options{Stdout: stdout, Stderr: stdout}, "docker", args...); err != nil {
		return "", fmt.Errorf("criar contêiner nginx: %w", err)
	}

	if err := config.Update(func(cfg *config.Config) error {
		cfg.Docker.NginxCreated = true
		return nil
	}); err != nil {
		return "", fmt.Errorf("contêiner nginx criado, mas falha ao salvar configurações: %w", err)
	}

	return certFile, nil
}

// RecreateNginx is CreateNginx's "Recriar" flavor: removes the existing
// container first if there is one, with no form and no confirmation of its
// own. Nginx has no user-editable parameters, so Recriar is its only
// mutating action beyond Start/Stop/Restart/Remover.
func RecreateNginx(ctx context.Context, exe *executor.Executor, stdin io.Reader, stdout io.Writer) (certFile string, err error) {
	if NginxExists(ctx, exe) {
		if err := RemoveNginx(ctx, exe, stdout); err != nil {
			return "", err
		}
	}
	return CreateNginx(ctx, exe, stdin, stdout)
}

// DeleteNginx removes the nginx container (if any) and clears
// cfg.Docker.NginxCreated. The caller is responsible for confirming with
// the user first.
func DeleteNginx(ctx context.Context, exe *executor.Executor, stdout io.Writer) error {
	if NginxExists(ctx, exe) {
		if err := RemoveNginx(ctx, exe, stdout); err != nil {
			return err
		}
	}
	if err := config.Update(func(cfg *config.Config) error {
		cfg.Docker.NginxCreated = false
		return nil
	}); err != nil {
		return fmt.Errorf("nginx removido, mas falha ao salvar configurações: %w", err)
	}
	return nil
}
