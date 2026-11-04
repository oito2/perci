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
	"slices"
	"testing"

	"github.com/oito2/perci/internal/config"
	"github.com/oito2/perci/internal/executor"
)

func TestParseRemote(t *testing.T) {
	cases := []struct {
		raw  string
		ok   bool
		want Remote
	}{
		{"git@github.com:oito2/perci.git", true, Remote{SSH: true, Host: "github.com", Owner: "oito2", Repo: "perci"}},
		{"GitHub.com:oito2/perci", true, Remote{SSH: true, Host: "github.com", Owner: "oito2", Repo: "perci"}},
		{"ssh://git@git.example.com:2222/carlos/app.git", true, Remote{SSH: true, Host: "git.example.com", Port: 2222, Owner: "carlos", Repo: "app"}},
		{"https://github.com/oito2/perci.git", true, Remote{Host: "github.com", Owner: "oito2", Repo: "perci"}},
		{"https://oito2:ghp_x@github.com/oito2/perci", true, Remote{Host: "github.com", Owner: "oito2", Repo: "perci"}},
		{"https://gitlab.com/carlos/group/app.git", true, Remote{Host: "gitlab.com", Owner: "carlos", Repo: "group/app"}},
		{"/srv/bare/repo.git", false, Remote{}},
		{"../repo", false, Remote{}},
		{"file:///srv/bare/repo.git", false, Remote{}},
		{"git://github.com/oito2/perci.git", false, Remote{}},
		{"https://github.com/oito2", false, Remote{}},
		{"", false, Remote{}},
	}
	for _, c := range cases {
		got, ok := ParseRemote(c.raw)
		if ok != c.ok || got != c.want {
			t.Errorf("ParseRemote(%q) = %+v, %v; want %+v, %v", c.raw, got, ok, c.want, c.ok)
		}
	}
}

