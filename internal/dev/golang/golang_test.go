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

package golang

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/oito2/perci/internal/executor"
)

func dryRun(buf *bytes.Buffer) *executor.Executor {
	return &executor.Executor{DryRun: true, UsePolicyKit: true, Stdout: buf, Stderr: buf}
}

// fakeBin writes executable scripts into a directory placed first on PATH.
func fakeBin(t *testing.T, scripts map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range scripts {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return dir
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// withReleases serves releases as go.dev's JSON API for the test's duration.
func withReleases(t *testing.T, releases []goRelease) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(releases)
	}))
	t.Cleanup(srv.Close)
	old := goVersionAPI
	goVersionAPI = srv.URL
	t.Cleanup(func() { goVersionAPI = old })
}

func archiveFile(sum string) goFile {
	return goFile{OS: runtime.GOOS, Arch: runtime.GOARCH, Kind: "archive", SHA256: sum}
}

func TestLatestRelease(t *testing.T) {
	withReleases(t, []goRelease{
		{Version: "go1.99rc1", Stable: false, Files: []goFile{archiveFile("rc")}},
		{Version: "go1.98.2", Stable: true, Files: []goFile{
			{OS: runtime.GOOS, Arch: runtime.GOARCH, Kind: "source", SHA256: "source"},
			{OS: "windows", Arch: runtime.GOARCH, Kind: "archive", SHA256: "windows"},
			archiveFile("right"),
		}},
		{Version: "go1.97.9", Stable: true, Files: []goFile{archiveFile("older")}},
	})
	version, sum, err := LatestRelease(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if version != "go1.98.2" || sum != "right" {
		t.Errorf("got %s %s, want go1.98.2 right", version, sum)
	}
}

func TestLatestRelease_Errors(t *testing.T) {
	t.Run("no stable release", func(t *testing.T) {
		withReleases(t, []goRelease{{Version: "go1.99rc1", Files: []goFile{archiveFile("x")}}})
		if _, _, err := LatestRelease(context.Background()); err == nil {
			t.Error("expected an error")
		}
	})
	t.Run("no archive for this platform", func(t *testing.T) {
		withReleases(t, []goRelease{{Version: "go1.98.2", Stable: true, Files: []goFile{
			{OS: "plan9", Arch: "mips", Kind: "archive", SHA256: "x"},
		}}})
		if _, _, err := LatestRelease(context.Background()); err == nil {
			t.Error("expected an error")
		}
	})
	t.Run("invalid JSON", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, "<html>")
		}))
		defer srv.Close()
		old := goVersionAPI
		goVersionAPI = srv.URL
		defer func() { goVersionAPI = old }()
		if _, _, err := LatestRelease(context.Background()); err == nil {
			t.Error("expected an error")
		}
	})
}

// fakeCurl makes curl copy src to its -o argument, logging the URL.
func fakeCurl(t *testing.T, src string) (logPath string) {
	t.Helper()
	logPath = filepath.Join(t.TempDir(), "curl.log")
	fakeBin(t, map[string]string{"curl": `out=""; url=""
while [ $# -gt 0 ]; do
  case "$1" in -o) out="$2"; shift 2 ;; http*) url="$1"; shift ;; *) shift ;; esac
done
printf '%s\n' "$url" >> "` + logPath + `"
cp "` + src + `" "$out"`})
	return logPath
}

