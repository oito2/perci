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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oito2/perci/internal/config"
)

func TestCertsDir(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	got, err := CertsDir()
	if err != nil {
		t.Fatalf("CertsDir: %v", err)
	}
	want := filepath.Join(tmp, ".perci", "appstack", "certs")
	if got != want {
		t.Errorf("CertsDir() = %q, want %q", got, want)
	}
}

func TestNginxConfDir(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	got, err := NginxConfDir()
	if err != nil {
		t.Fatalf("NginxConfDir: %v", err)
	}
	want := filepath.Join(tmp, ".perci", "appstack", "nginx")
	if got != want {
		t.Errorf("NginxConfDir() = %q, want %q", got, want)
	}
}

func TestBuildNginxConf_NoApps(t *testing.T) {
	got := BuildNginxConf(nil)

	// Docker's embedded DNS (127.0.0.11), paired with every routing recipe's
	// "set $backend ...;": without it, Nginx resolves each app's container
	// name once at config-load/reload time instead of per-request, so one
	// unrelated app merely being stopped fails `nginx -t` ("host not found in
	// upstream") for the whole file.
	if !strings.Contains(got, "resolver 127.0.0.11") {
		t.Errorf("expected a resolver directive pointing at Docker's embedded DNS, got:\n%s", got)
	}
	if !strings.Contains(got, "default_server") {
		t.Errorf("expected a default_server catch-all block, got:\n%s", got)
	}
	if !strings.Contains(got, "listen 443 ssl default_server;") {
		t.Errorf("expected TLS on the catch-all block, got:\n%s", got)
	}
	if !strings.Contains(got, "/etc/nginx/certs/"+wildcardCertFile) {
		t.Errorf("expected ssl_certificate to reference the mounted cert path, got:\n%s", got)
	}
	if !strings.Contains(got, "/etc/nginx/certs/"+wildcardKeyFile) {
		t.Errorf("expected ssl_certificate_key to reference the mounted key path, got:\n%s", got)
	}
	if strings.Contains(got, "server_name php-app.localhost") {
		t.Errorf("no apps given, but an app server block was generated:\n%s", got)
	}
}

func TestBuildAppServerBlock_PHP(t *testing.T) {
	app := config.AppContainer{Folder: "meuapp", Type: config.AppTypePHP, URL: "meuapp.localhost"}
	got := buildAppServerBlock(app)

	if !strings.Contains(got, "server_name meuapp.localhost;") {
		t.Errorf("missing server_name, got:\n%s", got)
	}
	if !strings.Contains(got, "root "+appHTMLMount+"/meuapp;") {
		t.Errorf("expected root at the app's own folder (no /public for non-Moodle), got:\n%s", got)
	}
	// fastcgi_pass goes through a "set $backend ...;" variable, not a literal
	// hostname (see nginxResolver's doc comment), so one unrelated app being
	// stopped can't fail `nginx -t` for the whole file.
	if !strings.Contains(got, "set $backend meuapp:9000;") || !strings.Contains(got, "fastcgi_pass $backend;") {
		t.Errorf("expected fastcgi_pass, via $backend, to the app's own container name, got:\n%s", got)
	}
	if !strings.Contains(got, "try_files $uri $uri/ /index.php?$query_string;") {
		t.Errorf("expected the default (non-Moodle) routing recipe, got:\n%s", got)
	}
	if strings.Contains(got, "r.php") {
		t.Errorf("non-Moodle app must not get Moodle's r.php routing, got:\n%s", got)
	}
}

func TestBuildAppServerBlock_Generic(t *testing.T) {
	// "Servidor Genérico" is technically identical to "Aplicação PHP" — same
	// routing recipe, same root convention.
	php := buildAppServerBlock(config.AppContainer{Folder: "app1", Type: config.AppTypePHP, URL: "app1.localhost"})
	generic := buildAppServerBlock(config.AppContainer{Folder: "app1", Type: config.AppTypeGeneric, URL: "app1.localhost"})
	if php != generic {
		t.Errorf("expected AppTypeGeneric and AppTypePHP to render identically:\nphp:\n%s\ngeneric:\n%s", php, generic)
	}
}

