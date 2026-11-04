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

package gitaccount

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/oito2/perci/internal/config"
	"github.com/oito2/perci/internal/executor"
)

// TestMain lets the test binary act as the askpass program, like perci's
// own executable does.
func TestMain(m *testing.M) {
	if IsAskpassCall() {
		os.Exit(RespondAskpass(os.Stdout))
	}
	os.Exit(m.Run())
}

// isolate points every location this package reads or writes (home, XDG
// folders, global git configuration) at a new temporary folder and
// disconnects the session's SSH agent, so no test reaches the user's real
// files, keys or agent. It returns the temporary home.
func isolate(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(home, ".gitconfig"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("SSH_AUTH_SOCK", "")
	t.Setenv("SSH_AGENT_PID", "")
	return home
}

// fakeBin writes executable shell scripts (name → body) into a new folder
// placed first on PATH.
func fakeBin(t *testing.T, scripts map[string]string) {
	t.Helper()
	dir := t.TempDir()
	for name, body := range scripts {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// requireTool skips the test when name is not installed.
func requireTool(t *testing.T, name string) {
	t.Helper()
	if _, ok := executor.New(nil, nil).Which(name); !ok {
		t.Skip(name + " not available")
	}
}

// testAskpass is the askpass program of the tests: this test binary.
func testAskpass(t *testing.T) Askpass {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return Askpass{Program: exe}
}

// validAccount returns an account that passes Validate, with its folder
// created under home.
func validAccount(t *testing.T, home string) config.GitAccount {
	t.Helper()
	folder := filepath.Join(home, "repos", "oito2")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	return Normalize(config.GitAccount{
		Name:     "OITO2",
		Provider: ProviderGitHub,
		Username: "oito2",
		GitName:  "Oito Dois",
		GitEmail: "oito2@example.com",
		Folder:   folder,
		SSHKey:   filepath.Join(home, ".ssh", "perci_oito2"),
	})
}

// quietExecutor runs real commands with their output discarded.
func quietExecutor() *executor.Executor {
	return executor.New(nil, nil)
}