func TestDownload(t *testing.T) {
	content := []byte("tarball bytes")
	src := filepath.Join(t.TempDir(), "src.tar.gz")
	if err := os.WriteFile(src, content, 0o644); err != nil {
		t.Fatal(err)
	}
	logPath := fakeCurl(t, src)
	exe := &executor.Executor{}
	tarball := "go1.98.2." + runtime.GOOS + "-" + runtime.GOARCH + ".tar.gz"

	t.Run("given checksum", func(t *testing.T) {
		dest, sum, err := download(context.Background(), exe, io.Discard, t.TempDir(), "go1.98.2", sha256Hex(content))
		if err != nil {
			t.Fatal(err)
		}
		if filepath.Base(dest) != tarball || sum != sha256Hex(content) {
			t.Errorf("dest=%s sum=%s", dest, sum)
		}
		log, _ := os.ReadFile(logPath)
		if !strings.Contains(string(log), downloadBase+"/"+tarball) {
			t.Errorf("curl URL = %q", log)
		}
	})
	t.Run("checksum looked up", func(t *testing.T) {
		withReleases(t, []goRelease{{Version: "go1.98.2", Stable: true, Files: []goFile{archiveFile(sha256Hex(content))}}})
		if _, sum, err := download(context.Background(), exe, io.Discard, t.TempDir(), "go1.98.2", ""); err != nil || sum != sha256Hex(content) {
			t.Fatalf("sum=%s err=%v", sum, err)
		}
	})
	t.Run("checksum mismatch", func(t *testing.T) {
		if _, _, err := download(context.Background(), exe, io.Discard, t.TempDir(), "go1.98.2", sha256Hex([]byte("other"))); err == nil {
			t.Error("expected a verification error")
		}
	})
	t.Run("download fails", func(t *testing.T) {
		fakeBin(t, map[string]string{"curl": "exit 22"})
		if _, _, err := download(context.Background(), exe, io.Discard, t.TempDir(), "go1.98.2", "x"); err == nil {
			t.Error("expected a download error")
		}
	})
}

