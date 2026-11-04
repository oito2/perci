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
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/oito2/perci/internal/config"
)

// syncFixture builds, under home, a bare "remote" with one commit and
// returns it with the account. Repositories cloned from it have a local
// origin, which Fetch and Pull use like any other.
func syncFixture(t *testing.T) (home, bare string, acc config.GitAccount) {
	t.Helper()
	requireTool(t, "git")
	home = isolate(t)
	acc = validAccount(t, home)
	bare = filepath.Join(home, "bare.git")
	git(t, home, "init", "-q", "--bare", bare)
	seed := filepath.Join(home, "seed")
	git(t, home, "clone", "-q", bare, seed)
	writeFile(t, filepath.Join(seed, "a.txt"), "1")
	git(t, seed, "add", "a.txt")
	git(t, seed, "commit", "-q", "-m", "first")
	git(t, seed, "push", "-q", "origin", "main")
	return home, bare, acc
}

// pushRemoteCommit adds one commit to bare from a throwaway clone.
func pushRemoteCommit(t *testing.T, home, bare, file string) {
	t.Helper()
	tmp := filepath.Join(home, "pusher-"+file)
	git(t, home, "clone", "-q", bare, tmp)
	writeFile(t, filepath.Join(tmp, file), file)
	git(t, tmp, "add", file)
	git(t, tmp, "commit", "-q", "-m", "remote "+file)
	git(t, tmp, "push", "-q", "origin", "main")
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func cloneInto(t *testing.T, acc config.GitAccount, bare, rel string) string {
	t.Helper()
	dir := filepath.Join(acc.Folder, rel)
	git(t, acc.Folder, "clone", "-q", bare, dir)
	return dir
}

func states(t *testing.T, acc config.GitAccount, dirs ...string) []RepoState {
	t.Helper()
	var out []RepoState
	for _, d := range dirs {
		out = append(out, ReadRepo(t.Context(), quietExecutor(), acc, d))
	}
	return out
}

func TestNetEnv_NeverPrompts(t *testing.T) {
	home := isolate(t)
	env, err := netEnv(validAccount(t, home))
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(env, "\n")
	for _, want := range []string{"GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=\n", "SSH_ASKPASS=\n", "BatchMode=yes", "IdentitiesOnly=yes", "perci_oito2"} {
		if !strings.Contains(joined+"\n", want) {
			t.Errorf("netEnv lacks %q:\n%s", want, joined)
		}
	}
}

func TestFetch(t *testing.T) {
	home, bare, acc := syncFixture(t)
	ok := cloneInto(t, acc, bare, "ok")
	broken := cloneInto(t, acc, bare, "broken")
	git(t, broken, "remote", "set-url", "origin", filepath.Join(home, "missing.git"))
	draft := filepath.Join(acc.Folder, "draft")
	git(t, acc.Folder, "init", "-q", draft)
	pushRemoteCommit(t, home, bare, "b.txt")

	var out bytes.Buffer
	err := Fetch(t.Context(), quietExecutor(), &out, acc, states(t, acc, ok, broken, draft))
	if err == nil || !strings.Contains(err.Error(), "1 repositório") {
		t.Errorf("Fetch error = %v, want one failure", err)
	}
	for _, want := range []string{"✓ ok", "✗ broken", "– draft: sem remote"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output lacks %q:\n%s", want, out.String())
		}
	}
	if st := states(t, acc, ok)[0]; st.Behind != 1 || st.LastFetch == "" {
		t.Errorf("after Fetch: behind %d, lastFetch %q", st.Behind, st.LastFetch)
	}
}

func TestPull(t *testing.T) {
	home, bare, acc := syncFixture(t)
	behind := cloneInto(t, acc, bare, "behind")
	dirty := cloneInto(t, acc, bare, "dirty")
	writeFile(t, filepath.Join(dirty, "a.txt"), "local edit")
	diverged := cloneInto(t, acc, bare, "diverged")
	git(t, diverged, "commit", "-q", "--allow-empty", "-m", "local")
	pushRemoteCommit(t, home, bare, "b.txt")
	current := cloneInto(t, acc, bare, "current")
	noUpstream := filepath.Join(acc.Folder, "noupstream")
	git(t, acc.Folder, "init", "-q", noUpstream)
	git(t, noUpstream, "remote", "add", "origin", bare)

	var out bytes.Buffer
	err := Pull(t.Context(), quietExecutor(), &out, acc, states(t, acc, behind, dirty, diverged, current, noUpstream))
	if err != nil {
		t.Errorf("Pull = %v; skips are not failures\n%s", err, out.String())
	}
	for _, want := range []string{
		"✓ behind: 1 commit(s) baixado(s)",
		"– dirty: há alterações não commitadas",
		"– diverged: divergiu do remoto (1 local, 1 remoto)",
		"= current: já está em dia",
		"– noupstream: a branch main não acompanha",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output lacks %q:\n%s", want, out.String())
		}
	}
	if _, err := os.Stat(filepath.Join(behind, "b.txt")); err != nil {
		t.Errorf("behind was not updated: %v", err)
	}
	if _, err := os.Stat(filepath.Join(diverged, "b.txt")); err == nil {
		t.Error("a diverged repository was changed")
	}
	if data, _ := os.ReadFile(filepath.Join(dirty, "a.txt")); string(data) != "local edit" {
		t.Errorf("a dirty repository lost its edit: %q", data)
	}

	broken := cloneInto(t, acc, bare, "broken")
	git(t, broken, "remote", "set-url", "origin", filepath.Join(home, "missing.git"))
	if err := Pull(t.Context(), quietExecutor(), &bytes.Buffer{}, acc, states(t, acc, broken)); err == nil {
		t.Error("Pull with an unreachable remote succeeded")
	}
}

func TestSelectRepos(t *testing.T) {
	_, bare, acc := syncFixture(t)
	a := cloneInto(t, acc, bare, "a")
	git(t, a, "remote", "set-url", "origin", "git@github.com:oito2/a.git")
	b := cloneInto(t, acc, bare, "b")
	git(t, b, "remote", "set-url", "origin", "git@github.com:oito2/b.git")
	foreign := cloneInto(t, acc, bare, "foreign")
	ctx, exe := t.Context(), quietExecutor()

	all, err := SelectRepos(ctx, exe, acc, "")
	if err != nil || len(all) != 2 {
		t.Fatalf("SelectRepos(all) = %+v, %v", all, err)
	}
	one, err := SelectRepos(ctx, exe, acc, b+"/")
	if err != nil || len(one) != 1 || one[0].Dir != b {
		t.Errorf("SelectRepos(b) = %+v, %v", one, err)
	}
	for _, dir := range []string{foreign, acc.Folder, "/etc"} {
		if _, err := SelectRepos(ctx, exe, acc, dir); !errors.Is(err, ErrNotAccountRepo) {
			t.Errorf("SelectRepos(%s) = %v, want ErrNotAccountRepo", dir, err)
		}
	}
	if !slices.ContainsFunc(all, func(st RepoState) bool { return st.Dir == a }) {
		t.Errorf("a missing from %+v", all)
	}
}
