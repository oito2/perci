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

package executor

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The program handed to pkexec never comes from $PATH: a forged sh or
// apt-get first on PATH is ignored, the root-owned one is used.
func TestBuildCmd_ElevatedProgramIgnoresPATH(t *testing.T) {
	fake := t.TempDir()
	for _, name := range []string{"sh", "env", "apt-get"} {
		if err := os.WriteFile(filepath.Join(fake, name), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", fake+string(os.PathListSeparator)+os.Getenv("PATH"))
	e := &Executor{UsePolicyKit: true}
	for _, tc := range []struct {
		env  []string
		name string
	}{{nil, "sh"}, {nil, "apt-get"}, {[]string{"A=1"}, "apt-get"}} {
		_, args := e.buildCmd(true, tc.env, tc.name, nil)
		if !filepath.IsAbs(args[0]) || strings.HasPrefix(args[0], fake) {
			t.Errorf("buildCmd(%v, %q) runs %q, want an absolute path outside $PATH", tc.env, tc.name, args[0])
		}
	}
	if got := trustedPath("perci-no-such-tool"); got != "/usr/bin/perci-no-such-tool" {
		t.Errorf("unknown tool resolved to %q", got)
	}
}

// A canceled unprivileged command returns promptly even when a grandchild
// keeps its output pipe open (shell pipelines, installers).
func TestRun_CancelKillsProcessGroup(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	var out bytes.Buffer
	start := time.Now()
	err := New(nil, nil).Run(ctx, Options{Stdout: &out, Stderr: &out}, "bash", "-c", "sleep 30 | cat")
	if err == nil {
		t.Fatal("a canceled command must fail")
	}
	if d := time.Since(start); d > 10*time.Second {
		t.Errorf("Run returned after %v", d)
	}
}
