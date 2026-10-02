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

package flutter

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oito2/perci/internal/distro"
	"github.com/oito2/perci/internal/executor"
)

// Every Android export line is written, once each.
func TestEnsureAndroidEnvInBashrc_WritesEveryLineOnce(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SHELL", "/bin/bash")
	EnsureAndroidEnvInBashrc(io.Discard, home)
	EnsureAndroidEnvInBashrc(io.Discard, home) // idempotent

	data, err := os.ReadFile(filepath.Join(home, ".bashrc"))
	if err != nil {
		t.Fatalf("rc file not written: %v", err)
	}
	text := string(data)
	for _, line := range androidEnvLines {
		if n := strings.Count(text, line); n != 1 {
			t.Errorf("%q appears %d times, want 1:\n%s", line, n, text)
		}
	}
	if n := strings.Count(text, "# Android SDK"); n != 1 {
		t.Errorf("comment appears %d times, want 1", n)
	}
}

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

// Flutter's native libraries install in one batch on both families.
func TestInstallPrereqs_OnePrompt(t *testing.T) {
	for family, want := range map[string]string{distro.Debian: "xz-utils", distro.Fedora: "mesa-libGLU"} {
		var buf bytes.Buffer
		if err := installPrereqsFor(context.Background(), dryRun(&buf), &buf, family); err != nil {
			t.Fatal(err)
		}
		if n := strings.Count(buf.String(), "pkexec"); n != 1 || !strings.Contains(buf.String(), want) {
			t.Errorf("%s: %d prompts, output:\n%s", family, n, buf.String())
		}
	}
	var buf bytes.Buffer
	if err := installPrereqsFor(context.Background(), dryRun(&buf), &buf, distro.Unknown); err != nil || strings.Contains(buf.String(), "pkexec") {
		t.Errorf("unknown family: %v\n%s", err, buf.String())
	}
}

// Java: nothing when java runs; openjdk-21 on Debian, Temurin 21 on
// Fedora, each in one privileged call.
func TestEnsureJava(t *testing.T) {
	for family, want := range map[string]string{distro.Debian: "openjdk-21-jdk", distro.Fedora: "temurin-21-jdk"} {
		var buf bytes.Buffer
		if err := ensureJavaInstall(context.Background(), dryRun(&buf), &buf, family); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(buf.String(), want) || strings.Count(buf.String(), "pkexec") != 1 {
			t.Errorf("%s:\n%s", family, buf.String())
		}
	}
	var buf bytes.Buffer
	if err := ensureJavaInstall(context.Background(), dryRun(&buf), &buf, distro.Unknown); err == nil {
		t.Error("an unknown family must fail")
	}

	fakeBin(t, map[string]string{"java": ""})
	buf.Reset()
	if err := ensureJava(context.Background(), executor.New(&buf, &buf), &buf, distro.Debian); err != nil || buf.Len() != 0 {
		t.Errorf("java present: %v %q", err, buf.String())
	}
}

