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
	"slices"
	"strings"
	"testing"

	"github.com/oito2/perci/internal/config"
)

// registryState is the state answered by the fake docker of fakeRegistry:
// local image IDs by reference, image IDs a pull downloads, the perci.hash
// label of built images and the image ID each container runs.
type registryState struct {
	images     map[string]string
	newer      map[string]string
	labels     map[string]string
	containers map[string]string
}

// stateKey turns an image or container name into a file name.
func stateKey(name string) string {
	return strings.NewReplacer("/", "_", ":", "_").Replace(name)
}

// fakeRegistry puts a `docker` script first on PATH that answers image and
// container inspection, `pull` and `build` from files under a temporary
// directory seeded with st, and returns its call log. A build gives the
// image a new ID and records its label.
func fakeRegistry(t *testing.T, st registryState) string {
	t.Helper()
	bin := t.TempDir()
	dir := t.TempDir()
	for sub, m := range map[string]map[string]string{"img": st.images, "newer": st.newer, "label": st.labels, "ctr": st.containers} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			t.Fatal(err)
		}
		for k, v := range m {
			if err := os.WriteFile(filepath.Join(dir, sub, stateKey(k)), []byte(v+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	log := filepath.Join(bin, "calls.log")
	script := `#!/bin/sh
D='` + dir + `'
printf '%s\n' "$*" >> '` + log + `'
key() { printf '%s' "$1" | tr '/:' '__'; }
last() { for a in "$@"; do l="$a"; done; printf '%s' "$l"; }
case "$1" in
image)
  ref=$(last "$@")
  case "$*" in
  *Labels*) [ -f "$D/label/$(key "$ref")" ] || exit 1; cat "$D/label/$(key "$ref")" ;;
  *) [ -f "$D/img/$(key "$ref")" ] || exit 1; cat "$D/img/$(key "$ref")" ;;
  esac ;;
pull)
  ref=$(last "$@")
  [ -f "$D/newer/$(key "$ref")" ] && cp "$D/newer/$(key "$ref")" "$D/img/$(key "$ref")"
  exit 0 ;;
build)
  name=$3
  hash=${5#perci.hash=}
  echo "built-$name" > "$D/img/$(key "$name")"
  echo "$hash" > "$D/label/$(key "$name")" ;;
inspect)
  c=$(last "$@")
  [ -f "$D/ctr/$(key "$c")" ] || exit 1
  cat "$D/ctr/$(key "$c")" ;;
esac
exit 0
`
	if err := os.WriteFile(filepath.Join(bin, "docker"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return log
}

var (
	loja82  = config.AppContainer{Name: "Loja", Folder: "loja", URL: "loja.localhost", Type: config.AppTypePHP, PHPVersion: "8.2"}
	blog82  = config.AppContainer{Name: "Blog", Folder: "blog", URL: "blog.localhost", Type: config.AppTypePHP, PHPVersion: "8.2"}
	site83  = config.AppContainer{Name: "Site", Folder: "site", URL: "site.localhost", Type: config.AppTypePHP, PHPVersion: "8.3"}
	front24 = config.AppContainer{Name: "Front", Folder: "front", URL: "front.localhost", Type: config.AppTypeNode, NodeVersion: "24", DevCommand: "npm run dev", DevPort: 5173}
)

func TestAppImageTarget_GroupsAppsSharingTheImage(t *testing.T) {
	stackHome(t, loja82, site83, blog82)
	got, err := AppImageTarget("blog")
	if err != nil {
		t.Fatalf("AppImageTarget: %v", err)
	}
	if want := []string{"loja", "blog"}; !slices.Equal(got.Apps, want) || got.Nginx || got.MariaDB {
		t.Errorf("got %+v, want only apps %v", got, want)
	}
	if _, err := AppImageTarget("nginx"); err == nil {
		t.Error("an unregistered folder was accepted")
	}
}

func TestPrepareImageUpdate(t *testing.T) {
	php82 := phpImageBuild("8.2")
	php83 := phpImageBuild("8.3")
	upToDate := registryState{
		images: map[string]string{
			"php:8.2-fpm": "php82-old", "php:8.3-fpm": "php83-old", NginxImage: "nginx-old",
			php82.name: "perci82-old", php83.name: "perci83-old",
		},
		labels:     map[string]string{php82.name: php82.hash(), php83.name: php83.hash()},
		containers: map[string]string{"loja": "perci82-old", "blog": "perci82-old", "site": "perci83-old", NginxContainerName: "nginx-old"},
	}
	with := func(f func(st *registryState)) registryState {
		st := registryState{images: map[string]string{}, newer: map[string]string{}, labels: map[string]string{}, containers: map[string]string{}}
		for k, v := range upToDate.images {
			st.images[k] = v
		}
		for k, v := range upToDate.labels {
			st.labels[k] = v
		}
		for k, v := range upToDate.containers {
			st.containers[k] = v
		}
		f(&st)
		return st
	}

	cases := []struct {
		name      string
		state     registryState
		target    ImageUpdateTarget
		want      ImageUpdatePlan
		wantBuild []string
	}{
		{
			name:   "nothing new: no build, nothing to recreate",
			state:  with(func(*registryState) {}),
			target: ImageUpdateTarget{Nginx: true, Apps: []string{"loja", "blog", "site"}},
		},
		{
			name:      "new php 8.2 build: its image is rebuilt and both 8.2 apps recreated",
			state:     with(func(st *registryState) { st.newer["php:8.2-fpm"] = "php82-new" }),
			target:    ImageUpdateTarget{Apps: []string{"loja", "blog", "site"}},
			want:      ImageUpdatePlan{Apps: []string{"loja", "blog"}},
			wantBuild: []string{php82.name},
		},
		{
			name:      "image built by another Perci version is rebuilt",
			state:     with(func(st *registryState) { st.labels[php83.name] = "other" }),
			target:    ImageUpdateTarget{Apps: []string{"site"}},
			want:      ImageUpdatePlan{Apps: []string{"site"}},
			wantBuild: []string{php83.name},
		},
		{
			name:   "new nginx build: nginx recreated",
			state:  with(func(st *registryState) { st.newer[NginxImage] = "nginx-new" }),
			target: ImageUpdateTarget{Nginx: true},
			want:   ImageUpdatePlan{Nginx: true},
		},
		{
			name: "container left on an older image is recreated",
			state: with(func(st *registryState) {
				st.containers["site"] = "perci83-older"
			}),
			target: ImageUpdateTarget{Apps: []string{"site"}},
			want:   ImageUpdatePlan{Apps: []string{"site"}},
		},
		{
			name: "missing container is left alone",
			state: with(func(st *registryState) {
				st.newer["php:8.2-fpm"] = "php82-new"
				delete(st.containers, "blog")
			}),
			target:    ImageUpdateTarget{Apps: []string{"loja", "blog"}},
			want:      ImageUpdatePlan{Apps: []string{"loja"}},
			wantBuild: []string{php82.name},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stackHome(t, loja82, blog82, site83)
			if err := config.Update(func(c *config.Config) error {
				c.Docker.NginxCreated = true
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			log := fakeRegistry(t, tc.state)
			exe, out := testExe()

			got, err := PrepareImageUpdate(t.Context(), exe, out, tc.target)
			if err != nil {
				t.Fatalf("PrepareImageUpdate: %v\n%s", err, out)
			}
			if got.Nginx != tc.want.Nginx || got.MariaDB != tc.want.MariaDB || !slices.Equal(got.Apps, tc.want.Apps) {
				t.Errorf("plan = %+v, want %+v", got, tc.want)
			}
			var built []string
			for _, line := range strings.Split(calls(t, log), "\n") {
				if f := strings.Fields(line); len(f) > 2 && f[0] == "build" {
					built = append(built, f[2])
				}
			}
			if !slices.Equal(built, tc.wantBuild) {
				t.Errorf("built %v, want %v", built, tc.wantBuild)
			}
		})
	}
}

// A failed pull stops the update before anything is built or recreated.
func TestPrepareImageUpdate_PullFailureStops(t *testing.T) {
	stackHome(t, loja82)
	bin := t.TempDir()
	log := filepath.Join(bin, "calls.log")
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> '" + log + "'\n[ \"$1\" = pull ] && { echo 'network down' >&2; exit 1; }\nexit 0\n"
	if err := os.WriteFile(filepath.Join(bin, "docker"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	exe, out := testExe()

	if _, err := PrepareImageUpdate(t.Context(), exe, out, ImageUpdateTarget{Apps: []string{"loja"}}); err == nil {
		t.Fatal("a failed pull was not reported")
	}
	if c := calls(t, log); strings.Contains(c, "build") || strings.Contains(c, "rm") || strings.Contains(c, "run") {
		t.Errorf("work continued after the failed pull:\n%s", c)
	}
}

func TestAppImageBuild_Bases(t *testing.T) {
	combo := config.AppContainer{Type: config.AppTypePHPNode, PHPVersion: "8.3", NodeVersion: "24"}
	cases := map[string]struct {
		app  config.AppContainer
		want []string
	}{
		"php":   {loja82, []string{"php:8.2-fpm"}},
		"node":  {front24, []string{"node:24-bookworm"}},
		"combo": {combo, []string{"php:8.3-fpm", "node:24-bookworm-slim"}},
	}
	for name, tc := range cases {
		if got := appImageBuild(tc.app).bases; !slices.Equal(got, tc.want) {
			t.Errorf("%s: bases = %v, want %v", name, got, tc.want)
		}
	}
}

// Recreating keeps a stopped container stopped.
func TestApplyImageUpdate_KeepsStoppedContainersStopped(t *testing.T) {
	stackHome(t, lojaApp)
	log := fakeStack(t, stackOpts{status: "exited"})
	exe, out := testExe()

	if err := ApplyImageUpdate(t.Context(), exe, out, ImageUpdatePlan{Apps: []string{"loja"}}); err != nil {
		t.Fatalf("ApplyImageUpdate: %v\n%s", err, out)
	}
	c := calls(t, log)
	run := strings.Index(c, "run -d")
	stop := strings.LastIndex(c, "stop -- loja")
	if run < 0 || stop < run {
		t.Errorf("the stopped container was not recreated and stopped again:\n%s", c)
	}
}

func TestMariaDBRunArgs_AutoUpgrade(t *testing.T) {
	args := strings.Join(mariadbRunArgs("/data", "/init", nil), " ")
	if !strings.Contains(args, "-e MARIADB_AUTO_UPGRADE=1") {
		t.Errorf("MARIADB_AUTO_UPGRADE missing: %s", args)
	}
}
