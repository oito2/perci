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

package androidstudio

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/oito2/perci/internal/executor"
)

func TestCheckTarballEntries(t *testing.T) {
	cases := []struct {
		name    string
		listing string
		ok      bool
	}{
		{"valid", "android-studio/\nandroid-studio/bin/studio.sh\nandroid-studio/lib/a..b.jar\n", true},
		{"absolute path", "android-studio/\n/etc/cron.d/evil\n", false},
		{"dotdot segment", "android-studio/../../etc/passwd\n", false},
		{"outside top dir", "android-studio/\nother/file\n", false},
		{"empty", "\n", false},
	}
	for _, tc := range cases {
		err := checkTarballEntries(tc.listing)
		if (err == nil) != tc.ok {
			t.Errorf("%s: checkTarballEntries err = %v, want ok=%v", tc.name, err, tc.ok)
		}
	}
}

// makeTarball writes a .tar.gz with the given entries (name → content;
// a name ending in "/" is a directory) and returns its path.
func makeTarball(t *testing.T, entries map[string]string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "studio.tar.gz")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	names := make([]string, 0, len(entries))
	for n := range entries {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		hdr := &tar.Header{Name: n, Mode: 0o644, Size: int64(len(entries[n])), Typeflag: tar.TypeReg}
		if strings.HasSuffix(n, "/") {
			hdr.Typeflag, hdr.Mode, hdr.Size = tar.TypeDir, 0o755, 0
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(entries[n])); err != nil {
			t.Fatal(err)
		}
	}
	for _, c := range []io.Closer{tw, gz, f} {
		if err := c.Close(); err != nil {
			t.Fatal(err)
		}
	}
	return p
}

func TestValidateTarballEntries(t *testing.T) {
	good := makeTarball(t, map[string]string{"android-studio/": "", "android-studio/bin/studio.sh": "#!/bin/sh"})
	if err := validateTarballEntries(good); err != nil {
		t.Errorf("valid tarball refused: %v", err)
	}
	for name, entries := range map[string]map[string]string{
		"traversal": {"android-studio/../../etc/x": "x"},
		"absolute":  {"/etc/passwd": "x"},
		"other":     {"idea/bin/idea.sh": "x"},
	} {
		if err := validateTarballEntries(makeTarball(t, entries)); err == nil {
			t.Errorf("%s: tarball accepted", name)
		}
	}
	notGzip := filepath.Join(t.TempDir(), "x.tar.gz")
	if err := os.WriteFile(notGzip, []byte("plain"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateTarballEntries(notGzip); err == nil {
		t.Error("a non-gzip file was accepted")
	}
}

// Install: ONE privileged step (prepare an empty, user-owned /opt dir),
// then the extraction and the launch as the user — root never reads the
// tarball.
func TestInstall_DryRunOnePrivilegedStepThenUserExtraction(t *testing.T) {
	var buf bytes.Buffer
	exe := &executor.Executor{DryRun: true, UsePolicyKit: true, Stdout: &buf, Stderr: &buf}
	tgz := makeTarball(t, map[string]string{"android-studio/bin/studio.sh": "#!/bin/sh"})

	if err := Install(context.Background(), exe, &buf, tgz); err != nil {
		t.Fatalf("Install: %v\n%s", err, buf.String())
	}
	out := buf.String()
	if n := strings.Count(out, "pkexec"); n != 1 {
		t.Errorf("want one pkexec call, got %d:\n%s", n, out)
	}
	var extract string
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "-xzf") {
			extract = line
		}
	}
	if extract == "" || strings.Contains(extract, "pkexec") || !strings.Contains(extract, "--no-same-owner") || !strings.Contains(extract, tgz) {
		t.Errorf("extraction must run unprivileged with --no-same-owner: %q", extract)
	}
	if !strings.Contains(out, "setsid") {
		t.Errorf("studio.sh must be launched detached:\n%s", out)
	}
}

func TestInstall_RejectsForeignTarballBeforeAnyPrivilegedStep(t *testing.T) {
	var buf bytes.Buffer
	exe := &executor.Executor{DryRun: true, UsePolicyKit: true, Stdout: &buf, Stderr: &buf}
	tgz := makeTarball(t, map[string]string{"evil/x": "x"})
	if err := Install(context.Background(), exe, &buf, tgz); err == nil {
		t.Fatal("expected a refusal")
	}
	if strings.Contains(buf.String(), "pkexec") {
		t.Errorf("nothing privileged may run:\n%s", buf.String())
	}
}

// Uninstall removes both install dirs in one privileged call and the
// user's desktop entry.
func TestUninstall_DryRun(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	entry := desktopFile(home)
	if err := os.MkdirAll(filepath.Dir(entry), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(entry, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	exe := &executor.Executor{DryRun: true, UsePolicyKit: true, Stdout: &buf, Stderr: &buf}
	if err := Uninstall(context.Background(), exe, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if strings.Count(out, "pkexec") != 1 || !strings.Contains(out, installDir+".old") {
		t.Errorf("want one privileged rm of both dirs:\n%s", out)
	}
	if _, err := os.Stat(entry); !os.IsNotExist(err) {
		t.Error("desktop entry not removed")
	}
}