// The whole replacement runs as one privileged batch.
func TestInstallTarball_OnePrompt(t *testing.T) {
	var buf bytes.Buffer
	if err := installTarball(context.Background(), dryRun(&buf), &buf, "/tmp/x.tar.gz", "abc"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if n := strings.Count(out, "pkexec"); n != 1 {
		t.Errorf("pkexec called %d times, want 1:\n%s", n, out)
	}
	for _, want := range []string{goInstallDir, "/tmp/x.tar.gz", "abc"} {
		if !strings.Contains(out, want) {
			t.Errorf("batch missing %q:\n%s", want, out)
		}
	}
}

// goTarball builds a go*.tar.gz whose single top-level dir holds bin/go
// with content.
func goTarball(t *testing.T, content string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, h := range []*tar.Header{
		{Name: "go/", Typeflag: tar.TypeDir, Mode: 0o755},
		{Name: "go/bin/", Typeflag: tar.TypeDir, Mode: 0o755},
		{Name: "go/bin/go", Typeflag: tar.TypeReg, Mode: 0o755, Size: int64(len(content))},
	} {
		if err := tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := io.WriteString(tw, content); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// runInstallScript runs installScript for real, unprivileged, against a
// temporary install dir that starts with an "old" toolchain.
func runInstallScript(t *testing.T, tarball []byte, sum string) (installDir string, err error) {
	t.Helper()
	for _, tool := range []string{"sh", "tar", "sha256sum", "install"} {
		if _, lookErr := exec.LookPath(tool); lookErr != nil {
			t.Skip(tool + " not available")
		}
	}
	root := t.TempDir()
	installDir = filepath.Join(root, "go")
	if err := os.MkdirAll(filepath.Join(installDir, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(installDir, "bin", "go"), []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(t.TempDir(), "go.tar.gz")
	if err := os.WriteFile(src, tarball, 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("sh", "-c", installScript, "sh", installDir, src, sum).CombinedOutput()
	if err != nil {
		t.Logf("script output: %s", out)
	}
	return installDir, err
}

func assertToolchain(t *testing.T, installDir, want string) {
	t.Helper()
	got, err := os.ReadFile(filepath.Join(installDir, "bin", "go"))
	if err != nil || string(got) != want {
		t.Errorf("bin/go = %q (err %v), want %q", got, err, want)
	}
	for _, leftover := range []string{"-staging", "-old", "-staging.tar.gz"} {
		if _, err := os.Stat(installDir + leftover); !os.IsNotExist(err) {
			t.Errorf("%s left behind", installDir+leftover)
		}
	}
}

func TestInstallScript(t *testing.T) {
	t.Run("replaces the toolchain", func(t *testing.T) {
		tb := goTarball(t, "new")
		dir, err := runInstallScript(t, tb, sha256Hex(tb))
		if err != nil {
			t.Fatal(err)
		}
		assertToolchain(t, dir, "new")
	})
	t.Run("checksum mismatch keeps the old toolchain", func(t *testing.T) {
		dir, err := runInstallScript(t, goTarball(t, "new"), sha256Hex([]byte("other")))
		if err == nil {
			t.Fatal("expected an error")
		}
		assertToolchain(t, dir, "old")
	})
	t.Run("broken archive keeps the old toolchain", func(t *testing.T) {
		broken := []byte("not a tarball")
		dir, err := runInstallScript(t, broken, sha256Hex(broken))
		if err == nil {
			t.Fatal("expected an error")
		}
		assertToolchain(t, dir, "old")
	})
}

func readRC(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// Install replaces the unquoted PATH line with the quoted one, once;
// uninstall removes both forms.
func TestPathEntry_FishLegacyReplacedAndRemoved(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SHELL", "/usr/bin/fish")
	rc := filepath.Join(home, ".config", "fish", "config.fish")
	if err := os.MkdirAll(filepath.Dir(rc), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rc, []byte("set -x A 1\n\n# Go\n"+legacyPathEntry+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	EnsurePathInBashrc(io.Discard)
	EnsurePathInBashrc(io.Discard)
	text := readRC(t, rc)
	if strings.Contains(text, legacyPathEntry+"\n") {
		t.Errorf("legacy line still present:\n%s", text)
	}
	if strings.Count(text, pathEntry) != 1 || strings.Count(text, "# Go") != 1 {
		t.Errorf("want one comment and one quoted line:\n%s", text)
	}

	// An rc file that still has the legacy form is cleaned on uninstall too.
	bashrc := filepath.Join(home, ".bashrc")
	if err := os.WriteFile(bashrc, []byte("# Go\n"+legacyPathEntry+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	RemovePathFromBashrc(io.Discard)
	for _, p := range []string{rc, bashrc} {
		text := readRC(t, p)
		if strings.Contains(text, "/usr/local/go/bin") || strings.Contains(text, "# Go") {
			t.Errorf("%s still has the Go entry:\n%s", p, text)
		}
	}
	if !strings.Contains(readRC(t, rc), "set -x A 1") {
		t.Error("unrelated lines must stay")
	}
}

func TestUninstall_OnePromptAndPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SHELL", "/bin/bash")
	bashrc := filepath.Join(home, ".bashrc")
	if err := os.WriteFile(bashrc, []byte("# Go\n"+pathEntry+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := Uninstall(context.Background(), dryRun(&buf), &buf); err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(buf.String(), "pkexec"); n != 1 {
		t.Errorf("pkexec called %d times, want 1", n)
	}
	if strings.Contains(readRC(t, bashrc), "/usr/local/go/bin") {
		t.Error("PATH entry not removed")
	}
}

func TestInstalledVersion(t *testing.T) {
	dir := t.TempDir()
	old := goBinary
	t.Cleanup(func() { goBinary = old })
	exe := &executor.Executor{}

	goBinary = filepath.Join(dir, "missing")
	if ok, v := InstalledVersion(context.Background(), exe); ok || v != "" {
		t.Errorf("missing binary: got %v %q", ok, v)
	}

	goBinary = filepath.Join(dir, "go")
	write := func(body string) {
		if err := os.WriteFile(goBinary, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write("echo 'go version go1.98.2 linux/amd64'")
	if ok, v := InstalledVersion(context.Background(), exe); !ok || v != "go1.98.2" {
		t.Errorf("got %v %q, want true go1.98.2", ok, v)
	}
	write("echo 'something else entirely'")
	if ok, _ := InstalledVersion(context.Background(), exe); ok {
		t.Error("unexpected output must not count as installed")
	}
	// DryRun's placeholder output is not read as a version.
	if ok, v := InstalledVersion(context.Background(), &executor.Executor{DryRun: true}); ok {
		t.Errorf("DryRun reported Go installed (%q)", v)
	}
}
