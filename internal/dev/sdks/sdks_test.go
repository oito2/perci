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

// Removing both SDKs: Go asks for the password once (rm of /usr/local/go)
// and both PATH entries leave the rc file; Flutter's checkout (in HOME)
// needs no password — and, this being DryRun, stays on disk.
func TestApply_RemoveBoth(t *testing.T) {
	home := tempHome(t)
	flutterDir := filepath.Join(home, "development", "flutter")
	writeFile(t, filepath.Join(flutterDir, "bin", "flutter"), "#!/bin/sh\n")
	bashrc := filepath.Join(home, ".bashrc")
	writeFile(t, bashrc, "alias x=y\n# Go\nexport PATH=\"$PATH:/usr/local/go/bin\"\n"+
		"# Flutter\nexport PATH=\"$PATH:$HOME/development/flutter/bin\"\n")

	var buf bytes.Buffer
	if err := Apply(context.Background(), dryRun(&buf), &buf, nil, []string{"Go SDK", "Flutter + Dart SDK"}); err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(buf.String(), "pkexec"); n != 1 {
		t.Errorf("pkexec called %d times, want 1:\n%s", n, buf.String())
	}
	if _, err := os.Stat(flutterDir); err != nil {
		t.Errorf("DryRun must not remove the Flutter checkout: %v", err)
	}
	if !strings.Contains(buf.String(), "rm [-rf -- "+flutterDir+"]") {
		t.Errorf("Flutter removal not planned:\n%s", buf.String())
	}
	rc, err := os.ReadFile(bashrc)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(rc)); got != "alias x=y" {
		t.Errorf(".bashrc = %q, want only the unrelated line", got)
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
}