// androidTools serves a cmdline-tools zip (with a fake sdkmanager) and the
// repository manifest carrying manifestSHA1 for it.
func androidTools(t *testing.T, manifestSHA1 string) (zipSHA1 string) {
	t.Helper()
	var zbuf bytes.Buffer
	zw := zip.NewWriter(&zbuf)
	hdr := &zip.FileHeader{Name: "cmdline-tools/bin/sdkmanager", Method: zip.Deflate}
	hdr.SetMode(0o755)
	w, err := zw.CreateHeader(hdr)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = w.Write([]byte("#!/bin/sh\necho \"sdkmanager $*\" >> \"$HOME/sdk.log\"\n"))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	sum := sha1.Sum(zbuf.Bytes())
	zipSHA1 = hex.EncodeToString(sum[:])
	if manifestSHA1 == "" {
		manifestSHA1 = zipSHA1
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/tools.zip":
			_, _ = w.Write(zbuf.Bytes())
		case "/manifest.xml":
			fmt.Fprintf(w, `<sdk><remotePackage><archives><archive><complete><checksum type="sha1">%s</checksum><url>tools.zip</url></complete><host-os>linux</host-os></archive></archives></remotePackage></sdk>`, manifestSHA1)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	origURL, origManifest := androidCmdlineToolsURL, androidRepoManifestURL
	androidCmdlineToolsURL, androidRepoManifestURL = srv.URL+"/tools.zip", srv.URL+"/manifest.xml"
	t.Cleanup(func() { androidCmdlineToolsURL, androidRepoManifestURL = origURL, origManifest })
	return zipSHA1
}

// The real download: verified against the manifest's SHA-1, extracted
// into ~/Android/Sdk/cmdline-tools/latest, licenses accepted, flutter
// pointed at the SDK.
func TestEnsureAndroidCmdlineTools_DownloadsVerifiesAndConfigures(t *testing.T) {
	for _, tool := range []string{"curl", "unzip", "bash"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skip(tool + " not available")
		}
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SHELL", "/bin/bash")
	androidTools(t, "")
	fakeBin(t, map[string]string{"java": ""})
	flutterBin := filepath.Join(t.TempDir(), "flutter")
	if err := os.WriteFile(flutterBin, []byte("#!/bin/sh\necho \"flutter $*\" >> \"$HOME/sdk.log\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := EnsureAndroidCmdlineTools(context.Background(), executor.New(&buf, &buf), &buf, home, flutterBin); err != nil {
		t.Fatalf("EnsureAndroidCmdlineTools: %v\n%s", err, buf.String())
	}
	sdk := filepath.Join(home, "Android", "Sdk")
	if _, err := os.Stat(filepath.Join(sdk, "cmdline-tools", "latest", "bin", "sdkmanager")); err != nil {
		t.Fatalf("sdkmanager not installed: %v", err)
	}
	logb, _ := os.ReadFile(filepath.Join(home, "sdk.log"))
	for _, want := range []string{"sdkmanager --licenses", "sdkmanager platform-tools", "flutter config --android-sdk " + sdk} {
		if !strings.Contains(string(logb), want) {
			t.Errorf("missing %q in:\n%s", want, logb)
		}
	}
	if rc, _ := os.ReadFile(filepath.Join(home, ".bashrc")); !strings.Contains(string(rc), "ANDROID_HOME") {
		t.Error("Android env not written")
	}
	staging, _ := filepath.Glob(filepath.Join(sdk, "cmdline-tools", "staging-*"))
	if len(staging) > 0 {
		t.Errorf("staging dir left behind: %v", staging)
	}

	// Already installed: only flutter is (re)pointed at the SDK.
	buf.Reset()
	if err := EnsureAndroidCmdlineTools(context.Background(), executor.New(&buf, &buf), &buf, home, flutterBin); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "já instalado") {
		t.Errorf("second run:\n%s", buf.String())
	}
}

// A zip that doesn't match the manifest's SHA-1 is never extracted.
func TestEnsureAndroidCmdlineTools_ChecksumMismatch(t *testing.T) {
	for _, tool := range []string{"curl", "bash"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skip(tool + " not available")
		}
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	androidTools(t, strings.Repeat("0", 40))
	fakeBin(t, map[string]string{"java": ""})

	var buf bytes.Buffer
	err := EnsureAndroidCmdlineTools(context.Background(), executor.New(&buf, &buf), &buf, home, "/bin/true")
	if err == nil || !strings.Contains(err.Error(), "integridade") {
		t.Fatalf("err = %v, want an integrity failure", err)
	}
	if _, err := os.Stat(filepath.Join(home, "Android", "Sdk", "cmdline-tools", "latest")); !os.IsNotExist(err) {
		t.Error("a zip failing its checksum was installed")
	}
}

func TestAndroidCmdlineToolsChecksum_Errors(t *testing.T) {
	for name, body := range map[string]string{
		"no entry": `<sdk></sdk>`,
		"not sha1": `<sdk><remotePackage><archives><archive><complete><checksum type="sha256">x</checksum><url>tools.zip</url></complete><host-os>linux</host-os></archive></archives></remotePackage></sdk>`,
		"not xml":  `{`,
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(body)) }))
		origURL, origManifest := androidCmdlineToolsURL, androidRepoManifestURL
		androidCmdlineToolsURL, androidRepoManifestURL = srv.URL+"/tools.zip", srv.URL+"/manifest.xml"
		if _, err := androidCmdlineToolsChecksum(context.Background()); err == nil {
			t.Errorf("%s: expected an error", name)
		}
		androidCmdlineToolsURL, androidRepoManifestURL = origURL, origManifest
		srv.Close()
	}
}