func TestBuildAppServerBlock_Moodle(t *testing.T) {
	app := config.AppContainer{Folder: "curso1", Type: config.AppTypeMoodle, URL: "curso1.localhost"}
	got := buildAppServerBlock(app)

	if !strings.Contains(got, "root "+appHTMLMount+"/curso1/public;") {
		t.Errorf("expected root at <folder>/public for Moodle, got:\n%s", got)
	}
	if !strings.Contains(got, "try_files $uri $uri/ /r.php$is_args$args;") {
		t.Errorf("expected Moodle's r.php routing recipe, got:\n%s", got)
	}
	if !strings.Contains(got, "set $backend curso1:9000;") || !strings.Contains(got, "fastcgi_pass $backend;") {
		t.Errorf("expected fastcgi_pass, via $backend, to the app's own container name, got:\n%s", got)
	}
	// SCRIPT_FILENAME/DOCUMENT_ROOT are hardcoded to /var/www/html instead of
	// using $realpath_root: Nginx and the app's php-fpm are separate
	// containers with different mounts of the same tree, and the app
	// container always sees its project root at /var/www/html.
	if !strings.Contains(got, "fastcgi_param SCRIPT_FILENAME /var/www/html/public$fastcgi_script_name;") {
		t.Errorf("expected SCRIPT_FILENAME hardcoded to the app container's own /var/www/html/public, got:\n%s", got)
	}
	if !strings.Contains(got, "fastcgi_param DOCUMENT_ROOT /var/www/html/public;") {
		t.Errorf("expected DOCUMENT_ROOT hardcoded to the app container's own /var/www/html/public, got:\n%s", got)
	}
}

func TestBuildAppServerBlock_MoodleVersionRanges(t *testing.T) {
	tests := []struct {
		name             string
		moodleVersion    string
		wantRootPublic   bool
		wantRPHP         bool
		wantClassicRegex bool
	}{
		{"empty falls back to 5.1+ (pre-MoodleVersion-field entries)", "", true, true, false},
		{MoodleVersion51Plus, MoodleVersion51Plus, true, true, false},
		{MoodleVersion50, MoodleVersion50, false, true, false},
		{MoodleVersion41, MoodleVersion41, false, false, true},
		{MoodleVersion42to43, MoodleVersion42to43, false, false, true},
		{MoodleVersion44to45, MoodleVersion44to45, false, false, true},
		{MoodleVersion4x, MoodleVersion4x, false, false, true},
		{MoodleVersion3x, MoodleVersion3x, false, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := config.AppContainer{Folder: "curso1", Type: config.AppTypeMoodle, URL: "curso1.localhost", MoodleVersion: tt.moodleVersion}
			got := buildAppServerBlock(app)

			hasPublicRoot := strings.Contains(got, "root "+appHTMLMount+"/curso1/public;")
			if hasPublicRoot != tt.wantRootPublic {
				t.Errorf("MoodleVersion %q: root .../public present = %v, want %v, got:\n%s", tt.moodleVersion, hasPublicRoot, tt.wantRootPublic, got)
			}
			if !tt.wantRootPublic && !strings.Contains(got, "root "+appHTMLMount+"/curso1;\n") {
				t.Errorf("MoodleVersion %q: expected root at the project folder (no /public), got:\n%s", tt.moodleVersion, got)
			}
			hasRPHP := strings.Contains(got, "r.php")
			if hasRPHP != tt.wantRPHP {
				t.Errorf("MoodleVersion %q: r.php present = %v, want %v, got:\n%s", tt.moodleVersion, hasRPHP, tt.wantRPHP, got)
			}
			hasClassicRegex := strings.Contains(got, `location ~ [^/]\.php(/|$) {`)
			if hasClassicRegex != tt.wantClassicRegex {
				t.Errorf("MoodleVersion %q: classic [^/]\\.php(/|$) location present = %v, want %v, got:\n%s", tt.moodleVersion, hasClassicRegex, tt.wantClassicRegex, got)
			}
			if !strings.Contains(got, "set $backend curso1:9000;") || !strings.Contains(got, "fastcgi_pass $backend;") {
				t.Errorf("MoodleVersion %q: expected fastcgi_pass, via $backend, to the app's own container name, got:\n%s", tt.moodleVersion, got)
			}
		})
	}
}

