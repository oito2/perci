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

package prereqs

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oito2/perci/internal/distro"
	"github.com/oito2/perci/internal/executor"
)

// dryRun is a DryRun executor whose LookPath reports exactly present as
// installed.
func dryRun(buf *bytes.Buffer, present ...string) *executor.Executor {
	set := map[string]bool{}
	for _, p := range present {
		set[p] = true
	}
	return &executor.Executor{DryRun: true, UsePolicyKit: true, Stdout: buf, Stderr: buf,
		LookPath: func(name string) (string, error) {
			if set[name] {
				return "/usr/bin/" + name, nil
			}
			return "", errors.New("not found")
		}}
}

// fakeBin writes executable scripts (name → body) into a new directory
// placed first on PATH, alongside the real bash.
func fakeBin(t *testing.T, scripts map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range scripts {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	bash, ok := executor.New(nil, nil).Which("bash")
	if !ok {
		t.Skip("bash not available")
	}
	if err := os.Symlink(bash, filepath.Join(dir, "bash")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	return dir
}

// Every catalogue item installs and uninstalls on both families without
// error, asking for the password at most once.
func TestInstallUninstall_EveryItemAtMostOnePrompt(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	// appimagetool needs its release API; served locally below.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"assets":[{"name":%q,"digest":"sha256:%s"}]}`,
			"appimagetool-"+appImageToolArch()+".AppImage", strings.Repeat("0", 64))
	}))
	defer srv.Close()
	withGitHub(t, srv.URL)

	for _, family := range []string{distro.Debian, distro.Fedora} {
		for _, p := range Catalogue {
			for _, remove := range []bool{false, true} {
				var buf bytes.Buffer
				exe := dryRun(&buf, "go", "wails3")
				var err error
				if remove {
					err = uninstallFor(context.Background(), exe, &buf, p, family)
				} else {
					err = installFor(context.Background(), exe, &buf, p, family)
				}
				// appimagetool's install can't pass the checksum gate under
				// DryRun (nothing is downloaded) — and must stop before root.
				if p.ID == "appimagetool" && !remove {
					if err == nil || !strings.Contains(err.Error(), "integridade") {
						t.Errorf("%s: err = %v, want the integrity gate", family, err)
					}
				} else if err != nil {
					t.Errorf("%s %s remove=%v: %v", family, p.ID, remove, err)
				}
				if n := strings.Count(buf.String(), "pkexec"); n > 1 {
					t.Errorf("%s %s remove=%v: %d password prompts:\n%s", family, p.ID, remove, n, buf.String())
				}
			}
		}
	}
}

func TestInstallFor_UnsupportedFamily(t *testing.T) {
	var buf bytes.Buffer
	for _, id := range []string{"git", "gh", "docker"} {
		p := Prereq{ID: id, Name: id}
		if err := installFor(context.Background(), dryRun(&buf), &buf, p, distro.Unknown); err == nil {
			t.Errorf("%s: install on an unknown family must fail", id)
		}
	}
	if err := installFor(context.Background(), dryRun(&buf), &buf, Prereq{ID: "nope", Name: "Nope"}, distro.Debian); err == nil {
		t.Error("an unknown prerequisite must fail")
	}
	if err := uninstallFor(context.Background(), dryRun(&buf), &buf, Prereq{ID: "nope", Name: "Nope"}, distro.Debian); err == nil {
		t.Error("an unknown prerequisite must fail")
	}
}

// Docker: engine, service and group in one batch; the group step only
// when the user isn't in it yet.
func TestInstallDocker_OneBatch(t *testing.T) {
	var buf bytes.Buffer
	if err := installDocker(context.Background(), dryRun(&buf), &buf, distro.Debian); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"'docker.io'", "'systemctl' 'enable' '--now' 'docker'"} {
		if !strings.Contains(out, want) {
			t.Errorf("batch missing %s:\n%s", want, out)
		}
	}
	if strings.Count(out, "pkexec") != 1 {
		t.Errorf("want one prompt:\n%s", out)
	}
}