// Chrome wrapper: only when Chrome exists solely as the Flatpak.
func TestEnsureChromeWrapper(t *testing.T) {
	t.Setenv("SHELL", "/bin/bash")
	for _, tc := range []struct {
		name         string
		nativeChrome bool
		flatpakHas   bool
		wantWrapper  bool
	}{
		{"flatpak only", false, true, true},
		{"native chrome", true, true, false},
		{"no chrome", false, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			code := "1"
			if tc.flatpakHas {
				code = "0"
			}
			fakeBin(t, map[string]string{"flatpak": "exit " + code})
			exe := executor.New(nil, nil)
			exe.LookPath = func(name string) (string, error) {
				if tc.nativeChrome && name == "google-chrome" {
					return "/usr/bin/google-chrome", nil
				}
				return "", errors.New("not found")
			}
			var buf bytes.Buffer
			if err := EnsureChromeWrapper(context.Background(), exe, &buf, home); err != nil {
				t.Fatal(err)
			}
			wrapper := filepath.Join(home, ".local", "bin", "google-chrome")
			b, err := os.ReadFile(wrapper)
			if (err == nil) != tc.wantWrapper {
				t.Fatalf("wrapper exists = %v, want %v", err == nil, tc.wantWrapper)
			}
			if tc.wantWrapper {
				if !strings.Contains(string(b), "flatpak run "+chromeFlatpakID) {
					t.Errorf("wrapper:\n%s", b)
				}
				rc, _ := os.ReadFile(filepath.Join(home, ".bashrc"))
				if !strings.Contains(string(rc), "export CHROME_EXECUTABLE="+executor.ShellQuote(wrapper)) {
					t.Errorf(".bashrc:\n%s", rc)
				}
			}
		})
	}
}

// Clone, PATH export and uninstall round trip.
func TestCloneUninstallAndPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SHELL", "/bin/bash")
	flutterDir := filepath.Join(home, "development", "flutter")

	var buf bytes.Buffer
	if err := CloneFlutter(context.Background(), dryRun(&buf), &buf, home, flutterDir); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "clone --branch stable "+flutterRepo+" "+flutterDir) {
		t.Errorf("clone:\n%s", buf.String())
	}

	EnsurePathInBashrc(&buf, home)
	EnsurePathInBashrc(&buf, home)
	rc := filepath.Join(home, ".bashrc")
	if b, _ := os.ReadFile(rc); strings.Count(string(b), pathEntry) != 1 {
		t.Errorf(".bashrc:\n%s", b)
	}

	if err := os.MkdirAll(filepath.Join(flutterDir, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(flutterDir, "bin", "flutter"), nil, 0o755); err != nil {
		t.Fatal(err)
	}
	if !IsInstalled(filepath.Join(flutterDir, "bin", "flutter")) || IsInstalled(flutterDir) {
		t.Error("IsInstalled must accept only the flutter binary file")
	}
	// DryRun leaves the checkout in place; a real run removes it.
	if err := Uninstall(context.Background(), dryRun(&buf), &buf, home, flutterDir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(flutterDir); err != nil {
		t.Errorf("DryRun removed the flutter dir: %v", err)
	}
	if err := Uninstall(context.Background(), &executor.Executor{}, &buf, home, flutterDir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(flutterDir); !os.IsNotExist(err) {
		t.Error("flutter dir not removed")
	}
	if b, _ := os.ReadFile(rc); strings.Contains(string(b), pathEntry) {
		t.Errorf("PATH entry left in .bashrc:\n%s", b)
	}
}
