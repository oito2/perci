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
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/oito2/perci/internal/config"
	"github.com/oito2/perci/internal/executor"
)

// stackOpts selects how the fake docker of fakeStack answers.
type stackOpts struct {
	status         string // `docker inspect` State.Status of any container; "" = no container
	buildFails     bool
	runFails       bool // `docker run` of a container (not the --rm throwaway)
	nginxTestFails bool // `nginx -t` in a throwaway container
	noNginxImage   bool // `docker image inspect nginx:...` fails
}

// fakeStack puts a `docker` script first on PATH and returns its call log.
func fakeStack(t *testing.T, o stackOpts) string {
	t.Helper()
	flag := func(b bool) string {
		if b {
			return "1"
		}
		return "0"
	}
	bin := t.TempDir()
	log := filepath.Join(bin, "calls.log")
	script := `#!/bin/sh
printf '%s\n' "$*" >> '` + log + `'
case "$1" in
inspect) [ -n '` + o.status + `' ] || exit 1; printf '%s\n' '` + o.status + `' ;;
image) case "$*" in *nginx:*) [ ` + flag(o.noNginxImage) + ` = 1 ] && exit 1 ;; esac; exit 0 ;;
build) [ ` + flag(o.buildFails) + ` = 1 ] && { echo "build failed" >&2; exit 1; } ;;
run) case "$*" in
  *" --rm "*) [ ` + flag(o.nginxTestFails) + ` = 1 ] && { echo "emerg: bad directive" >&2; exit 1; } ;;
  *) [ ` + flag(o.runFails) + ` = 1 ] && { echo "run failed" >&2; exit 1; } ;;
  esac ;;
exec) case "$*" in *"nginx -t"*) ;; esac ;;
esac
exit 0
`
	if err := os.WriteFile(filepath.Join(bin, "docker"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return log
}

// calls returns the docker calls recorded in log.
func calls(t *testing.T, log string) string {
	t.Helper()
	b, _ := os.ReadFile(log)
	return string(b)
}

// stackHome sets HOME to a temporary folder, points the config at a
// workspace inside it and registers apps; it returns the workspace.
func stackHome(t *testing.T, apps ...config.AppContainer) string {
	t.Helper()
	home, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	ws := filepath.Join(home, "ws")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := config.Update(func(c *config.Config) error {
		c.WorkspacePath = ws
		c.Docker.Apps = apps
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return ws
}

func testExe() (*executor.Executor, *bytes.Buffer) {
	var buf bytes.Buffer
	return executor.New(&buf, &buf), &buf
}

var lojaApp = config.AppContainer{Name: "Loja", Folder: "loja", URL: "loja.localhost", Type: config.AppTypePHP, PHPVersion: "8.3"}

// A failed image build must not take the running container down, neither
// when editing nor when recreating.
func TestUpdateAndRecreateApp_BuildFailureKeepsContainer(t *testing.T) {
	stackHome(t, lojaApp)
	log := fakeStack(t, stackOpts{status: "running", buildFails: true})
	exe, _ := testExe()

	if err := UpdateApp(context.Background(), exe, &bytes.Buffer{}, lojaApp); err == nil {
		t.Fatal("UpdateApp: expected the build error")
	}
	if err := RecreateApp(context.Background(), exe, &bytes.Buffer{}, "loja"); err == nil {
		t.Fatal("RecreateApp: expected the build error")
	}
	got := calls(t, log)
	if strings.Contains(got, "rm -f") {
		t.Errorf("the running container was removed although the build failed:\n%s", got)
	}
}

// With a good build, the container is replaced: build, then rm, then run.
func TestUpdateApp_BuildsThenReplaces(t *testing.T) {
	stackHome(t, lojaApp)
	log := fakeStack(t, stackOpts{status: "running"})
	exe, buf := testExe()

	edited := lojaApp
	edited.PHPVersion = "8.4"
	if err := UpdateApp(context.Background(), exe, buf, edited); err != nil {
		t.Fatalf("UpdateApp: %v\n%s", err, buf.String())
	}
	got := calls(t, log)
	build, rm, run := strings.Index(got, "build "), strings.Index(got, "rm -f"), strings.Index(got, "run -d --name loja")
	if build < 0 || rm < build || run < rm {
		t.Errorf("want build, rm, run in this order:\n%s", got)
	}
	cfg, _ := config.Load()
	if len(cfg.Docker.Apps) != 1 || cfg.Docker.Apps[0].PHPVersion != "8.4" {
		t.Errorf("edit not persisted: %+v", cfg.Docker.Apps)
	}
}

// The image build runs before stackMu is taken: another stack operation is
// not blocked by it.
func TestCreateApp_BuildRunsOutsideStackLock(t *testing.T) {
	stackHome(t)
	bin := t.TempDir()
	started := filepath.Join(bin, "build-started")
	release := filepath.Join(bin, "release")
	script := `#!/bin/sh
case "$1" in
inspect) exit 1 ;;
build) : > '` + started + `'; while [ ! -e '` + release + `' ]; do sleep 0.01; done ;;
esac
exit 0
`
	if err := os.WriteFile(filepath.Join(bin, "docker"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	exe, buf := testExe()

	done := make(chan error, 1)
	go func() { done <- CreateApp(context.Background(), exe, buf, lojaApp) }()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(started); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the build never started")
		}
		time.Sleep(5 * time.Millisecond)
	}
	locked := make(chan struct{})
	go func() { stackMu.Lock(); stackMu.Unlock(); close(locked) }() //nolint:staticcheck // only acquires and releases
	select {
	case <-locked:
	case <-time.After(2 * time.Second):
		t.Error("stackMu is held while the image builds")
	}
	if err := os.WriteFile(release, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatalf("CreateApp: %v\n%s", err, buf.String())
	}
}

func TestStackContainerGuards(t *testing.T) {
	stackHome(t, lojaApp)
	log := fakeStack(t, stackOpts{status: "running"})
	exe, _ := testExe()
	ctx := context.Background()
	var out bytes.Buffer

	for _, name := range []string{"loja", NginxContainerName, MariaDBContainerName} {
		if err := StartContainer(ctx, exe, &out, name); err != nil {
			t.Errorf("Start %s: %v", name, err)
		}
		if err := StopContainer(ctx, exe, &out, name); err != nil {
			t.Errorf("Stop %s: %v", name, err)
		}
		if err := RestartContainer(ctx, exe, &out, name); err != nil {
			t.Errorf("Restart %s: %v", name, err)
		}
		if _, err := ContainerLogs(ctx, exe, name, 200); err != nil {
			t.Errorf("Logs %s: %v", name, err)
		}
	}
	before := calls(t, log)

	for _, name := range []string{"postgres", "outro", ""} {
		if err := StartContainer(ctx, exe, &out, name); err == nil {
			t.Errorf("Start %q accepted", name)
		}
		if err := StopContainer(ctx, exe, &out, name); err == nil {
			t.Errorf("Stop %q accepted", name)
		}
		if err := RestartContainer(ctx, exe, &out, name); err == nil {
			t.Errorf("Restart %q accepted", name)
		}
		if _, err := ContainerLogs(ctx, exe, name, 200); err == nil {
			t.Errorf("Logs %q accepted", name)
		}
	}
	if after := calls(t, log); after != before {
		t.Errorf("docker ran for a container that is not Perci's:\n%s", strings.TrimPrefix(after, before))
	}
}

func TestDeleteApp_RefusesNonApps(t *testing.T) {
	stackHome(t, lojaApp)
	log := fakeStack(t, stackOpts{status: "running"})
	exe, _ := testExe()

	for _, name := range []string{NginxContainerName, MariaDBContainerName, "unregistered"} {
		if err := DeleteApp(context.Background(), exe, &bytes.Buffer{}, name); err == nil {
			t.Errorf("DeleteApp(%q) accepted", name)
		}
	}
	if got := calls(t, log); strings.Contains(got, "rm -f") {
		t.Errorf("a container was removed:\n%s", got)
	}
	if err := DeleteApp(context.Background(), exe, &bytes.Buffer{}, "loja"); err != nil {
		t.Errorf("DeleteApp(loja): %v", err)
	}
	if cfg, _ := config.Load(); len(cfg.Docker.Apps) != 0 {
		t.Errorf("app kept: %+v", cfg.Docker.Apps)
	}
}

func TestValidateApp_NodeRejectsDBAccess(t *testing.T) {
	node := config.AppContainer{Folder: "spa", URL: "spa.localhost", Type: config.AppTypeNode, NodeVersion: "24", DevCommand: "npm run dev", DevPort: 5173}
	if err := ValidateApp(node); err != nil {
		t.Fatalf("valid Node app: %v", err)
	}
	node.DBAccess = true
	if err := ValidateApp(node); err == nil {
		t.Error("DBAccess on a Node app was accepted")
	}
}

func TestMountPathRules(t *testing.T) {
	cfg := &config.Config{WorkspacePath: "/srv/a,b"}
	if _, err := resolveWorkspace(cfg); err == nil {
		t.Error("a workspace with a comma was accepted")
	}
	cfg.WorkspacePath = "/srv/with:colon"
	if got, err := resolveWorkspace(cfg); err != nil || got != "/srv/with:colon" {
		t.Errorf("a colon is fine with --mount: %q, %v", got, err)
	}
	for _, p := range []string{"/a,b", `/a"b`, "/a\nb"} {
		if mountableError(p) == nil {
			t.Errorf("mountableError(%q) = nil", p)
		}
	}
	args := appRunArgs("x", "img", "/ws/with:colon/html", "", "/logs", "/conf.ini", nil)
	if !strings.Contains(strings.Join(args, " "), "source=/ws/with:colon/html,target=/var/www/html") {
		t.Errorf("colon path not passed through --mount: %v", args)
	}
	for _, a := range args {
		if a == "-v" {
			t.Errorf("-v still used: %v", args)
		}
	}
}

func TestMoodleDataDirPermissions(t *testing.T) {
	moodle := config.AppContainer{Name: "M", Folder: "mdle", URL: "mdle.localhost", Type: config.AppTypeMoodle, PHPVersion: "8.3", MoodleVersion: "5.1+"}
	ws := stackHome(t, moodle)
	fakeStack(t, stackOpts{})
	exe, buf := testExe()

	data := AppDataDir(ws, "mdle")
	if err := os.MkdirAll(data, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(data, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := prepareApp(context.Background(), exe, buf, moodle); err != nil {
		t.Fatalf("prepareApp: %v", err)
	}
	if info, _ := os.Stat(data); info.Mode().Perm() != 0o750 {
		t.Errorf("existing moodledata mode = %v, want 0750", info.Mode().Perm())
	}

	other := moodle
	other.Folder, other.URL = "mdle2", "mdle2.localhost"
	if _, err := prepareApp(context.Background(), exe, buf, other); err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Stat(AppDataDir(ws, "mdle2")); info.Mode().Perm() != 0o750 {
		t.Errorf("new moodledata mode = %v, want 0750", info.Mode().Perm())
	}
}

// ---- MariaDB ----

func TestCreateMariaDB_Password(t *testing.T) {
	stackHome(t)
	log := fakeStack(t, stackOpts{runFails: true})
	exe, buf := testExe()

	for _, pass := range []string{"", "a\nMYSQL_USER=root", "a\r"} {
		if err := CreateMariaDB(context.Background(), exe, buf, "dev", pass); err == nil {
			t.Errorf("password %q accepted", pass)
		}
	}
	if strings.Contains(calls(t, log), "run -d") {
		t.Error("docker run reached with an invalid password")
	}

	// The root password is saved before docker run: a failing run (or a
	// failing save after it) must not leave a data dir nobody can open.
	if err := CreateMariaDB(context.Background(), exe, buf, "dev", "s3cr3t"); err == nil {
		t.Fatal("expected the fake docker run failure")
	}
	cfg, _ := config.Load()
	if cfg.Docker.MariaDB.DBRootPass == "" {
		t.Error("root password not saved before docker run")
	}
}

func TestRecreateMariaDB_ChecksCredentialsBeforeRemoving(t *testing.T) {
	ws := stackHome(t)
	if err := os.MkdirAll(filepath.Join(MariaDBDataDir(ws), "mysql"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := config.Update(func(c *config.Config) error {
		c.Docker.MariaDB = config.MariaDBConfig{DBUser: "dev", DBPass: "new", DataUser: "dev", DataPass: "old"}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	log := fakeStack(t, stackOpts{status: "running"})
	exe, buf := testExe()

	if err := RecreateMariaDB(context.Background(), exe, buf); err == nil {
		t.Fatal("expected a credentials refusal")
	}
	if strings.Contains(calls(t, log), "rm -f") {
		t.Error("the database was removed before the credentials were checked")
	}
}

// Credentials from the file that the local data directory was not created
// with are refused before anything is saved or removed.
func TestImportConfig_CredentialMismatchTouchesNothing(t *testing.T) {
	ws := stackHome(t)
	if err := os.MkdirAll(filepath.Join(MariaDBDataDir(ws), "mysql"), 0o755); err != nil {
		t.Fatal(err)
	}
	local := config.MariaDBConfig{DBUser: "dev", DBPass: "old", DBRootPass: "rootsecret", DataUser: "dev", DataPass: "old"}
	if err := config.Update(func(c *config.Config) error { c.Docker.MariaDB = local; return nil }); err != nil {
		t.Fatal(err)
	}
	log := fakeStack(t, stackOpts{status: "running"})
	exe, buf := testExe()

	imported := ExportedConfig{WorkspacePath: ws, Docker: config.DockerConfig{
		MariaDB: config.MariaDBConfig{DBUser: "dev", DBPass: "other-machine"},
		Apps:    []config.AppContainer{lojaApp},
	}}
	if err := ImportConfig(context.Background(), exe, nil, buf, imported); err == nil {
		t.Fatal("expected a credentials refusal")
	}
	cfg, _ := config.Load()
	if cfg.Docker.MariaDB != local || len(cfg.Docker.Apps) != 0 {
		t.Errorf("config changed despite the refusal: %+v", cfg.Docker)
	}
	if got := calls(t, log); strings.Contains(got, "rm -f") || strings.Contains(got, "run -d") {
		t.Errorf("containers touched despite the refusal:\n%s", got)
	}
}

// The local root and data credentials survive an import whose file does not
// carry them.
func TestImportConfig_KeepsLocalMariaDBSecrets(t *testing.T) {
	ws := stackHome(t)
	if err := os.MkdirAll(filepath.Join(MariaDBDataDir(ws), "mysql"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := config.Update(func(c *config.Config) error {
		c.Docker.MariaDB = config.MariaDBConfig{DBUser: "dev", DBPass: "pw", DBRootPass: "rootsecret", DataUser: "dev", DataPass: "pw"}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	fakeStack(t, stackOpts{status: "running"})
	exe, buf := testExe()

	imported := ExportedConfig{WorkspacePath: ws, Docker: config.DockerConfig{MariaDB: config.MariaDBConfig{DBUser: "dev", DBPass: "pw"}}}
	if err := ImportConfig(context.Background(), exe, nil, buf, imported); err != nil {
		t.Fatalf("ImportConfig: %v\n%s", err, buf.String())
	}
	m := loadCfgNoFail(t).Docker.MariaDB
	if m.DBRootPass != "rootsecret" || m.DataUser != "dev" || m.DataPass != "pw" {
		t.Errorf("local secrets lost: %+v", m)
	}
}

// With an existing local data directory, the local root password wins over
// a different one carried by the file.
func TestImportConfig_LocalRootPassWinsOverFile(t *testing.T) {
	ws := stackHome(t)
	if err := os.MkdirAll(filepath.Join(MariaDBDataDir(ws), "mysql"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := config.Update(func(c *config.Config) error {
		c.Docker.MariaDB = config.MariaDBConfig{DBUser: "dev", DBPass: "pw", DBRootPass: "rootsecret", DataUser: "dev", DataPass: "pw"}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	fakeStack(t, stackOpts{status: "running"})
	exe, buf := testExe()
	imported := ExportedConfig{WorkspacePath: ws, Docker: config.DockerConfig{MariaDB: config.MariaDBConfig{DBUser: "dev", DBPass: "pw", DBRootPass: "fromfile"}}}
	if err := ImportConfig(context.Background(), exe, nil, buf, imported); err != nil {
		t.Fatalf("ImportConfig: %v\n%s", err, buf.String())
	}
	if m := loadCfgNoFail(t).Docker.MariaDB; m.DBRootPass != "rootsecret" {
		t.Errorf("root password = %q, want the local one", m.DBRootPass)
	}
}

func loadCfgNoFail(t *testing.T) *config.Config {
	t.Helper()
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

// ---- wrappers ----

func writeFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}

func TestWrappers_MarkerAndForeignFiles(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	bin := filepath.Join(home, ".local", "bin")
	var out bytes.Buffer

	writeFile(t, filepath.Join(bin, "php-loja"), "#!/bin/sh\necho mine\n", 0o755)                                            // not perci's
	writeFile(t, filepath.Join(bin, "composer-loja"), legacyWrapperPrefix+"'loja'\nexec docker exec -i x composer\n", 0o755) // old perci layout
	writeAppWrappers("loja", "/ws/html/loja", &out)

	if b, _ := os.ReadFile(filepath.Join(bin, "php-loja")); string(b) != "#!/bin/sh\necho mine\n" {
		t.Errorf("a file that is not a wrapper was overwritten: %q", b)
	}
	if !strings.Contains(out.String(), "não é um wrapper do Perci") {
		t.Errorf("no warning about the kept file:\n%s", out.String())
	}
	if b, _ := os.ReadFile(filepath.Join(bin, "composer-loja")); !strings.Contains(string(b), wrapperMarker) {
		t.Errorf("an old perci wrapper was not replaced:\n%s", b)
	}
	if b, _ := os.ReadFile(filepath.Join(bin, "phpcs-loja")); !strings.HasPrefix(string(b), "#!/usr/bin/env bash\n"+wrapperMarker+"\n") {
		t.Errorf("new wrapper lacks the marker line:\n%s", b)
	}

	removeAppWrappers("loja")
	if _, err := os.Stat(filepath.Join(bin, "php-loja")); err != nil {
		t.Error("removeAppWrappers deleted a file that is not a perci wrapper")
	}
	for _, tool := range []string{"phpcs", "phpcbf", "phpunit", "composer"} {
		if _, err := os.Stat(filepath.Join(bin, tool+"-loja")); !os.IsNotExist(err) {
			t.Errorf("%s-loja kept", tool)
		}
	}
}

// Changing an app's type replaces its wrappers: the old type's are gone.
func TestUpdateApp_TypeChangeRemovesOldWrappers(t *testing.T) {
	stackHome(t, lojaApp)
	fakeStack(t, stackOpts{status: "running"})
	exe, buf := testExe()
	bin := filepath.Join(os.Getenv("HOME"), ".local", "bin")

	if err := RecreateApp(context.Background(), exe, buf, "loja"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(bin, "php-loja")); err != nil {
		t.Fatalf("php wrapper missing: %v", err)
	}
	node := config.AppContainer{Name: "Loja", Folder: "loja", URL: "loja.localhost", Type: config.AppTypeNode, NodeVersion: "24", DevCommand: "npm run dev", DevPort: 5173}
	if err := UpdateApp(context.Background(), exe, buf, node); err != nil {
		t.Fatalf("UpdateApp: %v\n%s", err, buf.String())
	}
	if _, err := os.Stat(filepath.Join(bin, "php-loja")); !os.IsNotExist(err) {
		t.Error("the PHP wrapper survived the change to a Node app")
	}
	if _, err := os.Stat(filepath.Join(bin, "npm-loja")); err != nil {
		t.Errorf("npm wrapper missing: %v", err)
	}
}

// A workspace path with glob characters is translated literally.
func TestWrapper_TranslatesPathsWithGlobCharacters(t *testing.T) {
	dir := t.TempDir()
	fake := filepath.Join(dir, "docker")
	writeFile(t, fake, "#!/bin/sh\nprintf '%s\\n' \"$@\"\n", 0o755)
	host := "/srv/proj[1]/*/html"
	script := filepath.Join(dir, "php-x")
	writeFile(t, script, buildAppWrapperScript("c", "php", host, "/var/www/html", ""), 0o755)

	cmd := exec.Command(script, host+"/index.php", "plain")
	cmd.Env = []string{"PATH=" + dir + ":/usr/bin:/bin"}
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "\n/var/www/html/index.php\n") {
		t.Errorf("path not translated:\n%s", out)
	}
}

// ---- Moodle plugins ----

func rewriteComponents(t *testing.T, workspace, content string) {
	t.Helper()
	writeFile(t, filepath.Join(AppHTMLDir(workspace, "mdle"), "lib", "components.json"), content, 0o644)
}

func TestPlanMoodlePlugin_RefusesDestinationsOutsideTheMoodle(t *testing.T) {
	workspace, dev := pluginStack(t, map[string]string{"local": "local"})
	html := AppHTMLDir(workspace, "mdle")
	outside := t.TempDir()

	cases := map[string]string{
		"parent traversal": `{"plugintypes": {"local": "../../../../etc/polkit-1/rules.d"}}`,
		"absolute":         `{"plugintypes": {"local": "/etc/polkit-1/rules.d"}}`,
		"empty":            `{"plugintypes": {"local": ""}}`,
	}
	for name, components := range cases {
		t.Run(name, func(t *testing.T) {
			rewriteComponents(t, workspace, components)
			if _, err := PlanMoodlePlugin(loadCfg(t), dev, "mdle", "local", "x"); err == nil {
				t.Error("accepted")
			}
		})
	}

	rewriteComponents(t, workspace, `{"plugintypes": {"local": "local"}}`)
	t.Run("symlinked type folder", func(t *testing.T) {
		if err := os.RemoveAll(filepath.Join(html, "local")); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, filepath.Join(html, "local")); err != nil {
			t.Fatal(err)
		}
		if _, err := PlanMoodlePlugin(loadCfg(t), dev, "mdle", "local", "x"); err == nil {
			t.Error("accepted a destination reached through a link to another folder")
		}
	})
	t.Run("dangling link", func(t *testing.T) {
		if err := os.Remove(filepath.Join(html, "local")); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(outside, "missing"), filepath.Join(html, "local")); err != nil {
			t.Fatal(err)
		}
		if _, err := PlanMoodlePlugin(loadCfg(t), dev, "mdle", "local", "x"); err == nil {
			t.Error("accepted a destination below a dangling link")
		}
	})
}

func TestResolvedWithin(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "a"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		path string
		want bool
	}{
		{filepath.Join(root, "a", "b", "c"), true},
		{filepath.Join(root, "a"), true},
		{filepath.Join(root, "link", "x"), false},
		{filepath.Join(root, "..", "x"), false},
	}
	for _, tc := range cases {
		if got := resolvedWithin(root, tc.path); got != tc.want {
			t.Errorf("resolvedWithin(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

// A tampered record must not reach the privileged step.
func TestRemoveMoodlePlugin_RefusesTamperedRecord(t *testing.T) {
	_, dev := pluginStack(t, map[string]string{"local": "local"})
	var buf bytes.Buffer
	exe := &executor.Executor{DryRun: true, UsePolicyKit: true, Stdout: &buf, Stderr: &buf}
	for _, rel := range []string{"../../../etc/polkit-1/rules.d", "/etc/systemd/system"} {
		if err := config.Update(func(c *config.Config) error {
			c.MoodlePlugins = []config.MoodlePluginMount{{App: "mdle", Component: "local_x", DevPath: dev, RelPath: rel}}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		buf.Reset()
		if err := RemoveMoodlePlugin(context.Background(), exe, &buf, "mdle", rel); err == nil {
			t.Errorf("record %q accepted", rel)
		}
		if strings.Contains(buf.String(), "pkexec") {
			t.Errorf("record %q reached the privileged step:\n%s", rel, buf.String())
		}
	}
}

// A recorded path that reaches outside the Moodle through a link is
// refused too.
func TestRemoveMoodlePlugin_RefusesRecordBehindALink(t *testing.T) {
	workspace, dev := pluginStack(t, map[string]string{"local": "local"})
	html := AppHTMLDir(workspace, "mdle")
	if err := os.RemoveAll(filepath.Join(html, "local")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(t.TempDir(), filepath.Join(html, "local")); err != nil {
		t.Fatal(err)
	}
	if err := config.Update(func(c *config.Config) error {
		c.MoodlePlugins = []config.MoodlePluginMount{{App: "mdle", Component: "local_x", DevPath: dev, RelPath: "local/x"}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	exe := &executor.Executor{DryRun: true, UsePolicyKit: true, Stdout: &buf, Stderr: &buf}
	if err := RemoveMoodlePlugin(context.Background(), exe, &buf, "mdle", "local/x"); err == nil || strings.Contains(buf.String(), "pkexec") {
		t.Errorf("err = %v, output:\n%s", err, buf.String())
	}
}

func TestMountUnit_EscapesPercentAndRefusesBackslash(t *testing.T) {
	got := mountUnitContent(MoodlePluginPlan{App: "mdle", Component: "local_x", DevPath: "/w/100%dev", Dest: "/w/html/100%/x"})
	for _, want := range []string{"What=/w/100%%dev\n", "Where=/w/html/100%%/x\n", "RequiresMountsFor=/w/100%%dev\n"} {
		if !strings.Contains(got, want) {
			t.Errorf("unit lacks %q:\n%s", want, got)
		}
	}

	_, dev := pluginStack(t, map[string]string{"local": "local"})
	backslashed := filepath.Join(filepath.Dir(dev), `back\slash`)
	if err := os.MkdirAll(backslashed, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := PlanMoodlePlugin(loadCfg(t), backslashed, "mdle", "local", "x"); err == nil {
		t.Error("a development folder with a backslash was accepted")
	}
	pct := filepath.Join(filepath.Dir(dev), "100%dev")
	if err := os.MkdirAll(pct, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := PlanMoodlePlugin(loadCfg(t), pct, "mdle", "local", "x"); err != nil {
		t.Errorf("a percent sign is escaped, not refused: %v", err)
	}
}

// hookWriter calls fn on every write, to inspect state at the time a step
// is announced.
type hookWriter struct {
	buf bytes.Buffer
	fn  func(string)
}

func (h *hookWriter) Write(p []byte) (int, error) {
	h.fn(string(p))
	return h.buf.Write(p)
}

// The mount is recorded before the privileged step runs.
func TestAddMoodlePlugin_RecordsBeforeMounting(t *testing.T) {
	_, dev := pluginStack(t, map[string]string{"local": "local"})
	recordedAtPrivilegedStep := false
	w := &hookWriter{fn: func(s string) {
		if strings.Contains(s, "pkexec") {
			cfg, err := config.Load()
			recordedAtPrivilegedStep = err == nil && len(cfg.MoodlePlugins) == 1
		}
	}}
	exe := &executor.Executor{DryRun: true, UsePolicyKit: true, Stdout: w, Stderr: w}
	if err := AddMoodlePlugin(context.Background(), exe, w, dev, "mdle", "local", "example"); err != nil {
		t.Fatal(err)
	}
	if !recordedAtPrivilegedStep {
		t.Error("the record did not exist when the privileged step ran")
	}
}

// A failed installation leaves no record and no folder it created.
func TestAddMoodlePlugin_FailureRollsBack(t *testing.T) {
	workspace, dev := pluginStack(t, map[string]string{"local": "local"})
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "missing")) // the staging file cannot be created
	var buf bytes.Buffer
	exe := &executor.Executor{DryRun: true, UsePolicyKit: true, Stdout: &buf, Stderr: &buf}

	if err := AddMoodlePlugin(context.Background(), exe, &buf, dev, "mdle", "local", "example"); err == nil {
		t.Fatal("expected the staging failure")
	}
	if recs := loadCfg(t).MoodlePlugins; len(recs) != 0 {
		t.Errorf("record kept after the failure: %+v", recs)
	}
	if _, err := os.Stat(filepath.Join(AppHTMLDir(workspace, "mdle"), "local", "example")); !os.IsNotExist(err) {
		t.Error("the folder created for the mount was kept")
	}
}

// stubBin puts executables (name → script body) first on PATH.
func stubBin(t *testing.T, scripts map[string]string) {
	t.Helper()
	dir := t.TempDir()
	for name, body := range scripts {
		writeFile(t, filepath.Join(dir, name), "#!/bin/sh\n"+body+"\n", 0o755)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// The script that removes a unit file keeps it while the folder is still a
// mount point.
func TestGuardedRemoveScript(t *testing.T) {
	for _, mounted := range []bool{true, false} {
		unitFile := filepath.Join(t.TempDir(), "x.mount")
		writeFile(t, unitFile, "unit", 0o644)
		rc := "1"
		if mounted {
			rc = "0"
		}
		stubBin(t, map[string]string{"mountpoint": "exit " + rc})
		err := exec.Command("sh", "-c", guardedRemoveScript, "sh", "x.mount", unitFile, "/dest").Run()
		_, statErr := os.Stat(unitFile)
		if mounted && (err == nil || statErr != nil) {
			t.Errorf("mounted: err=%v statErr=%v, want a failure and the unit kept", err, statErr)
		}
		if !mounted && (err != nil || !os.IsNotExist(statErr)) {
			t.Errorf("unmounted: err=%v statErr=%v, want the unit removed", err, statErr)
		}
	}
}

// When enabling fails, the same batch stops and removes the unit.
func TestEnableOrRollbackScript(t *testing.T) {
	unitFile := filepath.Join(t.TempDir(), "x.mount")
	writeFile(t, unitFile, "unit", 0o644)
	log := filepath.Join(t.TempDir(), "systemctl.log")
	stubBin(t, map[string]string{
		"systemctl":  `printf '%s\n' "$*" >> '` + log + `'; [ "$1" = enable ] && exit 1; exit 0`,
		"mountpoint": "exit 1",
	})
	if err := exec.Command("sh", "-c", enableOrRollbackScript, "sh", "x.mount", unitFile, "/dest").Run(); err == nil {
		t.Error("a failed enable must fail the batch")
	}
	if _, err := os.Stat(unitFile); !os.IsNotExist(err) {
		t.Error("the unit file was left behind")
	}
	b, _ := os.ReadFile(log)
	for _, want := range []string{"enable --now -- x.mount", "disable --now -- x.mount", "daemon-reload"} {
		if !strings.Contains(string(b), want) {
			t.Errorf("systemctl never ran %q:\n%s", want, b)
		}
	}
}

// Units are found by what they mount, so a changed workspace path does not
// orphan them; files that are not perci's are ignored.
func TestPluginUnits(t *testing.T) {
	dir := t.TempDir()
	old := systemdUnitDir
	systemdUnitDir = dir
	t.Cleanup(func() { systemdUnitDir = old })

	m := config.MoodlePluginMount{App: "mdle", Component: "local_x", DevPath: "/dev/x", RelPath: "local/x"}
	mine := mountUnitContent(MoodlePluginPlan{App: "mdle", Component: "local_x", DevPath: "/dev/x", Dest: "/old/ws/localhost/html/mdle/local/x"})
	writeFile(t, filepath.Join(dir, "old-name.mount"), mine, 0o644)
	writeFile(t, filepath.Join(dir, "foreign.mount"), strings.TrimPrefix(mine, mountUnitHeader+"\n"), 0o644)
	other := mountUnitContent(MoodlePluginPlan{App: "other", Component: "local_x", DevPath: "/dev/x", Dest: "/old/ws/localhost/html/other/local/x"})
	writeFile(t, filepath.Join(dir, "other-app.mount"), other, 0o644)

	got := pluginUnits(m, "/new/ws/localhost/html/mdle/local/x")
	names := map[string]bool{}
	for _, u := range got {
		names[u.name] = true
	}
	if !names["old-name.mount"] || !names[mountUnitName("/new/ws/localhost/html/mdle/local/x")] || len(names) != 2 {
		t.Errorf("units = %+v", got)
	}
}

// Each container is inspected once and asked once for all its folders.
func TestMoodlePluginStatuses_BatchesContainerCalls(t *testing.T) {
	workspace, dev := pluginStack(t, map[string]string{"local": "local"})
	dev2 := filepath.Join(filepath.Dir(dev), "second")
	for _, d := range []string{dev2, filepath.Join(AppHTMLDir(workspace, "mdle"), "local", "a"), filepath.Join(AppHTMLDir(workspace, "mdle"), "local", "b")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := config.Update(func(c *config.Config) error {
		c.MoodlePlugins = []config.MoodlePluginMount{
			{App: "mdle", Component: "local_a", DevPath: dev, RelPath: "local/a"},
			{App: "mdle", Component: "local_b", DevPath: dev2, RelPath: "local/b"},
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	log := fakeStack(t, stackOpts{status: "running"})
	exe, _ := testExe()

	states := MoodlePluginStatuses(context.Background(), exe, loadCfgNoFail(t))
	if len(states) != 2 {
		t.Fatalf("states = %+v", states)
	}
	got := calls(t, log)
	if n := strings.Count(got, "exec mdle "); n != 1 {
		t.Errorf("%d docker exec calls on the app container, want 1:\n%s", n, got)
	}
	if n := strings.Count(got, "exec nginx "); n != 1 {
		t.Errorf("%d docker exec calls on nginx, want 1:\n%s", n, got)
	}
	if n := strings.Count(got, "inspect -f {{.State.Status}} -- mdle"); n != 1 {
		t.Errorf("app container inspected %d times, want 1:\n%s", n, got)
	}
}

// ---- Nginx ----

func writeCertPair(t *testing.T) {
	t.Helper()
	dir, err := CertsDir()
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, wildcardCertFile), "cert", 0o600)
	writeFile(t, filepath.Join(dir, wildcardKeyFile), "key", 0o600)
}

func TestReloadNginxConfig_StoppedValidatesBeforeWriting(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	writeCertPair(t)
	path := nginxConfPath(t)
	writeFile(t, path, "OLD", 0o644)
	log := fakeStack(t, stackOpts{status: "exited", nginxTestFails: true})
	exe, buf := testExe()

	err := ReloadNginxConfig(context.Background(), exe, buf, reloadApps)
	if err == nil || !strings.Contains(err.Error(), "bad directive") || !strings.Contains(err.Error(), "nada foi gravado") {
		t.Fatalf("err = %v, want nginx's message and a refusal to write", err)
	}
	if b, _ := os.ReadFile(path); string(b) != "OLD" {
		t.Errorf("default.conf = %q, want the previous content", b)
	}
	got := calls(t, log)
	if !strings.Contains(got, "run --rm --network none --entrypoint nginx") || !strings.Contains(got, "readonly -- "+NginxImage+" -t") {
		t.Errorf("no throwaway nginx -t:\n%s", got)
	}
	dir, _ := NginxConfDir()
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Errorf("candidate file left in the conf dir: %v", entries)
	}
}

func TestReloadNginxConfig_StoppedWritesValidConfig(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	writeCertPair(t)
	path := nginxConfPath(t)
	fakeStack(t, stackOpts{status: "exited"})
	exe, buf := testExe()

	if err := ReloadNginxConfig(context.Background(), exe, buf, reloadApps); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(path); !strings.Contains(string(b), "loja.localhost") {
		t.Errorf("default.conf not written:\n%s", b)
	}
}

// With nothing to test with (no certificate yet, or no local image) the
// file is written without a throwaway container.
func TestReloadNginxConfig_StoppedSkipsValidationWithoutPrerequisites(t *testing.T) {
	for name, setup := range map[string]func(*testing.T) stackOpts{
		"no certificate": func(t *testing.T) stackOpts { return stackOpts{status: "exited", nginxTestFails: true} },
		"no image": func(t *testing.T) stackOpts {
			writeCertPair(t)
			return stackOpts{status: "exited", nginxTestFails: true, noNginxImage: true}
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			path := nginxConfPath(t)
			log := fakeStack(t, setup(t))
			exe, buf := testExe()
			if err := ReloadNginxConfig(context.Background(), exe, buf, reloadApps); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(path); err != nil {
				t.Errorf("default.conf not written: %v", err)
			}
			if strings.Contains(calls(t, log), "--rm") {
				t.Error("a throwaway container ran")
			}
		})
	}
}

func TestCertNeedsRenewal(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, notAfter time.Time) string {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		tmpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "*.localhost"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: notAfter}
		der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
		if err != nil {
			t.Fatal(err)
		}
		p := filepath.Join(dir, name)
		writeFile(t, p, string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})), 0o600)
		return p
	}
	now := time.Now()
	if certNeedsRenewal(write("fresh.pem", now.Add(90*24*time.Hour)), now) {
		t.Error("a certificate valid for 90 more days needs no renewal")
	}
	if !certNeedsRenewal(write("soon.pem", now.Add(10*24*time.Hour)), now) {
		t.Error("a certificate expiring in 10 days must be renewed")
	}
	if !certNeedsRenewal(write("expired.pem", now.Add(-time.Hour)), now) {
		t.Error("an expired certificate must be renewed")
	}
	garbage := filepath.Join(dir, "garbage.pem")
	writeFile(t, garbage, "not a certificate", 0o600)
	if !certNeedsRenewal(garbage, now) || !certNeedsRenewal(filepath.Join(dir, "missing.pem"), now) {
		t.Error("an unreadable certificate must be renewed")
	}
}

// An expiring certificate is regenerated by mkcert; a fresh one is reused.
func TestEnsureWildcardCert_RenewsExpiringCertificate(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir, _ := CertsDir()
	mkcertLog := filepath.Join(t.TempDir(), "mkcert.log")
	stubBin(t, map[string]string{"mkcert": `printf '%s\n' "$*" >> '` + mkcertLog + `'
while [ $# -gt 0 ]; do case "$1" in -cert-file|-key-file) : > "$2"; shift 2 ;; *) shift ;; esac; done`})
	exe, buf := testExe()

	writeFile(t, filepath.Join(dir, wildcardCertFile), "not a certificate", 0o600)
	writeFile(t, filepath.Join(dir, wildcardKeyFile), "key", 0o600)
	if _, _, err := EnsureWildcardCert(context.Background(), exe, nil, buf); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(mkcertLog); !strings.Contains(string(b), "-cert-file") {
		t.Errorf("mkcert did not regenerate the certificate:\n%s", b)
	}
	if !strings.Contains(buf.String(), "expira em menos de 30 dias") {
		t.Errorf("no renewal notice:\n%s", buf.String())
	}
}

// CreateNginx writes default.conf only while holding stackMu.
func TestCreateNginx_WritesConfUnderStackLock(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	fakeStack(t, stackOpts{})
	stubBin(t, map[string]string{"mkcert": `while [ $# -gt 0 ]; do case "$1" in -cert-file|-key-file) : > "$2"; shift 2 ;; *) shift ;; esac; done`})
	exe, buf := testExe()

	stackMu.Lock()
	var finished atomic.Bool
	done := make(chan error, 1)
	go func() {
		_, err := CreateNginx(context.Background(), exe, nil, buf)
		finished.Store(true)
		done <- err
	}()
	time.Sleep(300 * time.Millisecond)
	if finished.Load() {
		t.Error("CreateNginx wrote default.conf while another operation held stackMu")
	}
	stackMu.Unlock()
	if err := <-done; err != nil {
		t.Fatalf("CreateNginx: %v\n%s", err, buf.String())
	}
}

// ---- images ----

func TestDockerfilesPinTheirSupplyChain(t *testing.T) {
	if !regexp.MustCompile(`^2\.\d+\.\d+@sha256:[0-9a-f]{64}$`).MatchString(composerImage) {
		t.Errorf("composerImage %q is not a 2.x tag plus digest", composerImage)
	}
	for name, df := range map[string]string{"php": phpImageDockerfile, "combo": comboDockerfile} {
		if strings.Contains(df, "composer:latest") || !strings.Contains(df, "COPY --from=composer:"+composerImage) || !strings.Contains(composerImage, "@sha256:") {
			t.Errorf("%s: composer is not pinned by tag and digest", name)
		}
		if !strings.Contains(df, "pecl install redis-"+redisVersion) || !strings.Contains(df, `pecl install "xdebug-${XDEBUG}"`) {
			t.Errorf("%s: pecl extensions are not pinned", name)
		}
		for _, line := range strings.Split(df, "\n") {
			if i := strings.Index(line, "curl "); i >= 0 && !strings.Contains(line, "--max-time") && strings.Contains(line, "http") {
				t.Errorf("%s: curl without a timeout: %s", name, line)
			}
		}
	}
	if strings.Contains(comboDockerfile, "nodesource") {
		t.Error("combo image still installs Node from NodeSource")
	}
}

// Bullseye-based PHP images (7.4, 8.0) switch to archive.debian.org before
// the first apt-get, in both the PHP and the combo Dockerfiles; the switch
// script is valid shell.
func TestDockerfiles_BullseyeUsesArchive(t *testing.T) {
	for name, df := range map[string]string{"php": phpImageDockerfile, "combo": comboDockerfile} {
		sw := strings.Index(df, "archive.debian.org/debian-security")
		apt := strings.Index(df, "apt-get update")
		if sw < 0 || apt < 0 || sw > apt {
			t.Errorf("%s: bullseye mirror switch missing or after apt-get update", name)
		}
		if !strings.Contains(df, `[ "$VERSION_CODENAME" = bullseye ]`) {
			t.Errorf("%s: the switch is not limited to bullseye", name)
		}
	}
}

// Invalid credentials are refused by the pre-removal check, before any
// container is touched.
func TestCheckMariaDBCredentials_ValidatesFormat(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for _, c := range [][2]string{{"dev_user", ""}, {"dev user", "x"}, {"dev_user", "a\nb"}} {
		if err := CheckMariaDBCredentials(io.Discard, c[0], c[1]); err == nil {
			t.Errorf("CheckMariaDBCredentials(%q, %q) accepted", c[0], c[1])
		}
	}
}

// Recreating Nginx with a certificate that cannot be renewed fails before
// the running container is removed.
func TestRecreateNginx_CertFailureKeepsContainer(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dockerLog := filepath.Join(t.TempDir(), "docker.log")
	stubBin(t, map[string]string{
		"mkcert": "exit 1",
		"docker": `printf '%s\n' "$*" >> '` + dockerLog + `'
case "$1" in inspect) echo running ;; esac`,
	})
	exe, _ := testExe()
	if _, err := RecreateNginx(context.Background(), exe, nil, io.Discard); err == nil {
		t.Fatal("a failed certificate must fail the recreate")
	}
	data, _ := os.ReadFile(dockerLog)
	if strings.Contains(string(data), "rm") {
		t.Errorf("the container was removed before the certificate failed:\n%s", data)
	}
}

// A failed installation whose mount stayed active keeps the record, so the
// mount can still be removed.
func TestAddMoodlePlugin_FailureKeepsRecordWhileMounted(t *testing.T) {
	_, dev := pluginStack(t, map[string]string{"local": "local"})
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "missing"))
	orig := mountActive
	mountActive = func(string, string) bool { return true }
	t.Cleanup(func() { mountActive = orig })
	var buf bytes.Buffer
	exe := &executor.Executor{DryRun: true, UsePolicyKit: true, Stdout: &buf, Stderr: &buf}
	err := AddMoodlePlugin(context.Background(), exe, &buf, dev, "mdle", "local", "example")
	if err == nil || !strings.Contains(err.Error(), "registro foi mantido") {
		t.Fatalf("err = %v", err)
	}
	if recs := loadCfg(t).MoodlePlugins; len(recs) != 1 {
		t.Errorf("records = %+v, want the record kept", recs)
	}
}