// isInstalled answers from what's on PATH (and pkg-config/dpkg/rpm for
// the packages without a command).
func TestIsInstalled(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	gobin := t.TempDir()
	if err := os.WriteFile(filepath.Join(gobin, "wails3"), nil, 0o755); err != nil {
		t.Fatal(err)
	}
	fakeBin(t, map[string]string{
		"git":         "",
		"ninja-build": "",
		"pkg-config":  `[ "$2" = gtk4 ]`,
		"dpkg":        `[ "$2" = libfuse2t64 ]`,
		"go":          `[ "$2" = GOBIN ] && echo '` + gobin + `'`,
	})
	exe := executor.New(nil, nil)

	want := map[string]bool{
		"git": true, "ninja": true, "gtk4dev": true, "wails3": true, "libfuse2": distro.Detect() != distro.Fedora,
		"curl": false, "gtk3dev": false, "webkitgtk6dev": false, "docker": false, "node": false, "unknown-id": false,
	}
	for id, installed := range want {
		if got := isInstalled(context.Background(), exe, id); got != installed {
			t.Errorf("isInstalled(%s) = %v, want %v", id, got, installed)
		}
	}

	// node: detected through nvm, not the process PATH.
	if err := os.MkdirAll(filepath.Join(home, ".nvm"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".nvm", "nvm.sh"), []byte("node() { :; }\nnpm() { :; }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !isInstalled(context.Background(), exe, "node") {
		t.Error("node via nvm not detected")
	}
}

// InstalledMap reports every catalogue item, keyed by name.
func TestInstalledMap(t *testing.T) {
	fakeBin(t, map[string]string{"git": ""})
	t.Setenv("HOME", t.TempDir())
	m := InstalledMap(context.Background(), executor.New(nil, nil))
	if len(m) != len(Catalogue) || !m["Git"] || m["Curl"] {
		t.Errorf("InstalledMap = %v", m)
	}
}

func TestGoBinDir(t *testing.T) {
	fakeBin(t, map[string]string{"go": `[ "$2" = GOPATH ] && echo /home/u/go`})
	if got := goBinDir(context.Background(), executor.New(nil, nil)); got != "/home/u/go/bin" {
		t.Errorf("goBinDir = %q, want GOPATH/bin when GOBIN is empty", got)
	}
}

func TestUninstallWails3(t *testing.T) {
	gobin := t.TempDir()
	bin := filepath.Join(gobin, "wails3")
	if err := os.WriteFile(bin, nil, 0o755); err != nil {
		t.Fatal(err)
	}
	fakeBin(t, map[string]string{"go": `[ "$2" = GOBIN ] && echo '` + gobin + `'`})
	if err := uninstallWails3(context.Background(), executor.New(nil, nil)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(bin); !os.IsNotExist(err) {
		t.Error("wails3 not removed")
	}
}

func TestInstallWails3_NeedsGo(t *testing.T) {
	var buf bytes.Buffer
	if err := installWails3(context.Background(), dryRun(&buf), &buf); err == nil || !strings.Contains(err.Error(), "go não encontrado") {
		t.Errorf("err = %v", err)
	}
}

func TestUninstallNode_RemovesNVMDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".nvm", "versions"), 0o755); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := uninstallNode(&buf); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(home, ".nvm")); !os.IsNotExist(err) {
		t.Error("~/.nvm not removed")
	}
}

func withGitHub(t *testing.T, base string) {
	t.Helper()
	origAPI, origDL := githubAPIBase, githubDownloadBase
	githubAPIBase, githubDownloadBase = base, base
	t.Cleanup(func() { githubAPIBase, githubDownloadBase = origAPI, origDL })
}

func TestAppImageToolChecksum(t *testing.T) {
	asset := "appimagetool-" + appImageToolArch() + ".AppImage"
	for _, tc := range []struct {
		name, body string
		status     int
		want       string
		errSub     string
	}{
		{"ok", fmt.Sprintf(`{"assets":[{"name":%q,"digest":"sha256:abc"}]}`, asset), 200, "abc", ""},
		{"missing", `{"assets":[{"name":"other","digest":"sha256:abc"}]}`, 200, "", "não encontrado"},
		{"bad digest", fmt.Sprintf(`{"assets":[{"name":%q,"digest":"md5:abc"}]}`, asset), 200, "", "digest"},
		{"http error", `{}`, 500, "", "status 500"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if !strings.HasSuffix(r.URL.Path, "/releases/tags/"+appImageToolVersion) {
					http.NotFound(w, r)
					return
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()
			withGitHub(t, srv.URL)
			got, err := appImageToolChecksum(context.Background(), asset)
			if tc.errSub != "" {
				if err == nil || !strings.Contains(err.Error(), tc.errSub) {
					t.Fatalf("err = %v, want %q", err, tc.errSub)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("got %q, %v", got, err)
			}
		})
	}
}

// Apply goes through the diff with one step per item.
func TestApply_DryRun(t *testing.T) {
	var buf bytes.Buffer
	if err := Apply(context.Background(), dryRun(&buf), &buf, []string{"Git"}, []string{"Curl"}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "[1/2]") || !strings.Contains(out, "[2/2]") {
		t.Errorf("want two steps:\n%s", out)
	}
}