func TestBuildAppServerBlock_DenyBlocksPresent(t *testing.T) {
	for _, typ := range []string{config.AppTypeMoodle, config.AppTypePHP, config.AppTypeGeneric} {
		got := buildAppServerBlock(config.AppContainer{Folder: "app1", Type: typ, URL: "app1.localhost"})
		if !strings.Contains(got, `location ~ /\.(?!well-known).* {`) {
			t.Errorf("type %q: missing dotfile-deny block, got:\n%s", typ, got)
		}
		if !strings.Contains(got, "composer\\.json") {
			t.Errorf("type %q: missing internal-files-deny block, got:\n%s", typ, got)
		}
	}
}

func TestBuildAppServerBlock_InvalidFieldsSkipped(t *testing.T) {
	tests := []config.AppContainer{
		{Folder: "has space", Type: config.AppTypePHP, URL: "app1.localhost"},
		{Folder: "app1", Type: config.AppTypePHP, URL: "not-localhost.com"},
		{Folder: "app;evil", Type: config.AppTypePHP, URL: "app1.localhost"},
		{Folder: "app1", Type: config.AppTypeNode, URL: "app1.localhost", DevPort: 0},
		{Folder: "app1", Type: config.AppTypeNode, URL: "app1.localhost", DevPort: 80},
	}
	for _, app := range tests {
		if got := buildAppServerBlock(app); got != "" {
			t.Errorf("buildAppServerBlock(%+v) = %q, want empty (invalid Folder/URL/DevPort must never reach generated config)", app, got)
		}
	}
}

func TestBuildAppServerBlock_Node(t *testing.T) {
	app := config.AppContainer{Folder: "frontend", Type: config.AppTypeNode, URL: "frontend.localhost", DevPort: 5173}
	got := buildAppServerBlock(app)

	if !strings.Contains(got, "server_name frontend.localhost;") {
		t.Errorf("missing server_name, got:\n%s", got)
	}
	if strings.Contains(got, "root ") {
		t.Errorf("AppTypeNode must not get a root/index directive (proxy_pass only), got:\n%s", got)
	}
	if !strings.Contains(got, "set $backend frontend:5173;") || !strings.Contains(got, "proxy_pass http://$backend;") {
		t.Errorf("expected proxy_pass, via $backend, to the app's own container name and DevPort, got:\n%s", got)
	}
	if !strings.Contains(got, "proxy_http_version 1.1;") ||
		!strings.Contains(got, `proxy_set_header Upgrade $http_upgrade;`) ||
		!strings.Contains(got, `proxy_set_header Connection "upgrade";`) {
		t.Errorf("expected WebSocket upgrade headers (needed for Vite/webpack-dev-server HMR behind the proxy), got:\n%s", got)
	}
	if strings.Contains(got, "fastcgi_pass") {
		t.Errorf("AppTypeNode must not get fastcgi routing, got:\n%s", got)
	}
}

func TestBuildAppServerBlock_PHPNodeCombo(t *testing.T) {
	app := config.AppContainer{Folder: "projeto", Type: config.AppTypePHPNode, URL: "projeto.localhost", DevPort: 5173}
	got := buildAppServerBlock(app)

	if !strings.Contains(got, "server_name projeto.localhost;") {
		t.Errorf("missing server_name, got:\n%s", got)
	}
	if !strings.Contains(got, "root "+appHTMLMount+"/projeto;") {
		t.Errorf("expected root at the whole project folder (not .../api) — root+$uri is what makes /api/index.php resolve correctly, got:\n%s", got)
	}
	if !strings.Contains(got, "location /api/ {") {
		t.Errorf("missing the /api/ location, got:\n%s", got)
	}
	if !strings.Contains(got, "set $backend projeto:9000;") || !strings.Contains(got, "fastcgi_pass $backend;") {
		t.Errorf("expected the /api/ half to fastcgi_pass, via $backend, to the app's own container on :9000, got:\n%s", got)
	}
	if !strings.Contains(got, "try_files $uri $uri/ /api/index.php?$query_string;") {
		t.Errorf("expected /api/ requests to fall back to /api/index.php, preserving the /api prefix (no rewrite), got:\n%s", got)
	}
	if !strings.Contains(got, "set $backend projeto:5173;") || !strings.Contains(got, "proxy_pass http://$backend;") {
		t.Errorf("expected the catch-all / to proxy_pass, via $backend, to the app's own container and DevPort, got:\n%s", got)
	}
	if !strings.Contains(got, `proxy_set_header Upgrade $http_upgrade;`) {
		t.Errorf("expected WebSocket upgrade headers on the / proxy (Vite HMR), got:\n%s", got)
	}
}

