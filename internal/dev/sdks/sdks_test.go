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

package sdks

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oito2/perci/internal/executor"
)

func dryRun(buf *bytes.Buffer) *executor.Executor {
	return &executor.Executor{DryRun: true, UsePolicyKit: true, Stdout: buf, Stderr: buf}
}

func tempHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SHELL", "/bin/bash")
	return home
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestInstalledMap_Flutter(t *testing.T) {
	home := tempHome(t)
	exe := &executor.Executor{}
	if InstalledMap(context.Background(), exe)["Flutter + Dart SDK"] {
		t.Error("Flutter reported installed in an empty HOME")
	}
	writeFile(t, filepath.Join(home, "development", "flutter", "bin", "flutter"), "#!/bin/sh\n")
	if !InstalledMap(context.Background(), exe)["Flutter + Dart SDK"] {
		t.Error("Flutter not detected")
	}
}

// Removing Go asks for the password once (rm of /usr/local/go), removing
// Flutter (in HOME) asks for none — and, this being DryRun, its checkout
// stays on disk; both PATH entries leave the rc file.
func TestUninstallOne(t *testing.T) {
	home := tempHome(t)
	flutterDir := filepath.Join(home, "development", "flutter")
	writeFile(t, filepath.Join(flutterDir, "bin", "flutter"), "#!/bin/sh\n")
	bashrc := filepath.Join(home, ".bashrc")
	writeFile(t, bashrc, "alias x=y\n# Go\nexport PATH=\"$PATH:/usr/local/go/bin\"\n"+
		"# Flutter\nexport PATH=\"$PATH:$HOME/development/flutter/bin\"\n")

	for _, s := range Catalogue {
		var buf bytes.Buffer
		if err := UninstallOne(context.Background(), dryRun(&buf), &buf, s); err != nil {
			t.Fatal(err)
		}
		want := map[string]int{"go": 1, "flutter": 0}[s.ID]
		if n := strings.Count(buf.String(), "pkexec"); n != want {
			t.Errorf("%s: pkexec called %d times, want %d:\n%s", s.ID, n, want, buf.String())
		}
		if s.ID == "flutter" && !strings.Contains(buf.String(), "rm [-rf -- "+flutterDir+"]") {
			t.Errorf("Flutter removal not planned:\n%s", buf.String())
		}
	}
	if _, err := os.Stat(flutterDir); err != nil {
		t.Errorf("DryRun must not remove the Flutter checkout: %v", err)
	}
	rc, err := os.ReadFile(bashrc)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(rc)); got != "alias x=y" {
		t.Errorf(".bashrc = %q, want only the unrelated line", got)
	}
}

// fakeGoVersions makes update see installed as the installed Go version
// and latest as the latest release.
func fakeGoVersions(t *testing.T, installed, latest string) {
	t.Helper()
	oldLatest, oldInstalled := latestGoRelease, installedGoVersion
	latestGoRelease = func(context.Context) (string, string, error) { return latest, strings.Repeat("0", 64), nil }
	installedGoVersion = func(context.Context, *executor.Executor) (bool, string) { return true, installed }
	t.Cleanup(func() { latestGoRelease, installedGoVersion = oldLatest, oldInstalled })
}

// Go updates only when a newer release exists; Flutter runs flutter upgrade
// without privileges.
func TestUpdateOne(t *testing.T) {
	tempHome(t)
	fakeGoVersions(t, "go1.26.3", "go1.26.3")
	var buf bytes.Buffer
	if err := UpdateOne(context.Background(), dryRun(&buf), &buf, Catalogue[0]); err != nil {
		t.Fatal(err)
	}
	if out := buf.String(); !strings.Contains(out, "já está na versão mais recente") || strings.Contains(out, "curl") {
		t.Errorf("up-to-date Go must not download:\n%s", out)
	}

	fakeGoVersions(t, "go1.25.0", "go1.26.3")
	buf.Reset()
	// DryRun downloads nothing, so the update stops at the integrity check,
	// before anything runs as root.
	err := UpdateOne(context.Background(), dryRun(&buf), &buf, Catalogue[0])
	if err == nil || !strings.Contains(buf.String(), "go1.26.3.linux") || strings.Contains(buf.String(), "pkexec") {
		t.Errorf("outdated Go must download go1.26.3: err=%v\n%s", err, buf.String())
	}

	buf.Reset()
	if err := UpdateOne(context.Background(), dryRun(&buf), &buf, Catalogue[1]); err != nil {
		t.Fatal(err)
	}
	if out := buf.String(); !strings.Contains(out, "upgrade") || strings.Contains(out, "pkexec") {
		t.Errorf("flutter update:\n%s", out)
	}
}

// Requirements come from the Pré-requisitos catalogue, labeled with it.
func TestMissingRequirements(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	got := MissingRequirements(context.Background(), executor.New(nil, nil), Catalogue[0])
	if len(got) != 1 || got[0] != "Curl (Pré-requisitos)" {
		t.Errorf("go without curl: %v", got)
	}
}

func TestUnknownSDK(t *testing.T) {
	home := tempHome(t)
	exe := &executor.Executor{DryRun: true}
	if err := install(context.Background(), exe, io.Discard, "rust", home, "", ""); err == nil {
		t.Error("install: expected an error")
	}
	if err := remove(context.Background(), exe, io.Discard, "rust", home, ""); err == nil {
		t.Error("remove: expected an error")
	}
	if err := update(context.Background(), exe, io.Discard, "rust", ""); err == nil {
		t.Error("update: expected an error")
	}
}

// The Flutter description says the Android SDK licenses are accepted and
// links the terms.
func TestCatalogue_FlutterDescribesTheLicenseAcceptance(t *testing.T) {
	for _, sk := range Catalogue {
		if sk.ID != "flutter" {
			continue
		}
		if !strings.Contains(sk.Description, "licenças do Android SDK") || !strings.Contains(sk.Description, "https://developer.android.com/studio/terms") {
			t.Errorf("description = %q", sk.Description)
		}
	}
}
