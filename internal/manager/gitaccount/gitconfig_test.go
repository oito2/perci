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
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oito2/perci/internal/config"
	"github.com/oito2/perci/internal/executor"
)

// repoConfig returns `git config <key>` as seen from a repository created
// at dir (git init), or "" when the key is unset.
func repoConfig(t *testing.T, dir, key string) string {
	t.Helper()
	exe := quietExecutor()
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		if err := exe.Run(t.Context(), executor.Options{}, "git", "init", "-q", "--", dir); err != nil {
			t.Fatal(err)
		}
	}
	out, _ := exe.Output(t.Context(), executor.Options{}, "git", "-C", dir, "config", "--", key)
	return strings.TrimSpace(out)
}

// includeRules returns every includeIf rule of the global git
// configuration, as "key value" lines.
func includeRules(t *testing.T) []string {
	t.Helper()
	out, _ := quietExecutor().Output(t.Context(), executor.Options{}, "git", "config", "--global", "--get-regexp", `^includeif\.`)
	var rules []string
	for _, l := range strings.Split(out, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			rules = append(rules, l)
		}
	}
	return rules
}

func TestApplyAndRemove(t *testing.T) {
	requireTool(t, "git")
	home := isolate(t)
	ctx := t.Context()
	exe := quietExecutor()
	acc := validAccount(t, home)
	acc.GitName = `Oito "Dois"`

	if err := Apply(ctx, exe, io.Discard, nil, acc); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	repo := filepath.Join(acc.Folder, "labs", "perci")
	if got := repoConfig(t, repo, "user.name"); got != acc.GitName {
		t.Errorf("user.name inside the account folder = %q, want %q", got, acc.GitName)
	}
	if got := repoConfig(t, repo, "user.email"); got != acc.GitEmail {
		t.Errorf("user.email inside the account folder = %q, want %q", got, acc.GitEmail)
	}
	if got := repoConfig(t, repo, "core.sshCommand"); !strings.Contains(got, "'-i' '"+acc.SSHKey+"'") || !strings.Contains(got, "IdentitiesOnly=yes") {
		t.Errorf("core.sshCommand = %q", got)
	}
	outside := filepath.Join(home, "elsewhere")
	if got := repoConfig(t, outside, "user.email"); got != "" {
		t.Errorf("user.email outside the account folder = %q, want unset", got)
	}

	// Applying again keeps a single rule.
	if err := Apply(ctx, exe, io.Discard, &acc, acc); err != nil {
		t.Fatalf("second Apply: %v", err)
	}
	if rules := includeRules(t); len(rules) != 1 {
		t.Fatalf("rules after a second Apply = %q, want 1", rules)
	}

	// Moving the folder replaces the rule.
	moved := acc
	moved.Folder = filepath.Join(home, "repos", "moved")
	if err := os.MkdirAll(moved.Folder, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Apply(ctx, exe, io.Discard, &acc, moved); err != nil {
		t.Fatalf("Apply with a new folder: %v", err)
	}
	rules := includeRules(t)
	if len(rules) != 1 || !strings.Contains(rules[0], moved.Folder+"/") {
		t.Fatalf("rules after moving the folder = %q", rules)
	}
	if got := repoConfig(t, repo, "user.email"); got != "" {
		t.Errorf("old folder still uses the account: user.email = %q", got)
	}

	if err := Remove(ctx, exe, io.Discard, moved); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if rules := includeRules(t); len(rules) != 0 {
		t.Errorf("rules after Remove = %q, want none", rules)
	}
	path, _ := GitconfigPath(acc.ID)
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("account gitconfig still exists: %v", err)
	}
	if err := Remove(ctx, exe, io.Discard, moved); err != nil {
		t.Errorf("Remove of an already removed account: %v", err)
	}
}

// Removing one account leaves another account's rule for the same folder
// value pattern untouched.
func TestRemove_KeepsOtherRules(t *testing.T) {
	requireTool(t, "git")
	home := isolate(t)
	ctx := t.Context()
	exe := quietExecutor()
	a := validAccount(t, home)
	b := a
	b.ID, b.Name = "kadu", "Kadu"
	b.Folder = filepath.Join(home, "repos", "kadu")
	if err := os.MkdirAll(b.Folder, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, acc := range []config.GitAccount{a, b} {
		if err := Apply(ctx, exe, io.Discard, nil, acc); err != nil {
			t.Fatal(err)
		}
	}
	if err := Remove(ctx, exe, io.Discard, a); err != nil {
		t.Fatal(err)
	}
	rules := includeRules(t)
	if len(rules) != 1 || !strings.Contains(rules[0], b.Folder+"/") {
		t.Errorf("rules = %q, want only kadu's", rules)
	}
}

func TestSSHCommand_RefusesSpacesInKnownHostsPath(t *testing.T) {
	home := filepath.Join(t.TempDir(), "with space")
	t.Setenv("HOME", home)
	_, err := SSHCommand(config.GitAccount{SSHKey: "/k"})
	if err == nil || !strings.Contains(err.Error(), "espaços") {
		t.Errorf("SSHCommand error = %v, want a message about spaces", err)
	}
}

// Forget removes the rules, files and tokens of every account, even after
// ~/.perci is gone, and leaves the SSH key in place.
func TestForget(t *testing.T) {
	requireTool(t, "git")
	home := isolate(t)
	store := fakeSecretTool(t)
	ctx, exe := t.Context(), quietExecutor()
	a := validAccount(t, home)
	b := a
	b.ID, b.Name, b.Folder = "kadu", "Kadu", filepath.Join(home, "repos", "kadu")
	if err := os.MkdirAll(b.Folder, 0o755); err != nil {
		t.Fatal(err)
	}
	b.HasToken = true
	for _, acc := range []config.GitAccount{a, b} {
		if err := Apply(ctx, exe, io.Discard, nil, acc); err != nil {
			t.Fatal(err)
		}
	}
	if err := StoreToken(ctx, exe, b, "glpat_secret"); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(a.SSHKey), 0o700); err != nil {
		t.Fatal(err)
	}
	writeFile(t, a.SSHKey, "key")
	dir, _ := Dir()
	if err := os.RemoveAll(dir); err != nil { // as uninstalling with ~/.perci removed
		t.Fatal(err)
	}

	if err := Forget(ctx, exe, io.Discard, []config.GitAccount{a, b}); err != nil {
		t.Fatalf("Forget: %v", err)
	}
	if rules := includeRules(t); len(rules) != 0 {
		t.Errorf("rules after Forget = %q", rules)
	}
	if tok, _ := LookupToken(ctx, exe, b.ID); tok != "" {
		t.Errorf("token still stored (%s)", store)
	}
	if _, err := os.Stat(a.SSHKey); err != nil {
		t.Errorf("Forget removed the SSH key: %v", err)
	}
}