func TestBuildAppServerBlock_PHPNodeCombo_UploadsAlias(t *testing.T) {
	// Files under api/uploads/ referenced by the frontend as /uploads/...
	// must be served by a dedicated location, not fall through to the
	// catch-all `location /` (proxied to the Node dev server).
	app := config.AppContainer{Folder: "projeto", Type: config.AppTypePHPNode, URL: "projeto.localhost", DevPort: 5173}
	got := buildAppServerBlock(app)

	if !strings.Contains(got, "location ^~ /uploads/ {") {
		t.Errorf("missing the /uploads/ alias location, got:\n%s", got)
	}
	if !strings.Contains(got, "alias "+appHTMLMount+"/projeto/api/uploads/;") {
		t.Errorf("expected /uploads/ aliased to the project's own api/uploads/ folder, got:\n%s", got)
	}
}

func TestBuildAppServerBlock_Node_NoDenyBlock(t *testing.T) {
	// Vite's dev server serves its own module cache at paths like
	// /node_modules/.vite/deps/vue.js. AppTypeNode is pure proxy_pass, so it
	// must carry no deny block for the dev server's own paths to reach it
	// (nginx dispatches regex locations, deny blocks included, ahead of the
	// `location /` proxy prefix).
	app := config.AppContainer{Folder: "frontend", Type: config.AppTypeNode, URL: "frontend.localhost", DevPort: 5173}
	got := buildAppServerBlock(app)

	if strings.Contains(got, "location ~") {
		t.Errorf("AppTypeNode must carry no deny block (regex location) at all, got:\n%s", got)
	}
}

func TestBuildAppServerBlock_PHPNodeCombo_DenyBlockScopedToAPI(t *testing.T) {
	// Same as TestBuildAppServerBlock_Node_NoDenyBlock, but for the combo
	// type: the /api/ (PHP) half must stay protected, while the catch-all /
	// (proxied to the Node dev server) must not be intercepted by a deny
	// block. "/node_modules/.vite/deps/vue.js" matches both the
	// dotfile-hiding regex (via "/.vite") and the vendor/node_modules regex,
	// so anchoring is required on both.
	app := config.AppContainer{Folder: "projeto", Type: config.AppTypePHPNode, URL: "projeto.localhost", DevPort: 5173}
	got := buildAppServerBlock(app)

	if !strings.Contains(got, `location ~ ^/api/\.(?!well-known).* {`) {
		t.Errorf("expected the dotfile-deny block anchored to /api/, got:\n%s", got)
	}
	if !strings.Contains(got, "^/api/.*(/vendor/|composer\\.json") {
		t.Errorf("expected the vendor/composer.json-deny block anchored to /api/, got:\n%s", got)
	}
	if strings.Contains(got, "node_modules") {
		t.Errorf("combo deny block must not reference node_modules at all — /node_modules/ is only ever requested outside /api/, got:\n%s", got)
	}
	// The unanchored blocks from appDenyBlocks must not appear verbatim —
	// they would shadow the / proxy location for paths like /node_modules/.
	if strings.Contains(got, "location ~ /\\.(?!well-known)") {
		t.Errorf("combo must not get the unanchored dotfile-deny block, got:\n%s", got)
	}
}

func TestBuildAppServerBlock_PHPNodeCombo_InvalidDevPortSkipped(t *testing.T) {
	app := config.AppContainer{Folder: "projeto", Type: config.AppTypePHPNode, URL: "projeto.localhost", DevPort: 9000}
	if got := buildAppServerBlock(app); got != "" {
		t.Errorf("buildAppServerBlock(%+v) = %q, want empty (DevPort colliding with the fastcgi port must never reach generated config)", app, got)
	}
}

