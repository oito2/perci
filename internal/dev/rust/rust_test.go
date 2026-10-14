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

package rust

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/oito2/perci/internal/executor"
)

// tempHome points HOME at a temporary folder with bash as the shell and
// clears CARGO_HOME.
func tempHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SHELL", "/bin/bash")
	t.Setenv("CARGO_HOME", "")
	return home
}

func TestChecksumsArePinned(t *testing.T) {
	hexSum := regexp.MustCompile(`^[0-9a-f]{64}$`)
	for _, target := range []string{"x86_64-unknown-linux-gnu", "aarch64-unknown-linux-gnu"} {
		if !hexSum.MatchString(rustupChecksums[target]) {
			t.Errorf("%s: checksum %q", target, rustupChecksums[target])
		}
	}
}

// Install refuses a download whose checksum doesn't match, before running
// it; a matching one runs without changing the shell files, with
// rust-analyzer, and Perci writes its own PATH line.
func TestInstall(t *testing.T) {
	home := tempHome(t)
	script := []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$HOME/args\"\n")
	var served atomic.Pointer[[]byte]
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(*served.Load()) }))
	t.Cleanup(srv.Close)
	oldBase, oldSums := downloadBase, rustupChecksums
	t.Cleanup(func() { downloadBase, rustupChecksums = oldBase, oldSums })
	downloadBase = srv.URL
	sum := sha256.Sum256(script)
	rustupChecksums = map[string]string{target(): hex.EncodeToString(sum[:])}

	var buf bytes.Buffer
	exe := executor.New(&buf, &buf)
	tampered := []byte("tampered")
	served.Store(&tampered)
	if err := Install(context.Background(), exe, &buf); err == nil || !strings.Contains(err.Error(), "integridade") {
		t.Fatalf("a tampered download was accepted: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, "args")); err == nil {
		t.Fatal("the tampered installer ran")
	}

	served.Store(&script)
	if err := Install(context.Background(), exe, &buf); err != nil {
		t.Fatalf("Install: %v\n%s", err, buf.String())
	}
	args, _ := os.ReadFile(filepath.Join(home, "args"))
	for _, want := range []string{"-y\n", "--no-modify-path\n", "--profile\ndefault\n", "--default-toolchain\nstable\n", "--component\nrust-analyzer\n"} {
		if !strings.Contains(string(args), want) {
			t.Errorf("rustup-init args lack %q:\n%s", want, args)
		}
	}
	rc, _ := os.ReadFile(filepath.Join(home, ".bashrc"))
	if !strings.Contains(string(rc), pathEntry) {
		t.Errorf(".bashrc lacks the PATH line:\n%s", rc)
	}
}

// Update and Uninstall run the rustup in ~/.cargo/bin without privileges;
// Uninstall removes the PATH line.
func TestUpdateAndUninstall(t *testing.T) {
	home := tempHome(t)
	ensurePath(&bytes.Buffer{})
	var buf bytes.Buffer
	exe := &executor.Executor{DryRun: true, Stdout: &buf, Stderr: &buf}
	rustup := filepath.Join(home, ".cargo", "bin", "rustup")

	if err := Update(context.Background(), exe, &buf); err != nil {
		t.Fatal(err)
	}
	if err := Uninstall(context.Background(), exe, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{rustup + " [update]", rustup + " [component add rust-analyzer]", rustup + " [self uninstall -y]"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "pkexec") || strings.Contains(out, "sudo") {
		t.Errorf("Rust ran with privileges:\n%s", out)
	}
	if rc, _ := os.ReadFile(filepath.Join(home, ".bashrc")); strings.Contains(string(rc), pathEntry) {
		t.Errorf("PATH line kept after uninstall:\n%s", rc)
	}
}

func TestIsInstalled(t *testing.T) {
	home := tempHome(t)
	if IsInstalled() {
		t.Fatal("installed with no ~/.cargo")
	}
	bin := filepath.Join(home, ".cargo", "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"rustup", "rustc"} {
		if err := os.WriteFile(filepath.Join(bin, name), nil, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if !IsInstalled() {
		t.Error("not installed with rustup and rustc in ~/.cargo/bin")
	}
	t.Setenv("CARGO_HOME", t.TempDir())
	if IsInstalled() {
		t.Error("CARGO_HOME ignored")
	}
}
