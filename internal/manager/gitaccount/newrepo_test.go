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
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oito2/perci/internal/config"
)

func TestCloneURL(t *testing.T) {
	acc := config.GitAccount{Host: "github.com", Username: "oito2"}
	ok := map[string]string{
		"perci":                              "git@github.com:oito2/perci.git",
		"perci.git":                          "git@github.com:oito2/perci.git",
		"oito2/perci":                        "git@github.com:oito2/perci.git",
		"group/app":                          "git@github.com:oito2/group/app.git",
		"https://github.com/oito2/perci.git": "git@github.com:oito2/perci.git",
		"git@github.com:OITO2/perci.git":     "git@github.com:OITO2/perci.git",
	}
	for in, want := range ok {
		if got, err := CloneURL(acc, in); err != nil || got != want {
			t.Errorf("CloneURL(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "../x", "git@github.com:someone/lib.git", "https://tok:x@gitlab.com/oito2/a", "/srv/bare.git"} {
		if got, err := CloneURL(acc, bad); err == nil {
			t.Errorf("CloneURL(%q) = %q, want an error", bad, got)
		} else if strings.Contains(err.Error(), "tok:x") {
			t.Errorf("the error shows the token: %v", err)
		}
	}
}

func TestDestDir(t *testing.T) {
	home := isolate(t)
	acc := validAccount(t, home)
	mkrepo(t, filepath.Join(acc.Folder, "labs", "perci"))
	if err := os.MkdirAll(filepath.Join(acc.Folder, "empty"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(acc.Folder, "notes.txt"), "x")

	for rel, want := range map[string]string{
		"apps/new": filepath.Join(acc.Folder, "apps", "new"),
		"empty":    filepath.Join(acc.Folder, "empty"),
		"./games/": filepath.Join(acc.Folder, "games"),
	} {
		if got, err := DestDir(acc, rel); err != nil || got != want {
			t.Errorf("DestDir(%q) = %q, %v; want %q", rel, got, err, want)
		}
	}
	for _, bad := range []string{"", ".", "..", "../other", "/tmp/x", "labs/perci", "labs/perci/sub", "labs", "notes.txt"} {
		if got, err := DestDir(acc, bad); err == nil {
			t.Errorf("DestDir(%q) = %q, want an error", bad, got)
		}
	}
}

func TestCloneAndInit(t *testing.T) {
	_, bare, acc := syncFixture(t)
	server := fakeSSHServer(t)
	if err := os.MkdirAll(filepath.Join(server, "oito2"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(bare, filepath.Join(server, "oito2", "perci.git")); err != nil {
		t.Fatal(err)
	}
	ctx, exe := t.Context(), quietExecutor()

	dest := filepath.Join(acc.Folder, "labs", "perci")
	var out bytes.Buffer
	if err := Clone(ctx, exe, &out, acc, "git@github.com:oito2/perci.git", dest); err != nil {
		t.Fatalf("Clone: %v\n%s", err, out.String())
	}
	if _, err := os.Stat(filepath.Join(dest, "a.txt")); err != nil {
		t.Errorf("clone has no a.txt: %v", err)
	}
	if got := originURL(t, dest); got != "git@github.com:oito2/perci.git" {
		t.Errorf("origin = %q", got)
	}

	missing := filepath.Join(acc.Folder, "apps", "missing")
	if err := Clone(ctx, exe, &out, acc, "git@github.com:oito2/missing.git", missing); err == nil {
		t.Error("Clone of a missing repository succeeded")
	}
	if _, err := os.Stat(filepath.Dir(missing)); !os.IsNotExist(err) {
		t.Errorf("a failed clone left the folders it created: %v", err)
	}
	if _, err := os.Stat(acc.Folder); err != nil {
		t.Errorf("a failed clone removed the account folder: %v", err)
	}

	draft := filepath.Join(acc.Folder, "apps", "draft")
	if err := Init(ctx, exe, acc, draft, "bad name"); err == nil {
		t.Error("Init with an invalid remote name succeeded")
	}
	if _, err := os.Stat(draft); !os.IsNotExist(err) {
		t.Errorf("Init with an invalid name created %s", draft)
	}
	if err := Init(ctx, exe, acc, draft, "draft"); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if got := originURL(t, draft); got != "git@github.com:oito2/draft.git" {
		t.Errorf("draft origin = %q", got)
	}
}

func TestAPIBase(t *testing.T) {
	cases := []struct {
		acc  config.GitAccount
		want string
	}{
		{config.GitAccount{Provider: ProviderGitHub, Host: "github.com"}, "https://api.github.com"},
		{config.GitAccount{Provider: ProviderGitHub, Host: "ghe.example.com"}, "https://ghe.example.com/api/v3"},
		{config.GitAccount{Provider: ProviderGitLab, Host: "gitlab.com"}, "https://gitlab.com/api/v4"},
	}
	for _, c := range cases {
		if got := APIBase(c.acc); got != c.want {
			t.Errorf("APIBase(%s) = %q, want %q", c.acc.Host, got, c.want)
		}
	}
}

// apiServer serves the GitHub and GitLab repository listings the way the
// providers page them, recording each request.
type apiServer struct {
	*httptest.Server
	requests []*http.Request
}

func newAPIServer(t *testing.T, status int) *apiServer {
	t.Helper()
	s := &apiServer{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.requests = append(s.requests, r)
		if status != http.StatusOK {
			w.WriteHeader(status)
			return
		}
		var items []map[string]any
		gitlab := strings.Contains(r.URL.Path, "projects")
		switch r.URL.Query().Get("page") {
		case "1":
			for i := 0; i < perPage; i++ {
				items = append(items, repoItem(gitlab, "oito2", fmt.Sprintf("r%03d", i), i%2 == 0))
			}
		case "2":
			items = append(items, repoItem(gitlab, "oito2", "Alpha", false), repoItem(gitlab, "someone", "lib", false))
		}
		_ = json.NewEncoder(w).Encode(items)
	}))
	t.Cleanup(s.Close)
	return s
}

func repoItem(gitlab bool, owner, name string, private bool) map[string]any {
	if gitlab {
		vis := "public"
		if private {
			vis = "private"
		}
		return map[string]any{"path_with_namespace": owner + "/" + name, "visibility": vis, "description": "d", "last_activity_at": "2026-11-01T10:00:00Z"}
	}
	return map[string]any{"name": name, "private": private, "description": "d", "pushed_at": "2026-11-01T10:00:00Z", "owner": map[string]any{"login": owner}}
}

func TestListRemoteRepos(t *testing.T) {
	cases := []struct {
		provider, token, path, query, header string
	}{
		{ProviderGitHub, "ghp_secret", "/user/repos", "affiliation=owner", "Bearer ghp_secret"},
		{ProviderGitHub, "", "/users/oito2/repos", "type=owner", ""},
		{ProviderGitLab, "glpat_secret", "/projects", "owned=true", "glpat_secret"},
		{ProviderGitLab, "", "/users/oito2/projects", "", ""},
	}
	for _, c := range cases {
		t.Run(c.provider+"/"+c.path, func(t *testing.T) {
			srv := newAPIServer(t, http.StatusOK)
			acc := config.GitAccount{Provider: c.provider, Host: "github.com", Username: "oito2"}
			repos, err := ListRemoteRepos(t.Context(), srv.URL, acc, c.token)
			if err != nil {
				t.Fatal(err)
			}
			if len(repos) != perPage+1 || repos[0].Name != "Alpha" || repos[0].SSHURL != "git@github.com:oito2/Alpha.git" {
				t.Errorf("got %d repos, first %+v", len(repos), repos[0])
			}
			if !repos[1].Private || repos[2].Private {
				t.Errorf("visibility: %+v %+v", repos[1], repos[2])
			}
			if len(srv.requests) != 2 {
				t.Fatalf("%d requests, want 2 pages", len(srv.requests))
			}
			r := srv.requests[0]
			if r.URL.Path != c.path || (c.query != "" && !strings.Contains(r.URL.RawQuery, c.query)) {
				t.Errorf("request %s?%s", r.URL.Path, r.URL.RawQuery)
			}
			got := r.Header.Get("Authorization") + r.Header.Get("PRIVATE-TOKEN")
			if got != c.header {
				t.Errorf("auth header %q, want %q", got, c.header)
			}
			if c.token != "" && strings.Contains(r.URL.String(), c.token) {
				t.Error("the token went in the URL")
			}
		})
	}
}

func TestListRemoteRepos_Errors(t *testing.T) {
	acc := config.GitAccount{Provider: ProviderGitHub, Host: "github.com", Username: "oito2"}
	for status, want := range map[int]string{
		http.StatusUnauthorized:        "recusou o token",
		http.StatusNotFound:            "não encontrado",
		http.StatusForbidden:           "limite",
		http.StatusInternalServerError: "500",
	} {
		srv := newAPIServer(t, status)
		_, err := ListRemoteRepos(t.Context(), srv.URL, acc, "ghp_secret")
		if err == nil || !strings.Contains(err.Error(), want) || strings.Contains(err.Error(), "ghp_secret") {
			t.Errorf("status %d: %v", status, err)
		}
	}
}

func TestMarkCloned(t *testing.T) {
	acc := config.GitAccount{Host: "github.com", Username: "oito2"}
	repos := []RemoteRepo{{Name: "Perci"}, {Name: "web"}, {Name: "group/app"}}
	MarkCloned(acc, repos, []RepoState{
		{Path: "labs/perci", Remote: "git@github.com:oito2/perci.git", RemoteClass: RemoteOK},
		{Path: "apps/app", Remote: "https://oito2@github.com/oito2/group/app.git", RemoteClass: RemoteHTTPS},
		{Path: "draft", RemoteClass: RemoteNone},
	})
	if repos[0].LocalPath != "labs/perci" || repos[1].LocalPath != "" || repos[2].LocalPath != "apps/app" {
		t.Errorf("MarkCloned = %+v", repos)
	}
}