func TestBuildNginxConf_MultipleApps(t *testing.T) {
	apps := []config.AppContainer{
		{Folder: "app1", Type: config.AppTypePHP, URL: "app1.localhost"},
		{Folder: "curso1", Type: config.AppTypeMoodle, URL: "curso1.localhost"},
	}
	got := BuildNginxConf(apps)

	if !strings.Contains(got, "server_name app1.localhost;") {
		t.Errorf("missing app1 block, got:\n%s", got)
	}
	if !strings.Contains(got, "server_name curso1.localhost;") {
		t.Errorf("missing curso1 block, got:\n%s", got)
	}
	if !strings.Contains(got, "server_name _;") {
		t.Errorf("missing the fixed catch-all default_server, got:\n%s", got)
	}
}

func TestWriteNginxConf(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	apps := []config.AppContainer{{Folder: "app1", Type: config.AppTypePHP, URL: "app1.localhost"}}
	path, err := writeNginxConf(apps)
	if err != nil {
		t.Fatalf("writeNginxConf: %v", err)
	}

	wantPath := filepath.Join(tmp, ".perci", "appstack", "nginx", "default.conf")
	if path != wantPath {
		t.Errorf("writeNginxConf() path = %q, want %q", path, wantPath)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read written conf: %v", err)
	}
	if !strings.Contains(string(data), "server_name app1.localhost;") {
		t.Errorf("written file missing app1 block, got:\n%s", data)
	}
}

func TestWriteNginxConf_Overwrites(t *testing.T) {
	// Recreating Nginx (CreateNginx -> writeNginxConf) must reflect the
	// current cfg.Docker.Apps, not silently keep stale content from a
	// previous call — this is the whole point of driving generation off
	// cfg.Docker.Apps instead of writing a static file once.
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	if _, err := writeNginxConf([]config.AppContainer{{Folder: "old", Type: config.AppTypePHP, URL: "old.localhost"}}); err != nil {
		t.Fatalf("seed writeNginxConf: %v", err)
	}

	path, err := writeNginxConf([]config.AppContainer{{Folder: "new", Type: config.AppTypePHP, URL: "new.localhost"}})
	if err != nil {
		t.Fatalf("writeNginxConf: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read written conf: %v", err)
	}
	if strings.Contains(string(data), "old.localhost") {
		t.Errorf("expected stale app block to be gone, got:\n%s", data)
	}
	if !strings.Contains(string(data), "new.localhost") {
		t.Errorf("expected current app block to be present, got:\n%s", data)
	}
}

func TestNginxRunArgs(t *testing.T) {
	got := nginxRunArgs(
		"/home/u/.perci/appstack/nginx/default.conf",
		"/home/u/.perci/appstack/certs",
		"/home/u/workspace/localhost/html",
		"/home/u/workspace/localhost/logs/nginx",
	)

	wantContains := []string{
		"--name", NginxContainerName,
		"--network", NetworkName,
		"-p", "127.0.0.1:80:80",
		"-p", "127.0.0.1:443:443",
		"/home/u/.perci/appstack/nginx/default.conf:/etc/nginx/conf.d/default.conf",
		"/home/u/.perci/appstack/certs:/etc/nginx/certs:ro",
		"/home/u/workspace/localhost/html:" + appHTMLMount + ":ro",
		"/home/u/workspace/localhost/logs/nginx:/var/log/nginx",
		NginxImage,
	}
	joined := strings.Join(got, " ")
	for _, w := range wantContains {
		if !strings.Contains(joined, w) {
			t.Errorf("nginxRunArgs() missing %q in:\n%v", w, got)
		}
	}
	if got[0] != "run" || got[1] != "-d" {
		t.Errorf("expected detached run, got args starting with %v", got[:2])
	}
	if got[len(got)-2] != "--" {
		t.Errorf("expected image to be preceded by \"--\" (defense against an image name starting with '-'), got %v", got[len(got)-3:])
	}
}
