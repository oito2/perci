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

package repo

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oito2/perci/internal/executor"
)

// gitHome runs the test against real git with a throwaway HOME (so
// --global writes there) and no system config; the credential helper
// lookup sees no libsecret, so it resolves to "cache".
func gitHome(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	old := libsecretPaths
	libsecretPaths = nil
	t.Cleanup(func() { libsecretPaths = old })
	return home
}

// gitConfig runs `git [-C dir] config args...` ("" dir = no -C).
func gitConfig(t *testing.T, dir string, args ...string) string {
	t.Helper()
	full := append([]string{"config"}, args...)
	if dir != "" {
		full = append([]string{"-C", dir}, full...)
	}
	out, err := exec.Command("git", full...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func TestConfigureGlobal(t *testing.T) {
	gitHome(t)
	exe := &executor.Executor{}
	if err := ConfigureGlobal(context.Background(), exe, io.Discard, "-Dev Name", "dev@example.com"); err != nil {
		t.Fatal(err)
	}
	name, email := GetGlobalIdentity(context.Background(), exe)
	if name != "-Dev Name" || email != "dev@example.com" {
		t.Errorf("global identity = %q %q", name, email)
	}
	if got := gitConfig(t, "", "--global", "credential.helper"); got == "" {
		t.Error("credential.helper not set")
	}
}

func TestConfigureGlobal_RequiresBoth(t *testing.T) {
	gitHome(t)
	for _, c := range [][2]string{{"", "a@b.c"}, {"Name", ""}} {
		if err := ConfigureGlobal(context.Background(), &executor.Executor{}, io.Discard, c[0], c[1]); err == nil {
			t.Errorf("ConfigureGlobal(%q, %q): expected an error", c[0], c[1])
		}
	}
	if got := gitConfig(t, "", "--global", "user.name"); got != "" {
		t.Errorf("nothing should be written, user.name = %q", got)
	}
}

func TestInit_AppliesLocalIdentity(t *testing.T) {
	gitHome(t)
	dir := filepath.Join(t.TempDir(), "new", "proj")
	exe := &executor.Executor{}
	if err := Init(context.Background(), exe, io.Discard, dir, "Proj Dev", "proj@example.com"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		t.Fatalf("repository not created: %v", err)
	}
	name, email := GetCurrentLocalIdentity(context.Background(), exe, dir)
	if name != "Proj Dev" || email != "proj@example.com" {
		t.Errorf("local identity = %q %q", name, email)
	}
	if got := gitConfig(t, dir, "--local", "credential.helper"); got != "cache" {
		t.Errorf("credential.helper = %q, want cache", got)
	}
	if head, _ := os.ReadFile(filepath.Join(dir, ".git", "HEAD")); !strings.Contains(string(head), "refs/heads/main") {
		t.Errorf("default branch should be main, HEAD = %q", head)
	}
}

func TestInit_WithoutIdentityKeepsGlobal(t *testing.T) {
	gitHome(t)
	dir := t.TempDir()
	if err := Init(context.Background(), &executor.Executor{}, io.Discard, dir, "", ""); err != nil {
		t.Fatal(err)
	}
	if got := gitConfig(t, dir, "--local", "user.name"); got != "" {
		t.Errorf("no local identity expected, got %q", got)
	}
}

func TestApplyLocalIdentityAt_RequiresBoth(t *testing.T) {
	gitHome(t)
	if err := ApplyLocalIdentityAt(context.Background(), &executor.Executor{}, io.Discard, "Name", "", t.TempDir()); err == nil {
		t.Error("expected an error")
	}
}

// Clone from a local repository: the clone lands in the chosen folder and
// gets the local identity.
func TestClone_IntoFolderWithIdentity(t *testing.T) {
	gitHome(t)
	src := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q", "-b", "main", src},
		{"-C", src, "-c", "user.name=x", "-c", "user.email=x@x", "commit", "-q", "--allow-empty", "-m", "init"},
	} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	dest := filepath.Join(t.TempDir(), "clone")
	exe := &executor.Executor{}
	if err := Clone(context.Background(), exe, io.Discard, src, dest, "Cloner", "cloner@example.com"); err != nil {
		t.Fatal(err)
	}
	name, email := GetCurrentLocalIdentity(context.Background(), exe, dest)
	if name != "Cloner" || email != "cloner@example.com" {
		t.Errorf("local identity = %q %q", name, email)
	}
}

func TestResolveCredHelper(t *testing.T) {
	old := libsecretPaths
	t.Cleanup(func() { libsecretPaths = old })
	bin := t.TempDir()
	t.Setenv("PATH", bin)

	helper := filepath.Join(t.TempDir(), "git-credential-libsecret")
	if err := os.WriteFile(helper, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	libsecretPaths = []string{"/nonexistent/helper", helper}
	if got := resolveCredHelper(context.Background(), &executor.Executor{}); got != helper {
		t.Errorf("known path: got %q, want %q", got, helper)
	}

	libsecretPaths = nil
	if got := resolveCredHelper(context.Background(), &executor.Executor{}); got != "cache" {
		t.Errorf("nothing found: got %q, want cache", got)
	}
	if err := os.WriteFile(filepath.Join(bin, "git-credential-libsecret"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := resolveCredHelper(context.Background(), &executor.Executor{}); got != "libsecret" {
		t.Errorf("on PATH: got %q, want libsecret", got)
	}
}