func TestRedactURL(t *testing.T) {
	cases := map[string]string{
		"https://oito2:ghp_secret@github.com/oito2/perci.git": "https://oito2@github.com/oito2/perci.git",
		"https://github.com/oito2/perci.git":                  "https://github.com/oito2/perci.git",
		"git@github.com:oito2/perci.git":                      "git@github.com:oito2/perci.git",
	}
	for raw, want := range cases {
		if got := RedactURL(raw); got != want {
			t.Errorf("RedactURL(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestClassifyRemote(t *testing.T) {
	acc := config.GitAccount{Host: "github.com", Username: "OITO2"}
	cases := map[string]string{
		"":                                   RemoteNone,
		"git@github.com:oito2/perci.git":     RemoteOK,
		"https://github.com/oito2/perci.git": RemoteHTTPS,
		"git@github.com:someone/perci.git":   RemoteForeign,
		"git@gitlab.com:oito2/perci.git":     RemoteForeign,
		"git@github-oito2:oito2/perci.git":   RemoteForeign,
		"/srv/bare/perci.git":                RemoteForeign,
	}
	for raw, want := range cases {
		if got := ClassifyRemote(acc, raw); got != want {
			t.Errorf("ClassifyRemote(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestParseStatus(t *testing.T) {
	out := "# branch.oid 1234\n# branch.head main\n# branch.upstream origin/main\n# branch.ab +2 -3\n" +
		"1 .M N... 100644 100644 100644 a b file.go\n2 R. N... 100644 100644 100644 a b R100 new\told\n" +
		"u UU N... 1 2 3 4 a b c conflict\n? notes.txt\n? tmp/\n"
	var st RepoState
	parseStatus(out, &st)
	want := RepoState{Branch: "main", Upstream: "origin/main", Ahead: 2, Behind: 3, Modified: 3, Untracked: 2}
	if st != want {
		t.Errorf("parseStatus = %+v, want %+v", st, want)
	}

	st = RepoState{}
	parseStatus("# branch.oid 1234\n# branch.head (detached)\n", &st)
	if !st.Detached || st.Branch != "" {
		t.Errorf("detached HEAD: %+v", st)
	}
}

// mkrepo creates dir with an empty .git folder — enough for Discover.
func mkrepo(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestDiscover(t *testing.T) {
	root := t.TempDir()
	for _, rel := range []string{
		"labs/perci",
		"labs/perci/nested", // inside a repository: not visited
		"apps/write82",
		"node_modules/pkg",
		"vendor/lib",
		".cache/repo",
	} {
		mkrepo(t, filepath.Join(root, rel))
	}
	// A worktree or submodule has a .git file instead of a folder.
	if err := os.MkdirAll(filepath.Join(root, "games", "wt"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "games", "wt", ".git"), []byte("gitdir: x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "labs"), filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}

	got, err := Discover(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		filepath.Join(root, "apps", "write82"),
		filepath.Join(root, "games", "wt"),
		filepath.Join(root, "labs", "perci"),
	}
	if !slices.Equal(got, want) {
		t.Errorf("Discover = %v, want %v", got, want)
	}

	mkrepo(t, root)
	if got, _ := Discover(t.Context(), root); !slices.Equal(got, []string{root}) {
		t.Errorf("Discover with the root a repository = %v", got)
	}
	if _, err := Discover(t.Context(), filepath.Join(root, "missing")); err == nil {
		t.Error("Discover of a missing folder succeeded")
	}
}

// git runs a git command in dir, with a fixed identity for commits.
func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	full := append([]string{"-C", dir, "-c", "user.name=T", "-c", "user.email=t@example.com",
		"-c", "init.defaultBranch=main", "-c", "commit.gpgsign=false"}, args...)
	if err := quietExecutor().Run(t.Context(), executor.Options{}, "git", full...); err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
}

func TestListRepos(t *testing.T) {
	requireTool(t, "git")
	home := isolate(t)
	acc := validAccount(t, home)
	root := acc.Folder

	// labs/perci: SSH remote of the account, one commit ahead of its
	// upstream, one modified and one untracked file.
	bare := filepath.Join(home, "bare.git")
	git(t, home, "init", "--bare", bare)
	perci := filepath.Join(root, "labs", "perci")
	git(t, home, "clone", "-q", bare, perci)
	if err := os.WriteFile(filepath.Join(perci, "a.txt"), []byte("1"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, perci, "add", "a.txt")
	git(t, perci, "commit", "-q", "-m", "first")
	git(t, perci, "push", "-q", "origin", "main")
	git(t, perci, "branch", "-q", "--set-upstream-to=origin/main")
	git(t, perci, "commit", "-q", "--allow-empty", "-m", "second")
	if err := os.WriteFile(filepath.Join(perci, "a.txt"), []byte("2"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(perci, "new.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, perci, "remote", "set-url", "origin", "git@github.com:oito2/perci.git")

	// apps/web: HTTPS remote with an embedded token, own identity.
	web := filepath.Join(root, "apps", "web")
	git(t, root, "init", "-q", web)
	git(t, web, "remote", "add", "origin", "https://oito2:ghp_secret@github.com/oito2/web.git")
	git(t, web, "config", "user.email", "other@example.com")

	// draft: no remote, no commits. third: someone else's repository.
	git(t, root, "init", "-q", filepath.Join(root, "draft"))
	third := filepath.Join(root, "third")
	git(t, root, "init", "-q", third)
	git(t, third, "remote", "add", "origin", "git@github.com:someone/lib.git")

	res, err := ListRepos(t.Context(), quietExecutor(), acc)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Hidden) != 1 || res.Hidden[0].Path != "third" {
		t.Errorf("Hidden = %+v", res.Hidden)
	}
	byPath := map[string]RepoState{}
	for _, st := range res.Repos {
		byPath[st.Path] = st
	}
	if len(byPath) != 3 {
		t.Fatalf("Repos = %+v", res.Repos)
	}

	p := byPath["labs/perci"]
	if p.RemoteClass != RemoteOK || p.Branch != "main" || p.Upstream != "origin/main" || p.Ahead != 1 || p.Behind != 0 ||
		p.Modified != 1 || p.Untracked != 1 || p.LastSubject != "second" || p.LastCommit == "" || p.LastFetch != "" ||
		p.IdentityDiverges || p.Error != "" {
		t.Errorf("labs/perci = %+v", p)
	}
	w := byPath["apps/web"]
	if w.RemoteClass != RemoteHTTPS || w.Remote != "https://oito2@github.com/oito2/web.git" || !w.IdentityDiverges ||
		w.LocalEmail != "other@example.com" || w.LastCommit != "" {
		t.Errorf("apps/web = %+v", w)
	}
	if d := byPath["draft"]; d.RemoteClass != RemoteNone || d.Branch != "main" || d.Error != "" {
		t.Errorf("draft = %+v", d)
	}
}
